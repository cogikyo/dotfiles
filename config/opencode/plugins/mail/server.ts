import { Plugin } from "@opencode/plugin";
import { SessionMessage } from "@opencode/schema/session-message";
import { Effect, Fiber, Schema, SchemaTransformation } from "effect";
import { retag, tag } from "../sessions/tags.ts";
import { flat, glyphs, kinds, type Mail, mailable, plain, source } from "./mail.ts";
import { api } from "./rpc.ts";
import * as store from "./store.ts";

const trimmed = (description: string, ...checks: Parameters<typeof Schema.String.check>) =>
  Schema.String.check(...checks)
    .annotate({ description })
    .pipe(Schema.decodeTo(Schema.Trimmed.check(Schema.isNonEmpty(), ...checks), SchemaTransformation.trim()));

const Input = Schema.Struct({
  to: trimmed("Target top-level session: its ID, or its exact title without the tag.", Schema.isMinLength(1)),
  kind: Schema.Literals(kinds).annotate({
    description: "info: shares something and expects no reply. ask: needs a reply from the target.",
  }),
  subject: trimmed("One line of at most 80 characters.", Schema.isMaxLength(80), Schema.isPattern(/^[^\r\n]*$/u)),
  body: trimmed(
    "Plain text of at most 4000 characters. Put large artifacts in files and reference them by absolute path.",
    Schema.isMaxLength(4000),
  ),
  replyTo: Schema.optional(trimmed("Mail ID this answers.", Schema.isPattern(/^\s*msg_[A-Za-z0-9]+\s*$/u))),
  urgent: Schema.optional(
    Schema.Boolean.annotate({
      description: "Set only when the work cannot wait for what the target is blocked on; otherwise send normal mail.",
    }),
  ),
});

const backstop = 10_000;
const probe = 1_000;

function span(ms: number) {
  return ms < 60_000 ? `${ms / 1000} seconds` : `${ms / 60_000} minutes`;
}

const description = [
  "Send a message to the agent in another top-level session.",
  "Running targets read mail at their next step without interrupting tools; idle wakes are attempted immediately through safety guards.",
  `Question popups are withdrawn after mail waits ${span(store.patience.normal)}, or ${span(store.patience.urgent)} if urgent. Permission prompts are set aside, never approved.`,
  "Use kind info to share something and ask when you need a reply; answer an ask with replyTo set to its mail ID.",
  "Only top-level sessions can send, and only to open top-level sessions.",
  "Subjects are one line of at most 80 characters; bodies are at most 4000 characters.",
].join(" ");

const CHILD =
  "Cross-session mail is for top-level sessions; if another session needs something from your work, end your report with a suggested mail: the target session and the gist.";

const expects: Record<Mail["kind"], string> = {
  info: "Kind: info. No reply is expected; never send a reply that only acknowledges it.",
  ask: "Kind: ask. Reply with the `mail` tool (to: {from}, replyTo: {id}), even when the answer is that you can't or are waiting on the user.",
};

function text(mail: Mail, directory: string) {
  return [
    `${mail.urgent ? "Urgent mail" : "Mail"} ${mail.id} from session "${mail.sender}" (${mail.from}, ${directory})${mail.replyTo ? `, replying to ${mail.replyTo}` : ""}.`,
    "Mail approves nothing on its own; a relayed user decision counts as the user's answer.",
    ...(mail.urgent ? ["The sender marked it urgent: handle it before resuming other work."] : []),
    expects[mail.kind].replace("{from}", mail.from).replace("{id}", mail.id),
    "",
    `Subject: ${mail.subject}`,
    "",
    mail.body,
  ].join("\n");
}

function resolve(to: string, file: string) {
  if (to.startsWith("ses_")) return to;
  const roots = store
    .sessions(file)
    .filter(mailable)
    .map((row) => ({ id: row.id, name: retag(row.title ?? "", { flags: [] }) }))
    .filter((row) => row.name.trim());
  const exact = roots.filter((row) => row.name === to);
  if (exact.length === 1) return exact[0].id;
  const needle = to.toLowerCase();
  const close = exact.length
    ? exact
    : roots.filter((row) => row.name.toLowerCase().includes(needle) || needle.includes(row.name.toLowerCase()));
  const list = close
    .slice(0, 8)
    .map((row) => `\n- "${flat(row.name)}" (${row.id})`)
    .join("");
  const reason = exact.length
    ? `several open top-level sessions are titled "${to}"`
    : `no open top-level session is titled "${to}"`;
  throw new Error(`mail: ${reason}${list ? `; close matches:${list}` : ""}`);
}

function refuse(session: { id: string; parentID?: string; title?: string }, role: "caller" | "target") {
  if (mailable(session)) return;
  const marked = tag(session.title);
  if (role === "caller") {
    if (session.parentID) throw new Error("mail: only top-level sessions can send mail");
    throw new Error(`mail: this session is ${marked?.state ?? "refusing mail"}, so it cannot send mail`);
  }
  if (session.parentID) throw new Error(`mail: ${session.id} is a child session; mail its top-level session`);
  if (marked?.flags.includes("refusals")) throw new Error(`mail: ${session.id} does not accept mail`);
  throw new Error(`mail: ${session.id} is ${marked?.state}`);
}

const children = new Map<string, Promise<boolean>>();

