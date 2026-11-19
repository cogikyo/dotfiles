export type Message = {
  id?: number | string;
  method?: string;
  params?: any;
  result?: any;
  error?: { code: number; message: string; data?: unknown };
};

export type Handlers = {
  notify: (method: string, params: any) => void;
  request: (method: string, params: any) => unknown;
};

export class Agent {
  readonly exited: Promise<number>;
  private readonly proc: Bun.Subprocess<"pipe", "pipe", "pipe">;
  private readonly pending = new Map<number, { resolve: (value: any) => void; reject: (error: Error) => void }>();
  private next = 0;
  private log = "";

  constructor(command: string[], cwd: string, env: Record<string, string>, handlers: Handlers) {
    this.proc = Bun.spawn(command, { cwd, env, stdin: "pipe", stdout: "pipe", stderr: "pipe" });
    this.exited = this.proc.exited;
    void this.drain(this.proc.stderr, (line) => this.note(line));
    void this.watch(command[0], handlers);
  }

  request(method: string, params: unknown): Promise<any> {
    const id = this.next++;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.send({ id, method, params });
    });
  }

  notify(method: string, params: unknown) {
    this.send({ method, params });
  }

  async close(grace: number) {
    void this.proc.stdin.end();
    const timer = setTimeout(() => this.kill(), grace);
    await this.exited;
    clearTimeout(timer);
  }

  kill() {
    this.proc.kill("SIGKILL");
  }

  private send(message: Message) {
    if (this.proc.exitCode !== null || this.proc.signalCode !== null) return;
    void this.proc.stdin.write(`${JSON.stringify({ jsonrpc: "2.0", ...message })}\n`);
    void this.proc.stdin.flush();
  }

  private receive(line: string, handlers: Handlers) {
    if (!line.trim()) return;
    let message: Message;
    try {
      message = JSON.parse(line);
    } catch {
      this.note(line);
      return;
    }
    if (message.method === undefined) {
      const waiter = this.pending.get(Number(message.id));
      if (!waiter) return;
      this.pending.delete(Number(message.id));
      if (message.error) waiter.reject(new Error(message.error.message));
      else waiter.resolve(message.result);
      return;
    }
    if (message.id === undefined) {
      handlers.notify(message.method, message.params);
      return;
    }
    this.reply(message, handlers);
  }

  private async watch(name: string | undefined, handlers: Handlers) {
    await this.drain(this.proc.stdout, (line) => this.receive(line, handlers));
    const code = await this.proc.exited;
    const reason = new Error(`${name} exited ${code}: ${this.log.trim().slice(-2000) || "no stderr"}`);
    for (const waiter of this.pending.values()) waiter.reject(reason);
    this.pending.clear();
  }

  private note(line: string) {
    this.log = `${this.log}${line}\n`.slice(-4000);
  }

  private reply(message: Message, handlers: Handlers) {
    try {
      this.send({ id: message.id, result: handlers.request(message.method!, message.params) });
    } catch (error) {
      this.send({ id: message.id, error: { code: -32601, message: String(error) } });
    }
  }

  private async drain(stream: ReadableStream<Uint8Array>, line: (text: string) => void) {
    const decoder = new TextDecoder();
    let buffer = "";
    for await (const chunk of stream) {
      buffer += decoder.decode(chunk, { stream: true });
      let end = buffer.indexOf("\n");
      while (end !== -1) {
        line(buffer.slice(0, end));
        buffer = buffer.slice(end + 1);
        end = buffer.indexOf("\n");
      }
    }
    if (buffer) line(buffer);
  }
}
