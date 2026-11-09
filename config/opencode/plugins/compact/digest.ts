import { Question } from "@opencode/schema/question";
import { SessionMessage } from "@opencode/schema/session-message";
import { Schema } from "effect";
import type { Open } from "../mail/mail.ts";
import { fences, parse } from "../workflow/parse.ts";
import { start, update, type Flow } from "../workflow/state.ts";

export const Message = Schema.toEncoded(
  Schema.Union([SessionMessage.User, SessionMessage.Assistant, SessionMessage.Idle]),
);
export type Message = typeof Message.Type;

export type Session = { id: string; messages: readonly Message[] };

export type Workflow = { text?: string } | { failed: true };

export type Lane = { id: string; name: string; agent?: string; state: string };

export type Live = {
  at: number;
  git?: { branch: string; head: string; dirty: readonly string[] };
  lanes: readonly Lane[];
  mail: readonly Open[];
};

const PASTE = 4_000;
const REPLY = 16_000;
const USER = 5_000 * 4;
const DIRTY = 40;

const Asked = Schema.Struct({ questions: Schema.Array(Question.Prompt) });
const Answered = Schema.Struct({ answers: Schema.Array(Question.Answer) });

const Steps = Schema.optionalKey(Schema.Array(Schema.Int));
const Call = Schema.Struct({
  start: Schema.optionalKey(Schema.Union([Schema.Literal(true), Schema.String])),
  done: Steps,
  skip: Steps,
  fail: Steps,
  repair: Steps,
  end: Schema.optionalKey(Schema.Literal(true)),
});

export const Kept = Schema.Struct({ text: Schema.String, at: Schema.Number });
export type Kept = typeof Kept.Type;

export type Brief = { kept?: Kept; fresh: boolean; ended: boolean };

export function digest(session: Session, brief: Brief, workflow: Workflow, live: Live) {
  return [
    "# Compaction digest",
    [
      "The compact plugin built this digest from session history and live probes without a model call; the chat shows no summary.",
      `This session is \`${session.id}\`; history scouts read it with the \`sessions\` tool.`,
      "Precedence: Live and the stored workflow take priority over later user words, which take priority over the brief.",
      "User is oldest first; Reply is the newest turn-final reply.",
    ].join(" "),
    `## User\n\n${said(session)}`,
    `## Reply\n\n${replies(session.messages)}`,
    `## Finished workflows\n\n${finished(session.messages)}`,
    `## Brief\n\n${briefing(brief)}`,
    `## Workflow\n\n${flow(workflow)}`,
    probe(live),
  ].join("\n\n");
}

function briefing({ kept, fresh, ended }: Brief) {
  if (kept && fresh) return kept.text;
  if (kept)
    return `Carried forward: no brief was written for this compaction, so this is the latest brief, written before the compaction at ${stamp(kept.at)}.\n\n${kept.text}`;
  return ended
    ? "No brief was written: the plugin compacted this session automatically when its workflow ended."
    : "No brief was written: this compaction came from a typed `/compact`, the plugin's context ceiling, or the host's context limit.";
}

function said(session: Session) {
  const entries = session.messages.flatMap((message) => {
    if (message.type === "user")
      return [{ user: true, text: tagged("message", message.time.created, clip(message.text)) }];
    if (message.type !== "assistant") return [];
    return message.content.flatMap((part) =>
      part.type === "tool" && part.name === "question"
        ? [{ user: false, text: tagged("question", message.time.created, asked(part)) }]
        : [],
    );
  });
  const pin = entries.findIndex((entry) => entry.user);
  const ask = entries[pin]?.text;
  if (entries.length === 0) return "No user messages.";
  const note = (omitted: number) =>
    [
      `${omitted} older user messages and questions omitted to fit the ~5K-token budget; the session store keeps them.`,
      `Read them with the \`sessions\` tool: \`query: "text"\`, \`role: "user"\`, \`session: "${session.id}"\`, paging with \`offset\`;`,
      `questions are tool calls under \`query: "tools"\`, \`tool: "question"\`.`,
    ].join(" ");
  const rest = entries.flatMap((entry, index) => (index === pin ? [] : [entry.text]));
  const room = USER - "## User\n\n".length - (ask?.length ?? 0) - note(rest.length).length - 4;
  const omitted = rest.length - newest(rest, room, pin < rest.length ? 1 : 0).length;
  // Map rest's kept suffix back to entries without duplicating the pinned user message.
  const from = pin < 0 || omitted < pin ? omitted : omitted + 1;
  return [
    ...(ask !== undefined && pin < from ? [ask] : []),
    ...(omitted > 0 ? [note(omitted)] : []),
    ...entries.slice(from).map((entry) => entry.text),
  ].join("\n\n");
}

type Part = Extract<Extract<Message, { type: "assistant" }>["content"][number], { type: "tool" }>;

function asked(part: Part) {
  const state = part.state;
  const questions = Schema.is(Asked)(state.input) ? state.input.questions : [];
  const answers =
    state.status === "completed" && Schema.is(Answered)(state.metadata) ? state.metadata.answers : undefined;
  const lines = questions.flatMap((prompt, index) => {
    const answer = answers?.[index];
    const labels = new Set(prompt.options.map((option) => option.label));
    const reply =
      state.status === "error"
        ? `Dismissed: ${state.error.message}`
        : answer === undefined
          ? `Not answered (${state.status})`
          : answer.length === 0
            ? "Unanswered"
            : answer.map((text) => (labels.has(text) ? text : `${text} (typed)`)).join("; ");
    return [`Q (${prompt.header}): ${prompt.question}`, `A: ${clip(reply)}`];
  });
  return lines.join("\n") || "A question call with no readable questions.";
}

