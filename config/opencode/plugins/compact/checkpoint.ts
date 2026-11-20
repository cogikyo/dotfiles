import { Schema } from "effect";
import type { Entry } from "./history.ts";

const BUDGET = 64_000;
const MESSAGE = 8_000;
const REPLY = 16_000;

type Said = Extract<Entry, { kind: "user" | "answer" }>;
type Spoke = Extract<Entry, { kind: "assistant" }>;

export const Ends = Schema.Struct({ end: Schema.Literal(true) });

export function recovery(sessionID: string) {
  return `Compaction finished. This session is \`${sessionID}\`. Load the \`recommended\` skill and follow it.`;
}

function boundary(entries: readonly Entry[]) {
  return entries.findLast(
    (entry): entry is Spoke =>
      entry.kind === "assistant" &&
      entry.parts.some((part) => part.type === "tool" && part.status === "completed" && Schema.is(Ends)(part.input)),
  );
}

export function render(facts: {
  sessionID: string;
  at: number;
  workflow: string;
  lanes: string;
  git: string;
  history: readonly Entry[] | string;
}) {
  const found = typeof facts.history === "string" ? facts.history : boundary(facts.history);
  const head = [
    "# Checkpoint",
    `The compact plugin built this checkpoint at ${stamp(facts.at)} from stored session history and live probes, without a model call. Sections are cut only where an omission note says so.`,
    `## Boundary\n\n${edge(found)}`,
    `## Workflow\n\n${facts.workflow}`,
    `## Lanes\n\n${facts.lanes}`,
    `## Git\n\n${facts.git}`,
  ].join("\n\n");
  if (typeof facts.history === "string")
    return folded(facts.sessionID, [head, `## User words\n\n${facts.history}`, `## Last reply\n\n${facts.history}`]);
  const seq = typeof found === "object" ? found.seq : -1;
  const said = facts.history.filter((entry): entry is Said => entry.kind === "user" || entry.kind === "answer");
  const recent = said.filter((entry) => entry.seq > seq).toReversed();
  const older = said.filter((entry) => entry.seq < seq).toReversed();
  const reply = `## Last reply\n\n${replies(facts.history)}`;
  let room = BUDGET - recovery(facts.sessionID).length - head.length - reply.length - 400;
  const fill = (list: readonly Said[], least: number) => {
    const kept: string[] = [];
    for (const entry of list) {
      const text = quote(entry);
      if (kept.length >= least && text.length + 2 > room) break;
      room -= text.length + 2;
      kept.push(text);
    }
    return kept;
  };
  const words = fill(recent, 1);
  const rest = fill(older, recent.length ? 0 : 1);
  const sections = [
    head,
    `## User words\n\n${listed(words, recent, "user messages since the boundary") || "No user messages since the boundary."}`,
    reply,
  ];
  if (older.length) sections.push(`## Older user words\n\n${listed(rest, older, "user messages before the boundary")}`);
  return folded(facts.sessionID, sections);
}

function folded(sessionID: string, sections: readonly string[]) {
  const body = sections.join("\n\n");
  const longest = (body.match(/`+/gu) ?? []).reduce((most, run) => Math.max(most, run.length), 0);
  const fence = "`".repeat(Math.max(3, longest + 1));
  return `${recovery(sessionID)}\n\n${fence}checkpoint\n${body}\n${fence}`;
}

function edge(found: Spoke | string | undefined) {
  if (found === undefined) return "Session start: no workflow has ended in this session.";
  if (typeof found === "string") return found;
  return `A workflow ended successfully at ${stamp(found.at)} in \`${found.id}\` (seq ${found.seq}).`;
}

function listed(kept: readonly string[], all: readonly Said[], what: string) {
  const dropped = all.slice(kept.length);
  if (!dropped.length) return kept.join("\n\n");
  const note = `[omitted: ${dropped.length} older ${what}, \`${dropped.at(-1)?.id}\` through \`${dropped[0]?.id}\`, to fit the checkpoint budget; read them with the \`sessions\` tool]`;
  return [...kept, note].join("\n\n");
}

function quote(entry: Said) {
  const files = entry.kind === "user" && entry.files ? ` files="${entry.files}"` : "";
  return tagged(entry.kind, entry, clip(entry.text, MESSAGE), files);
}

function replies(entries: readonly Entry[]) {
  let final: Spoke | undefined;
  let open: typeof final;
  let stuck = false;
  let finish: string | undefined;
  for (const entry of entries) {
    if (entry.kind === "idle" && entry.outcome !== "succeeded") stuck ||= open !== undefined;
    else if (entry.kind === "idle" && open && !stuck) {
      final = open;
      open = undefined;
    }
    if (entry.kind !== "assistant") continue;
    finish = entry.finish;
    if (!spoken(entry)) continue;
    open = entry;
    stuck = false;
  }
  if (open && !stuck && finish === "stop") {
    final = open;
    open = undefined;
  }
  return [
    final ? show(final, "reply") : "No turn-final reply.",
    ...(open ? [`In-flight text from the unfinished turn, not a final reply:\n\n${show(open, "commentary")}`] : []),
  ].join("\n\n");
}

function show(entry: Spoke, name: string) {
  return tagged(name, entry, clip(spoken(entry), REPLY));
}

function spoken(entry: Spoke) {
  return entry.parts
    .flatMap((part) => (part.type === "text" ? [part.text] : []))
    .join("\n\n")
    .trim();
}

function tagged(name: string, { id, at }: { id: string; at: number }, text: string, extra = "") {
  return `<${name} id="${id}" at="${stamp(at)}"${extra}>\n\n${text}\n\n</${name}>`;
}

function clip(text: string, limit: number) {
  if (text.length <= limit) return text;
  const half = Math.floor(limit / 2);
  return `${text.slice(0, half)}\n\n[clipped: ${text.length - limit} characters omitted from the middle of this message]\n\n${text.slice(-half)}`.toWellFormed();
}

function stamp(at: number) {
  return new Date(at).toLocaleString("sv-SE").slice(0, 16);
}
