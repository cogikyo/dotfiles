const DEVTOOLS = "http://127.0.0.1:9222";
const CONNECT_MS = 15_000;
const POLL_MS = 2_000;
const SETTLE_POLLS = 3;

type Agent = { id: string; name: string; title?: string };

type Entry = {
  seq: number;
  kind: string;
  role?: string;
  type?: string;
  text?: string;
  file?: string;
  url?: string;
  ask?: { action?: string; target?: string; status?: string; machineLabel?: string };
  secret?: string;
  requestId?: string;
  timestampMs: number;
};

type Chat = { planeKey: string; loadState: string; busy: boolean; entries: Entry[] };

const PRELUDE = `
const fiberRoot = () => {
  const el = document.getElementById("root");
  return el[Object.keys(el).find((k) => k.startsWith("__reactContainer"))];
};
const findProps = (test) => {
  const stack = [fiberRoot()];
  while (stack.length) {
    const f = stack.pop();
    const p = f.memoizedProps;
    if (p && typeof p === "object" && test(p)) return p;
    if (f.sibling) stack.push(f.sibling);
    if (f.child) stack.push(f.child);
  }
};
const agents = () => (findProps((p) => Array.isArray(p.agents) && typeof p.agents[0]?.name === "string")?.agents ?? [])
  .map((a) => ({ id: a.id, name: a.name, title: a.title }));
const editor = () => document.querySelector(".sand-prompt-field")?.editor;
const chat = (since) => {
  const p = findProps((p) => Array.isArray(p.entries) && "isGenerating" in p);
  if (!p) return undefined;
  const entries = p.entries.filter((e) => e.seq > since).map((e) => ({
    seq: e.seq,
    kind: e.kind,
    role: e.role,
    type: e.message?.type,
    text: e.content ?? e.message?.content,
    file: e.message?.file_name ?? e.fileName ?? e.file_name,
    url: e.message?.url ?? e.url,
    ask: e.message?.ask && { action: e.message.ask.action, target: e.message.ask.target, status: e.message.ask.status, machineLabel: e.message.ask.machineLabel },
    secret: e.message?.secretRequest?.label,
    requestId: e.requestId,
    timestampMs: e.timestampMs,
  }));
  return { planeKey: p.planeKey, loadState: p.loadState, busy: p.isGenerating || p.trailingRow != null, entries };
};
`;

class Page {
  private next = 0;
  private pending = new Map<number, (msg: any) => void>();

  private constructor(private ws: WebSocket) {
    ws.onmessage = (event) => {
      const msg = JSON.parse(String(event.data));
      this.pending.get(msg.id)?.(msg);
      this.pending.delete(msg.id);
    };
  }

  static async open() {
    const deadline = Date.now() + CONNECT_MS;
    for (;;) {
      const targets = await fetch(`${DEVTOOLS}/json/list`)
        .then((r) => r.json() as Promise<{ type: string; url: string; webSocketDebuggerUrl: string }[]>)
        .catch(() => undefined);
      const page = targets?.find((t) => t.type === "page" && t.url.endsWith("/renderer/index.html"));
      if (page) {
        const ws = new WebSocket(page.webSocketDebuggerUrl);
        await new Promise((resolve, reject) => {
          ws.onopen = resolve;
          ws.onerror = reject;
        });
        return new Page(ws);
      }
      if (Date.now() > deadline) {
        throw new Error(`Grok Bot is not listening on ${DEVTOOLS}; launch it with: setsid -f gtk-launch grok-bot`);
      }
      await Bun.sleep(500);
    }
  }

  send(method: string, params: object = {}) {
    const id = ++this.next;
    this.ws.send(JSON.stringify({ id, method, params }));
    return new Promise<any>((resolve) => this.pending.set(id, resolve));
  }

  async eval<T>(body: string): Promise<T> {
    const expression = `(async () => { ${PRELUDE}\n${body} })()`;
    const res = await this.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
    const failure = res.result?.exceptionDetails;
    if (failure) throw new Error(failure.exception?.description ?? failure.text);
    return res.result.result.value as T;
  }

  async enter() {
    const key = { key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 };
    await this.send("Input.dispatchKeyEvent", { type: "keyDown", text: "\r", ...key });
    await this.send("Input.dispatchKeyEvent", { type: "keyUp", ...key });
  }

  close() {
    this.ws.close();
  }
}

function normalize(name: string) {
  return name.toLowerCase().replace(/[^a-z0-9]/g, "");
}

async function lookup(page: Page, name: string) {
  const bots = await page.eval<Agent[]>("return agents();");
  const want = normalize(name);
  const exact = bots.filter((a) => normalize(a.name) === want);
  const matches = exact.length ? exact : bots.filter((a) => normalize(a.name).startsWith(want));
  if (matches.length === 1) return matches[0];
  throw new Error(`no single bot matches "${name}"; roster: ${bots.map((a) => a.name).join(", ")}`);
}

async function open(page: Page, agent: Agent, since = 0) {
  const deadline = Date.now() + CONNECT_MS;
  let clicked = false;
  for (;;) {
    const chat = await page.eval<Chat | undefined>(`return chat(${since});`);
    if (chat?.planeKey.includes(agent.id) && chat.loadState === "ready") return chat;
    if (!clicked) {
      const id = JSON.stringify(agent.id);
      await page.eval(`document.querySelector('button[data-agent-id=' + JSON.stringify(${id}) + ']')?.click();`);
      clicked = true;
    }
    if (Date.now() > deadline) throw new Error(`${agent.name}'s chat did not open`);
    await Bun.sleep(250);
  }
}