function replies(messages: readonly Message[]) {
  let final: { at: number; text: string } | undefined;
  let open: { at: number; text: string } | undefined;
  let stuck = false;
  let finish: string | undefined;
  for (const message of messages) {
    if (message.type === "idle" && message.outcome !== "succeeded") stuck ||= open !== undefined;
    else if (message.type === "idle" && open && !stuck) {
      final = open;
      open = undefined;
    }
    if (message.type !== "assistant") continue;
    finish = message.finish;
    const text = spoken(message);
    if (!text) continue;
    open = { at: message.time.created, text };
    stuck = false;
  }
  if (open && !stuck && finish === "stop") {
    final = open;
    open = undefined;
  }
  return [
    final ? tagged("reply", final.at, clip(final.text, REPLY)) : "No turn-final replies.",
    ...(open
      ? [
          `In-flight commentary from the unfinished turn, not a final reply:\n\n${tagged("commentary", open.at, clip(open.text, REPLY))}`,
        ]
      : []),
  ].join("\n\n");
}

function spoken(message: Extract<Message, { type: "assistant" }>) {
  return message.content
    .flatMap((part) => (part.type === "text" ? [part.text] : []))
    .join("\n\n")
    .trim();
}

function finished(messages: readonly Message[]) {
  const lines: string[] = [];
  let current: Flow | undefined;
  let fence: string | undefined;
  for (const message of messages) {
    if (message.type !== "assistant") continue;
    for (const part of message.content) {
      if (part.type === "text")
        fence =
          fences(part.text).findLast(({ text, partial }) => !partial && parse(text).steps.length > 0)?.text ?? fence;
      if (part.type !== "tool" || part.name !== "workflow" || part.state.status !== "completed") continue;
      const input = part.state.input;
      if (!Schema.is(Call)(input)) continue;
      const next = replay(current, fence, input);
      if (!next) continue;
      current = input.end ? undefined : next;
      if (input.end) lines.push(closed(next, message.time.created));
    }
  }
  return lines.join("\n") || "No workflow ended in this session.";
}

function replay(current: Flow | undefined, fence: string | undefined, input: typeof Call.Type) {
  const source = input.start === true || input.start?.trim() === "true" ? fence : input.start;
  try {
    const next = source === undefined ? current : start(current, source);
    return next && update(next, { done: input.done, skip: input.skip, fail: input.fail, repair: input.repair });
  } catch {
    return undefined;
  }
}

function closed(run: Flow, at: number) {
  const { title, steps } = parse(run.source);
  const groups = (["done", "skipped", "failed", undefined] as const).flatMap((mark) => {
    const numbers = steps.filter(({ n }) => run.marks[n] === mark).map(({ n }) => n);
    return numbers.length ? [`${mark ?? "unfinished"} ${numbers.join(", ")}`] : [];
  });
  return `- ${stamp(at)} ${title} (rev ${run.rev}): ${groups.join("; ") || "no steps"}`;
}

function newest(entries: readonly string[], room: number, least = 1) {
  let left = room;
  let kept = 0;
  for (const entry of entries.toReversed()) {
    if (kept >= least && entry.length + 2 > left) break;
    left -= entry.length + 2;
    kept += 1;
  }
  return entries.slice(entries.length - kept);
}

function flow(workflow: Workflow) {
  if ("failed" in workflow)
    return "The stored workflow could not be read during compaction; call `workflow` with `{}` first.";
  return workflow.text ?? "No workflow is stored.";
}

function probe(live: Live) {
  const git = live.git && [
    `- Git: branch \`${live.git.branch}\` at \`${live.git.head}\`; ${live.git.dirty.length} dirty paths${live.git.dirty.length ? ":" : "."}`,
    ...live.git.dirty.slice(0, DIRTY).map((line) => `  - \`${line}\``),
    ...(live.git.dirty.length > DIRTY ? [`  - and ${live.git.dirty.length - DIRTY} more`] : []),
  ];
  const lanes = live.lanes.length
    ? [
        "- Lanes:",
        ...live.lanes.map(
          (lane) => `  - \`${lane.name}\` (${[lane.agent, lane.id].filter(Boolean).join(", ")}): ${lane.state}`,
        ),
      ]
    : ["- Lanes: none open."];
  const mail = live.mail.length
    ? ["- Unanswered mail asks:", ...live.mail.map(letter)]
    : ["- Unanswered mail asks: none."];
  return [`## Live (probed ${stamp(live.at, true)})`, [...(git ?? []), ...lanes, ...mail].join("\n")].join("\n\n");
}

function letter(open: Open) {
  const what = `"${open.mail.subject}" (${open.mail.id}, sent ${stamp(open.mail.sent)})`;
  return open.state === "owed"
    ? `  - Owed: ${open.name} (${open.peer}) asked ${what}`
    : `  - Awaiting: ${open.name} (${open.peer}) has not answered ${what}`;
}

function tagged(name: string, at: number, text: string) {
  return `<${name} at="${stamp(at)}">\n\n${text}\n\n</${name}>`;
}

function clip(text: string, limit = PASTE) {
  return text.length > limit ? `${text.slice(0, limit)}\n[cut: ${text.length - limit} more characters]` : text;
}

function stamp(at: number, seconds = false) {
  return new Date(at).toLocaleString("sv-SE").slice(0, seconds ? 19 : 16);
}
