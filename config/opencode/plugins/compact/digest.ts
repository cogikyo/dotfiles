import { Question } from "@opencode/schema/question";
import { SessionMessage } from "@opencode/schema/session-message";
import { Schema } from "effect";
import type { Open } from "../mail/mail.ts";

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
const USER = 5_000 * 4;
const REPLIES = 15_000 * 4;
const NOTE = 80;
const DIRTY = 40;

const Asked = Schema.Struct({ questions: Schema.Array(Question.Prompt) });
const Answered = Schema.Struct({ answers: Schema.Array(Question.Answer) });

function precedence(last: string) {
  return `Precedence: Live and the stored workflow take priority over later user words, which take priority over ${last}.`;
}

const HEAD = "# Compaction digest";
const NEWEST = "User and Replies are newest first.";
const UNBRIEFED = "No brief was written: the plugin compacted this session automatically when its workflow ended.";

export function digest(session: Session, brief: string | undefined, workflow: Workflow, live: Live) {
  return [
    HEAD,
    [
      "The compact plugin built this digest from session history and live probes without a model call.",
      precedence("the brief"),
      "User and Replies are oldest first.",
    ].join(" "),
    `## User\n\n${said(session)}`,
    `## Replies\n\n${replies(session.messages)}`,
    `## Brief\n\n${brief ?? UNBRIEFED}`,
    `## Workflow\n\n${flow(workflow)}`,
    probe(live),
  ].join("\n\n");
}

export function recall(session: Session, workflow: Workflow, live: Live) {
  return [
    [
      "This compaction used a model summary without a handoff brief.",
      "The compact plugin added these records from session history and live probes at compaction time.",
      precedence("the summary"),
      "User is oldest first.",
    ].join(" "),
    `## User\n\n${said(session)}`,
    `## Workflow\n\n${flow(workflow)}`,
    probe(live),
  ].join("\n\n");
}

export function objective(summary: string) {
  if (!summary.startsWith(HEAD))
    return summary
      .split(/\n(?=#{1,6} )/u)
      .find((section) => section.trim())
      ?.trim();
  const start = summary.indexOf("\n## Brief\n\n") + "\n## Brief\n\n".length;
  const brief = summary.slice(start, summary.lastIndexOf("\n\n## Workflow\n\n")).trim();
  if (brief && brief !== UNBRIEFED) return brief;
  const head = summary.slice(0, summary.indexOf("\n\n## User\n\n"));
  const user = summary.slice(head.length, summary.indexOf("\n\n## Replies\n\n"));
  const asks = [...user.matchAll(/<message at="[^"]*">\n\n?([\s\S]*?)\n\n?<\/message>/gu)];
  return asks.at(head.includes(NEWEST) ? -1 : 0)?.[1];
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
  const finals: { at: number; text: string }[] = [];
  let last: { at: number; text: string } | undefined;
  for (const message of messages) {
    if (message.type === "idle") {
      if (last) finals.push(last);
      last = undefined;
    }
    if (message.type !== "assistant") continue;
    const text = message.content
      .flatMap((part) => (part.type === "text" ? [part.text] : []))
      .join("\n\n")
      .trim();
    if (text) last = { at: message.time.created, text };
  }
  if (last) finals.push(last);
  const entries = finals.map((reply) => tagged("reply", reply.at, reply.text));
  const kept = newest(entries, REPLIES - "## Replies\n\n".length - NOTE);
  const omitted = entries.length - kept.length;
  const note = omitted > 0 ? [`${omitted} older replies omitted to fit the ~15K-token budget.`] : [];
  return [...note, ...kept].join("\n\n") || "No replies.";
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

function clip(text: string) {
  return text.length > PASTE ? `${text.slice(0, PASTE)}\n[cut: ${text.length - PASTE} more characters]` : text;
}

function stamp(at: number, seconds = false) {
  return new Date(at).toLocaleString("sv-SE").slice(0, seconds ? 19 : 16);
}