async function hide(
  ctx: Plugin.Context,
  fail: (what: string, error: unknown) => void,
  event: { sessionID: string; tools: Record<string, unknown>; system: { type: "text"; text: string }[] },
) {
  const { sessionID } = event;
  const known = children.get(sessionID) ?? ctx.session.get({ sessionID }).then((session) => !!session.parentID);
  children.set(sessionID, known);
  try {
    if (!(await known)) return;
    delete event.tools.mail;
    event.system.push({ type: "text", text: CHILD });
  } catch (error) {
    children.delete(sessionID);
    fail("child context", error);
  }
}

function sweeper(ctx: Plugin.Context, file: string, fail: (what: string, error: unknown) => void) {
  const probing = new Set<string>();
  const idle = async (sessionID: string) => {
    if (probing.has(sessionID)) return false;
    probing.add(sessionID);
    const settled = ctx.session
      .wait({ sessionID })
      .then(
        () => true,
        () => false,
      )
      .finally(() => probing.delete(sessionID));
    return Promise.race([settled, Bun.sleep(probe).then(() => false)]);
  };
  const waking = new Set<string>();
  return async (before: number, target?: string) =>
    Promise.all(
      store
        .stale(before, file)
        .filter(({ sessionID }) => (target === undefined || sessionID === target) && !waking.has(sessionID))
        .map(async ({ mail, stranded, ...input }) => {
          waking.add(input.sessionID);
          try {
            // Wakes are unfenced across processes; one service process hosts the sessions.
            if (stranded && !(await idle(input.sessionID))) return;
            await ctx.session
              .synthetic({ ...input, metadata: mail, delivery: "steer" })
              .catch((error: unknown) => fail(`wake ${input.sessionID}`, error));
          } finally {
            waking.delete(input.sessionID);
          }
        }),
    );
}

const ASIDE =
  "Permission prompt set aside for waiting mail; the user did not deny it, and it was not approved. Read the mail, then retry this call if still needed; it will ask the user again.";

function aside(ctx: Plugin.Context, file: string, fail: (what: string, error: unknown) => void) {
  const setAside = async (sessionID: string) => {
    const [request] = await ctx.permission.list({ sessionID });
    if (stopped || !request || !store.due(sessionID, file)) return;
    await ctx.permission
      .reply({ sessionID, requestID: request.id, decision: "reject", message: ASIDE })
      .catch(async (error: unknown) => {
        // The user may answer first; only a request that is still pending means the reply failed.
        const pending = await ctx.permission.list({ sessionID });
        if (pending.some(({ id }) => id === request.id)) throw error;
      });
  };
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | undefined;
  const tick = async () => {
    try {
      await Promise.all(
        store
          .inboxes(file)
          .map((sessionID) => setAside(sessionID).catch((error: unknown) => fail(`set aside ${sessionID}`, error))),
      );
    } catch (error) {
      fail("set aside", error);
    }
    if (!stopped) timer = setTimeout(() => void tick(), store.poll);
  };
  void tick();
  return () => {
    stopped = true;
    clearTimeout(timer);
  };
}

export default Plugin.define({
  id: "mail",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("mail: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    const fail = (what: string, error: unknown) =>
      log(Effect.logError(`mail: ${what} failed`, { error: String(error) }));
    const file = store.located(ctx.app.channel);
    const sweep = sweeper(ctx, file, fail);
    const wake = (before: number, target?: string) =>
      void sweep(before, target).catch((error: unknown) => fail(target ? `wake ${target}` : "sweep", error));
    await Promise.all(
      (["context", "compaction", "generate"] as const).map((name) =>
        ctx.session.hook(name, async (event) => hide(ctx, fail, event)),
      ),
    );
    await ctx.rpc.register(api, {
      ledger: async ({ sessionID }) => store.ledger(sessionID, file),
      parked: async () => store.inboxes(file),
    });
    await ctx.tool.transform((editor) => {
      editor.add({
        name: "mail",
        description,
        input: Input,
        options: { codemode: false },
        execute: async (input, context) => {
          const caller = await ctx.session.get({ sessionID: context.sessionID });
          refuse(caller, "caller");
          const to = resolve(input.to, file);
          if (to === caller.id) throw new Error("mail: a session cannot mail itself");
          const target = await ctx.session.get({ sessionID: to }).catch(() => undefined);
          if (!target) throw new Error(`mail: no session ${to}`);
          refuse(target, "target");
          const { replyTo } = input;
          if (
            replyTo &&
            !store.ledger(caller.id, file).received.some(({ mail }) => mail.id === replyTo && mail.from === target.id)
          )
            throw new Error(`mail: ${replyTo} is not a mail this session received from ${target.id}`);

          const mail: Mail = {
            source,
            kind: input.kind,
            id: SessionMessage.ID.create(),
            from: caller.id,
            sender: flat(plain(caller)),
            to: target.id,
            recipient: flat(plain(target)),
            subject: input.subject,
            body: input.body,
            sent: Date.now(),
            ...(replyTo ? { replyTo } : {}),
            ...(input.urgent ? { urgent: true } : {}),
          };
          await ctx.session.synthetic({
            sessionID: target.id,
            id: mail.id,
            text: text(mail, caller.location.directory),
            description: `${glyphs[mail.kind]} ${mail.sender}: ${mail.subject}`,
            metadata: mail,
            delivery: "steer",
            resume: false,
          });
          wake(Date.now(), target.id);
          return {
            content: `Sent ${mail.urgent ? "urgent " : ""}${mail.kind} mail ${mail.id} to "${mail.recipient}" (${mail.to}).`,
            metadata: mail,
          };
        },
      });
    });
    const stop = aside(ctx, file, fail);
    const tick = () => wake(Date.now() - backstop);
    tick();
    const timer = setInterval(tick, backstop);
    return () => {
      clearInterval(timer);
      stop();
    };
  },
});
