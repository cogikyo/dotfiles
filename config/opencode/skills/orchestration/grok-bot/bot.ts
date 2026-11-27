const POOLS = {
  trend: { port: 9222, entry: "grok-bot" },
  x: { port: 9223, entry: "grok-bot-x" },
} as const;
const FLOOR = 95;
const PROBE_MS = 3_000;
const POOLS_MS = 5_000;
const CALL_MS = 15_000;
const CONNECT_MS = 15_000;
const POLL_MS = 2_000;
const SETTLE_POLLS = 3;
const WORKING = 2;
const UNTRACKED = 3;
const DAY_S = 86_400;

type Pool = keyof typeof POOLS;
const names = Object.keys(POOLS) as Pool[];

type Agent = { id: string; name: string; title?: string };
type Usage = { percentUsed: number | null; nextResetMs: number | null };
type Rating = { value: Usage } | { error: string };
type OnDemand = { usedCents: number | null; limitCents: number | null; resetMs: number | null };
type Down = { state: "down" | "unresponsive"; error: string };
type Probe = { pool: Pool } & ({ state: "live"; page: Page } | Down);
type Live = { pool: Pool; page: Page };
type Target = Live & { agent: Agent };

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

class Lost extends Error {}

function message(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function record(value: unknown) {
  return value && typeof value === "object" ? (value as Record<string, unknown>) : undefined;
}

function finite(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : null;
}

async function bounded<T>(ms: number, work: Promise<T>, what: string) {
  let timer: Timer | undefined;
  const late = new Promise<never>((_, reject) => {
    timer = setTimeout(
      () => reject(new Lost(`${what} got no answer within ${Math.round(ms / 100) / 10}s`)),
      Math.max(0, ms),
    );
  });
  try {
    return await Promise.race([work, late]);
  } finally {
    clearTimeout(timer);
  }
}

class Page {
  private next = 0;
  private pending = new Map<number, { resolve: (msg: any) => void; reject: (error: Error) => void }>();

  private constructor(private ws: WebSocket) {
    ws.onmessage = (event) => {
      const msg = JSON.parse(String(event.data));
      this.pending.get(msg.id)?.resolve(msg);
      this.pending.delete(msg.id);
    };
    ws.onclose = () => {
      for (const { reject } of this.pending.values()) reject(new Lost("the CDP socket closed"));
      this.pending.clear();
    };
  }

  static async open(port: number, windowMs: number): Promise<Page | Down> {
    const windowBy = Date.now() + windowMs;
    for (;;) {
      const probeBy = Date.now() + PROBE_MS;
      const res = await fetch(`http://127.0.0.1:${port}/json/list`, { signal: AbortSignal.timeout(PROBE_MS) }).catch(
        (error: Error) => error,
      );
      if (res instanceof Error) {
        const { code } = res as { code?: string };
        if (code === "ECONNREFUSED") return { state: "down", error: "not running" };
        const why =
          res.name === "TimeoutError" ? `gave no answer within ${PROBE_MS / 1000}s` : `failed: ${code ?? res.message}`;
        return { state: "unresponsive", error: `DevTools ${why}` };
      }
      const targets: unknown = await res.json().catch(() => undefined);
      const list = Array.isArray(targets) ? targets.map(record) : [undefined];
      if (list.some((t) => typeof t?.type !== "string" || typeof t.url !== "string")) {
        return { state: "unresponsive", error: "DevTools sent a malformed target list" };
      }
      const target = list.find((t) => t?.type === "page" && String(t.url).endsWith("/renderer/index.html"));
      if (target) {
        const url = target.webSocketDebuggerUrl;
        if (typeof url !== "string") return { state: "unresponsive", error: "the main window has no CDP socket URL" };
        return Page.attach(url, probeBy).catch((error) => ({
          state: "unresponsive" as const,
          error: message(error),
        }));
      }
      if (Date.now() >= windowBy) return { state: "unresponsive", error: "no main window" };
      await Bun.sleep(Math.min(500, windowBy - Date.now()));
    }
  }

  private static async attach(url: string, probeBy: number) {
    const ws = new WebSocket(url);
    const page = new Page(ws);
    try {
      await bounded(
        probeBy - Date.now(),
        new Promise((resolve, reject) => {
          ws.onopen = resolve;
          ws.onerror = () => reject(new Lost("the CDP socket failed to open"));
        }),
        "CDP connect",
      );
      const res = await page.send("Runtime.evaluate", { expression: "1", returnByValue: true }, probeBy - Date.now());
      if (res.result?.result?.value !== 1) throw new Lost("CDP did not evaluate a probe expression");
      return page;
    } catch (error) {
      page.close();
      throw error instanceof Lost ? new Lost(`DevTools gave no full answer within ${PROBE_MS / 1000}s`) : error;
    }
  }

  async send(method: string, params: object = {}, ms = CALL_MS) {
    if (this.ws.readyState !== WebSocket.OPEN) throw new Lost("the CDP socket is closed");
    const id = ++this.next;
    const reply = new Promise<any>((resolve, reject) => this.pending.set(id, { resolve, reject }));
    this.ws.send(JSON.stringify({ id, method, params }));
    try {
      const msg = await bounded(ms, reply, `CDP ${method}`);
      if (msg.error) throw new Error(`CDP ${method} failed: ${msg.error.message}`);
      return msg;
    } finally {
      this.pending.delete(id);
    }
  }

  async eval<T>(body: string, ms = CALL_MS): Promise<T> {
    const expression = `(async () => { ${PRELUDE}\n${body} })()`;
    const res = await this.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }, ms);
    const failure = res.result?.exceptionDetails;
    if (failure) throw new Error(failure.exception?.description ?? failure.text);
    return res.result.result.value as T;
  }

  async enter(by: number) {
    const key = { key: "Enter", code: "Enter", windowsVirtualKeyCode: 13 };
    await this.send("Input.dispatchKeyEvent", { type: "keyDown", text: "\r", ...key }, by - Date.now());
    await this.send("Input.dispatchKeyEvent", { type: "keyUp", ...key }, by - Date.now());
  }

  close() {
    this.ws.close();
  }
}