function stamp(ms: number) {
  const d = new Date(ms);
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function render(entry: Entry, bot: string) {
  const who =
    entry.kind === "message" && entry.role === "user" ? "You" : entry.kind === "user-attachment" ? "You" : bot;
  const head = `── ${who} · ${stamp(entry.timestampMs)} ──`;
  if (entry.type === "attachment" || entry.kind === "user-attachment")
    return `${head}\n[attachment] ${entry.file ?? entry.url}`;
  if (entry.type === "local-tool-permission") {
    const { action, status, machineLabel, target } = entry.ask ?? {};
    return `${head}\n[approval ${action} on ${machineLabel} · ${status}] ${target}`;
  }
  if (entry.type === "secret-request") return `${head}\n[secret request] ${entry.secret}`;
  if (entry.text !== undefined) return `${head}\n${entry.text}`;
  return `${head}\n[${entry.type ?? entry.kind}]`;
}

async function roster(page: Page) {
  for (const agent of await page.eval<Agent[]>("return agents();")) {
    console.log(agent.title ? `${agent.name}\t${agent.title}` : agent.name);
  }
}

async function read(page: Page, name: string, count = "20") {
  const agent = await lookup(page, name);
  const chat = await open(page, agent);
  for (const entry of chat.entries.slice(-Number(count))) console.log(render(entry, agent.name) + "\n");
}

async function ask(page: Page, name: string, prompt: string, timeoutS = "600") {
  const text = prompt === "-" ? await Bun.stdin.text() : prompt;
  if (!text.trim()) throw new Error("empty prompt");
  const agent = await lookup(page, name);
  const before = await open(page, agent);
  const since = Math.max(0, ...before.entries.map((e) => e.seq));
  await compose(page, agent, text);
  await follow(page, agent, since, Number(timeoutS));
}

async function compose(page: Page, agent: Agent, text: string) {
  const lines = JSON.stringify(text.trimEnd().split("\n"));
  const draft = await page.eval<string | undefined>(`
    const ed = editor();
    if (!ed) return undefined;
    if (ed.getText().trim()) return "draft";
    const doc = { type: "doc", content: ${lines}.map((l) => ({ type: "paragraph", content: l ? [{ type: "text", text: l }] : [] })) };
    ed.chain().focus().clearContent(true).insertContent(doc).run();
    return "ok";
  `);
  if (draft === undefined) throw new Error("prompt field not found");
  if (draft === "draft") throw new Error(`${agent.name}'s prompt field holds an unsent draft; not overwriting it`);
  await page.enter();
}

async function follow(page: Page, agent: Agent, since: number, timeoutS: number) {
  const deadline = Date.now() + timeoutS * 1000;
  const printed = new Set<number>();
  const asks = new Map<number, string>();
  let requestId: string | undefined;
  let quiet = 0;
  for (;;) {
    await Bun.sleep(POLL_MS);
    const chat = await page.eval<Chat>(`return chat(${since});`);
    requestId ??= chat.entries.find((e) => e.kind === "message" && e.role === "user")?.requestId;
    const replies = chat.entries.filter((e) => e.kind !== "message" && e.requestId === requestId);
    for (const entry of replies) {
      if (entry.ask) {
        const state = `${entry.ask.status}`;
        if (asks.get(entry.seq) !== state) {
          asks.set(entry.seq, state);
          console.error(
            `${agent.name} needs approval in the Grok Bot app: ${entry.ask.action} · ${state} · ${entry.ask.target}`,
          );
        }
      }
      if (printed.has(entry.seq)) continue;
      printed.add(entry.seq);
      console.log(render(entry, agent.name) + "\n");
    }
    quiet = requestId && replies.length && !chat.busy ? quiet + 1 : 0;
    if (quiet >= SETTLE_POLLS) return;
    if (Date.now() > deadline) {
      console.error(`${agent.name} is still working after ${timeoutS}s; check later with: read "${agent.name}"`);
      process.exit(2);
    }
  }
}

async function file(page: Page, name: string, fileName: string) {
  const agent = await lookup(page, name);
  const chat = await open(page, agent);
  const entry = chat.entries.findLast((e) => e.file === fileName && e.url);
  if (!entry?.url) throw new Error(`no attachment named ${fileName} in ${agent.name}'s chat`);
  const res = await page.eval<{ kind: string; text?: string }>(
    `return await window.desktop.readAttachmentText(${JSON.stringify(entry.url)}, ${JSON.stringify(agent.id)});`,
  );
  if (res.kind !== "text" || res.text === undefined)
    throw new Error(`${fileName} is not readable as text (${res.kind})`);
  process.stdout.write(res.text);
}

const USAGE = `usage: bot.ts roster
       bot.ts read <bot> [count]
       bot.ts ask <bot> <prompt | -> [timeout-seconds]
       bot.ts file <bot> <attachment-name>`;

const [command, ...args] = Bun.argv.slice(2);
const commands: Record<string, (page: Page, ...args: string[]) => Promise<void>> = { roster, read, ask, file };
const run = commands[command ?? ""];
const arity: Record<string, number> = { roster: 0, read: 1, ask: 2, file: 2 };
if (!run || args.length < (arity[command!] ?? 0)) {
  console.error(USAGE);
  process.exit(1);
}

const page = await Page.open().catch((error: Error) => {
  console.error(error.message);
  process.exit(1);
});
try {
  await run(page, ...args);
} catch (error) {
  console.error(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
} finally {
  page.close();
}
