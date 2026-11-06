import { Question } from "@opencode/schema/question";
import { SessionMessage } from "@opencode/schema/session-message";
import { Schema } from "effect";
import type { Open } from "../mail/mail.ts";

export const Message = Schema.toEncoded(
  Schema.Union([SessionMessage.User, SessionMessage.Assistant, SessionMessage.Idle]),
);
export type Message = typeof Message.Type;

export type Workflow = { text?: string } | { failed: true };

export type Lane = { id: string; name: string; agent?: string; state: string };

export type Live = {
  at: number;
  git?: { branch: string; head: string; dirty: readonly string[] };
  lanes: readonly Lane[];
  mail: readonly Open[];
};

const PASTE = 4_000;
const REPLIES = 15_000 * 4;
const NOTE = 80;
const DIRTY = 40;

const Asked = Schema.Struct({ questions: Schema.Array(Question.Prompt) });
const Answered = Schema.Struct({ answers: Schema.Array(Question.Answer) });

function precedence(last: string) {
  return `Precedence: Live and the stored workflow take priority over later user words, which take priority over ${last}.`;
}

const HEAD = "# Compaction digest";
const UNBRIEFED = "No brief was written: the plugin compacted this session automatically when its workflow ended.";

export function digest(messages: readonly Message[], brief: string | undefined, workflow: Workflow, live: Live) {
  return [
    HEAD,
    [
      "The compact plugin built this digest from session history and live probes without a model call.",
      precedence("the brief"),
      "User and Replies are newest first.",
    ].join(" "),
    `## User\n\n${said(messages)}`,
    `## Replies\n\n${replies(messages)}`,
    `## Brief\n\n${brief ?? UNBRIEFED}`,
    `## Workflow\n\n${flow(workflow)}`,
    probe(live),
  ].join("\n\n");
}

export function recall(messages: readonly Message[], workflow: Workflow, live: Live) {
  return [
    [
      "This compaction used a model summary without a handoff brief.",
      "The compact plugin added these records from session history and live probes at compaction time.",
      precedence("the summary"),
      "User is newest first.",
    ].join(" "),
    `## User\n\n${said(messages)}`,
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
  const user = summary.slice(0, summary.indexOf("\n\n## Replies\n\n"));
  return [...user.matchAll(/<message at="[^"]*">\n([\s\S]*?)\n<\/message>/gu)].at(-1)?.[1];
}

function said(messages: readonly Message[]) {
  const entries = messages.flatMap((message) => {
    if (message.type === "user") return [tagged("message", message.time.created, clip(message.text))];
    if (message.type !== "assistant") return [];
    return message.content.flatMap((part) =>
      part.type === "tool" && part.name === "question" ? [tagged("question", message.time.created, asked(part))] : [],
    );
  });
  return entries.toReversed().join("\n\n") || "No user messages.";
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
  const newest = finals.toReversed();
  const kept: string[] = [];
  let used = "## Replies\n\n".length + NOTE;
  for (const reply of newest) {
    const entry = tagged("reply", reply.at, reply.text);
    if (kept.length > 0 && used + entry.length + 2 > REPLIES) break;
    kept.push(entry);
    used += entry.length + 2;
  }
  const omitted = newest.length - kept.length;
  const note = omitted > 0 ? [`${omitted} older replies omitted to fit the ~15K-token budget.`] : [];
  return [...kept, ...note].join("\n\n") || "No replies.";
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
  return `<${name} at="${stamp(at)}">\n${text}\n</${name}>`;
}

function clip(text: string) {
  return text.length > PASTE ? `${text.slice(0, PASTE)}\n[cut: ${text.length - PASTE} more characters]` : text;
}

function stamp(at: number, seconds = false) {
  return new Date(at).toLocaleString("sv-SE").slice(0, seconds ? 19 : 16);
}