function normalize(name: string) {
  return name.toLowerCase().replace(/[^a-z0-9]/g, "");
}

async function connect(pinned: Pool | undefined, windowMs: number): Promise<Probe[]> {
  return Promise.all(
    (pinned ? [pinned] : names).map(async (pool): Promise<Probe> => {
      const page = await Page.open(POOLS[pool].port, windowMs);
      return page instanceof Page ? { pool, state: "live", page } : { pool, ...page };
    }),
  );
}

function unavailable(pool: Pool, { state, error }: Down) {
  const { port, entry } = POOLS[pool];
  return state === "down"
    ? `the ${pool} pool is not running on port ${port}; launch it with: setsid -f gtk-launch ${entry}`
    : `the ${pool} pool on port ${port} is unresponsive (${error}), so the window may be frozen`;
}

function running(probes: Probe[]): Live[] {
  const live = probes.flatMap((p) => (p.state === "live" ? [{ pool: p.pool, page: p.page }] : []));
  if (live.length) return live;
  const why = probes.flatMap((p) => (p.state === "live" ? [] : [unavailable(p.pool, p)])).join("; ");
  throw new Error(pinned ? why : `no Grok Bot pool is live: ${why}`);
}

async function each<T>(live: Live[], work: (page: Page) => Promise<T>): Promise<(Live & { value: T })[]> {
  const lost: string[] = [];
  const settled = await Promise.all(
    live.map(async (l) => {
      try {
        return [{ ...l, value: await work(l.page) }];
      } catch (error) {
        if (!(error instanceof Lost)) throw error;
        lost.push(`${l.pool}: ${error.message}`);
        return [];
      }
    }),
  );
  const kept = settled.flat();
  if (!kept.length) throw new Error(`no Grok Bot pool answered (${lost.join("; ")})`);
  if (lost.length) console.error(`skipped pools that stopped answering (${lost.join("; ")})`);
  return kept;
}

async function call<T>(page: Page, method: string, ms: number, shape: (got?: Record<string, unknown>) => T) {
  try {
    return { value: shape(record(await page.eval(`return await window.desktop.main.${method}();`, ms))) };
  } catch (error) {
    return { error: `${method} failed: ${message(error)}` };
  }
}

function rate(page: Page, ms = CALL_MS): Promise<Rating> {
  return call(page, "getCursorWeeklyUsage", ms, (got) => ({
    percentUsed: finite(got?.percentUsed),
    nextResetMs: finite(got?.nextResetMs),
  }));
}

function onDemand(page: Page, ms: number) {
  return call(page, "getCursorUsageSummary", ms, (got): OnDemand | null => {
    const spend = record(got?.onDemand);
    if (!spend) return null;
    return {
      usedCents: finite(spend.usedCents),
      limitCents: finite(spend.limitCents),
      resetMs: finite(spend.resetTimestampMs),
    };
  });
}

function account(page: Page, ms: number) {
  return call(page, "listCursorAccounts", ms, (got) => {
    if (!Array.isArray(got?.accounts)) throw new Error("no account list");
    const active = got.accounts.map(record).find((a) => a?.id === got.activeAccountId);
    return typeof active?.email === "string" ? active.email : null;
  });
}

function spent(rating: Rating) {
  if ("error" in rating) return rating.error;
  const { percentUsed, nextResetMs } = rating.value;
  const used = percentUsed === null ? "usage unknown" : `${percentUsed.toFixed(1)}% used`;
  return `${used}, ${nextResetMs === null ? "reset unknown" : `resets ${stamp(nextResetMs)}`}`;
}

// Both accounts bill on-demand past their allowance: Trend with no spend limit, X up to $50 a month.
// So FLOOR counts a pool as empty, and a failed usage call never passes; only an unknown percentage does.
// Of the rest, the soonest reset wins; no reset sorts last, and trend wins ties.
function pick<T extends { pool: Pool } & Rating>(rated: T[]): T | undefined {
  const reset = (r: T) => ("value" in r ? (r.value.nextResetMs ?? Infinity) : Infinity);
  return rated
    .filter((r) => "value" in r && (r.value.percentUsed === null || r.value.percentUsed < FLOOR))
    .toSorted((a, b) => reset(a) - reset(b) || Number(a.pool !== "trend") - Number(b.pool !== "trend"))[0];
}

async function holders(probes: Probe[], name: string): Promise<Target[]> {
  const rosters = await each(running(probes), (page) => page.eval<Agent[]>("return agents();"));
  const want = normalize(name);
  const bots = rosters.flatMap(({ pool, page, value }) => value.map((agent) => ({ pool, page, agent })));
  const exact = bots.filter((t) => normalize(t.agent.name) === want);
  const found = exact.length ? exact : bots.filter((t) => normalize(t.agent.name).startsWith(want));
  const one = new Set(found.map((t) => t.agent.name)).size === 1;
  if (one && new Set(found.map((t) => t.pool)).size === found.length) return found;
  const list = (found.length ? found : bots).map((t) => `${t.pool} ${t.agent.name}`).join(", ");
  throw new Error(`${found.length ? "several bots match" : "no bot matches"} "${name}": ${list}`);
}

function only(found: Target[]) {
  if (found.length === 1) return found[0];
  throw new Error(`${found[0].agent.name} is in ${found.map((t) => t.pool).join(" and ")}; pin one with --pool`);
}

async function choose(found: Target[]) {
  const rated = await Promise.all(found.map(async (t) => ({ ...t, ...(await rate(t.page)) })));
  const best = pick(rated);
  if (best) return best;
  const usages = rated.map((r) => `${r.pool} ${spent(r)}`).join("; ");
  throw new Error(
    `no pool with ${found[0].agent.name} is under ${FLOOR}% used with a working usage call (${usages}); --pool overrides`,
  );
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

async function pools(probes: Probe[], flag?: string) {
  const json = flag === "--json";
  if (pinned && !json) running(probes);
  const rows = await Promise.all(
    probes.map(async (p) => {
      const base = { pool: p.pool, port: POOLS[p.pool].port, state: p.state };
      if (p.state !== "live") {
        const row = { ...base, account: null, percentUsed: null, nextResetMs: null, onDemand: null, error: p.error };
        return { row, rating: undefined };
      }
      const ms = started + POOLS_MS - Date.now();
      const [rating, demand, email] = await Promise.all([rate(p.page, ms), onDemand(p.page, ms), account(p.page, ms)]);
      const usage = "value" in rating ? rating.value : { percentUsed: null, nextResetMs: null };
      const errors = [rating, demand, email].flatMap((r) => ("error" in r ? [r.error] : []));
      const row = {
        ...base,
        account: "value" in email ? email.value : null,
        ...usage,
        onDemand: "value" in demand ? demand.value : null,
        error: errors.length ? errors.join("; ") : null,
      };
      return { row, rating: { pool: p.pool, ...rating } };
    }),
  );
  if (json) {
    console.log(JSON.stringify(rows.map((r) => r.row)));
    return;
  }
  for (const { row, rating } of rows) {
    const head = `${row.pool}\t${row.port}\t${row.state}`;
    console.log(rating ? `${head}\t${spent(rating)}\t${row.account ?? "account unknown"}` : `${head}\t${row.error}`);
  }
  const live = rows.flatMap((r) => (r.rating ? [r.rating] : []));
  if (live.length) {
    console.log(`ask picks: ${pick(live)?.pool ?? `none; no live pool is under ${FLOOR}% with a working usage call`}`);
  }
}

async function roster(probes: Probe[]) {
  for (const { pool, value } of await each(running(probes), (page) => page.eval<Agent[]>("return agents();"))) {
    for (const agent of value) console.log([pool, agent.name, agent.title].filter(Boolean).join("\t"));
  }
}

async function read(probes: Probe[], name: string, count = "20") {
  const { page, agent } = only(await holders(probes, name));
  const chat = await open(page, agent);
  for (const entry of chat.entries.slice(-Number(count))) console.log(render(entry, agent.name) + "\n");
}

function untracked({ pool, agent }: Target, what: string, error: unknown): never {
  console.error(
    `${agent.name}: ${what} (${message(error)}); do not resend; check with: bot.ts --pool ${pool} read "${agent.name}"`,
  );
  process.exit(UNTRACKED);
}

async function ask(probes: Probe[], name: string, prompt: string, timeoutS = "600") {
  const text = prompt === "-" ? await Bun.stdin.text() : prompt;
  if (!text.trim()) throw new Error("empty prompt");
  const found = await holders(probes, name);
  const target = pinned ? found[0] : await choose(found);
  console.error(`pool: ${target.pool} (pin follow-ups with --pool ${target.pool})`);
  const before = await open(target.page, target.agent);
  const since = Math.max(0, ...before.entries.map((e) => e.seq));
  await compose(target.page, target.agent, text);
  await submit(target, since, Number(timeoutS));
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
}

async function submit(target: Target, since: number, timeoutS: number) {
  const { pool, page, agent } = target;
  const deadline = Date.now() + timeoutS * 1000;
  let doubt: string | undefined;
  await page.enter(Math.min(deadline, Date.now() + CALL_MS)).catch((error) => {
    doubt = `Enter got no acknowledgement: ${message(error)}`;
  });
  const printed = new Set<number>();
  const asks = new Map<number, string>();
  let requestId: string | undefined;
  let quiet = 0;
  try {
    for (let first = true; ; first = false) {
      if (!first) await Bun.sleep(Math.max(0, Math.min(POLL_MS, deadline - Date.now())));
      const left = deadline - Date.now();
      if (!first && left <= 0) break;
      const chat = await page.eval<Chat>(
        `return chat(${since});`,
        Math.min(CALL_MS, first ? Math.max(left, PROBE_MS) : left),
      );
      requestId ??= chat.entries.find((e) => e.kind === "message" && e.role === "user")?.requestId;
      const replies = requestId ? chat.entries.filter((e) => e.kind !== "message" && e.requestId === requestId) : [];
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
    }
  } catch (error) {
    untracked(target, requestId ? "sent, tracking lost" : "send unconfirmed", error);
  }
  if (!requestId) untracked(target, "send unconfirmed", doubt ?? `no sent message appeared within ${timeoutS}s`);
  console.error(
    `${agent.name} is still working after ${timeoutS}s; check later with: bot.ts --pool ${pool} read "${agent.name}"`,
  );
  process.exit(WORKING);
}

async function file(probes: Probe[], name: string, fileName: string) {
  const { page, agent } = only(await holders(probes, name));
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

type Command = {
  usage: string;
  min: number;
  max: number;
  windowMs: number;
  check?: (args: string[]) => string | undefined;
  run: (probes: Probe[], ...args: string[]) => Promise<void>;
};

const COMMANDS: Record<string, Command> = {
  pools: {
    usage: "pools [--json]",
    min: 0,
    max: 1,
    windowMs: 0,
    check: ([flag]) => (flag === undefined || flag === "--json" ? undefined : `unknown pools option ${flag}`),
    run: pools,
  },
  roster: { usage: "roster", min: 0, max: 0, windowMs: CONNECT_MS, run: roster },
  read: {
    usage: "read <bot> [count]",
    min: 1,
    max: 2,
    windowMs: CONNECT_MS,
    check: ([, count]) =>
      count === undefined || /^[1-9]\d*$/.test(count) ? undefined : "count must be a positive integer",
    run: read,
  },
  ask: {
    usage: "ask <bot> <prompt | -> [timeout-seconds]",
    min: 2,
    max: 3,
    windowMs: CONNECT_MS,
    check: ([, , timeout]) => {
      const seconds = Number(timeout ?? "600");
      return seconds > 0 && seconds <= DAY_S ? undefined : `timeout must be over 0 and at most ${DAY_S} seconds`;
    },
    run: ask,
  },
  file: { usage: "file <bot> <attachment-name>", min: 2, max: 2, windowMs: CONNECT_MS, run: file },
};

const USAGE = [
  `usage: bot.ts [--pool ${names.join("|")}] <command>`,
  ...Object.values(COMMANDS).map((c) => `       ${c.usage}`),
].join("\n");

function parse(argv: string[]) {
  const flagged = argv[0] === "--pool";
  const pool = flagged ? argv[1] : undefined;
  const [name, ...args] = flagged ? argv.slice(2) : argv;
  if (flagged && !Object.hasOwn(POOLS, pool ?? "")) return { error: `unknown pool ${pool ?? "(missing)"}` };
  const command = name && Object.hasOwn(COMMANDS, name) ? COMMANDS[name] : undefined;
  if (!command) return { error: name ? `unknown command ${name}` : "missing command" };
  if (args.length < command.min || args.length > command.max) return { error: `${name} takes: ${command.usage}` };
  const invalid = command.check?.(args);
  if (invalid) return { error: invalid };
  return { pool: pool as Pool | undefined, command, args };
}

const started = Date.now();
const parsed = parse(Bun.argv.slice(2));
if ("error" in parsed) {
  console.error(`${parsed.error}\n${USAGE}`);
  process.exit(1);
}
const { pool: pinned, command, args } = parsed;

const probes = await connect(pinned, command.windowMs);
try {
  await command.run(probes, ...args);
} catch (error) {
  console.error(message(error));
  process.exitCode = 1;
} finally {
  for (const p of probes) if (p.state === "live") p.page.close();
}
