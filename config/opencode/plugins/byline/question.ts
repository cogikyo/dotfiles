import {
  BoxRenderable,
  StyledText,
  TextAttributes,
  TextRenderable,
  Yoga,
  fg,
  type CliRenderer,
  type RGBA,
  type Renderable,
  type TextChunk,
} from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import { owned, type Ledger } from "./ledger.ts";
import { padding } from "./lane.ts";
import { clip } from "./notice.ts";

type Context = Plugin.Context;
type Message = NonNullable<ReturnType<Context["data"]["session"]["message"]["get"]>>;
type Tool = Extract<Extract<Message, { type: "assistant" }>["content"][number], { type: "tool" }>;
type Option = { label: string; name: string; star: boolean; description: string };
type Ask = { header: string; question: string; options: readonly Option[] };
type Answer = readonly string[] | undefined;
type Form = { asks: readonly Ask[]; answers?: readonly (readonly string[])[] };
type Tones = { accent: RGBA; text: RGBA; muted: RGBA; rule: RGBA; agreed: RGBA; star: RGBA };
type Paint = { renderer: CliRenderer; tones: Tones; column: number; room: number };
type Scan = { context: Context; sessionID: string; ledger: Ledger };

const recommended = /\s*\(recommended\)\s*$/i;
const headings = /^(# Questions|Asked \d+ questions?|(⋯ )?Asking questions…)$/;
const shown = new WeakMap<Renderable, { key: string; node: Renderable }>();

export function question(row: Renderable, blocks: readonly Renderable[], { context, sessionID, ledger }: Scan) {
  const [block, ...extra] = blocks;
  const part = find(row, block, context, sessionID);
  if (part?.name !== "question") return false;
  const form = parse(part.state);
  if (!form || !block || extra.length > 0 || !hosts(block, form)) {
    revert(row, blocks, ledger);
    return true;
  }
  const tones = palette(context);
  const column = Math.max(1, inset(block));
  const room = Math.max(0, Math.trunc(row.width) - column - 2);
  const key = JSON.stringify([form, Object.values(tones).map((tone) => tone.toInts()), column, room]);
  const known = shown.get(row);
  const fresh = known?.key === key && !known.node.isDestroyed;
  const node = fresh ? known.node : build(form, { renderer: context.renderer, tones, column, room });
  if (known && known.node !== node) unmount(known.node, ledger);
  shown.set(row, { key, node });
  ledger.hide(block);
  if (node.parent !== row) {
    row.add(node);
    ledger.record(node, node, () => node.destroyRecursively());
  }
  return true;
}

function find(row: Renderable, block: Renderable | undefined, { data }: Context, sessionID: string) {
  const [, messageID, partID] = /^session-part:([^:]+):(.+)$/.exec(row.id) ?? [];
  const message = data.session.message.get(sessionID, messageID ?? row.id);
  if (message?.type !== "assistant" || (partID === undefined && !heading(block))) return;
  return message.content.find(
    (part): part is Tool =>
      part.type === "tool" && (partID === undefined ? part.name === "question" : part.id === partID),
  );
}

function heading(block: Renderable | undefined) {
  const [first] = block?.getChildren() ?? [];
  const row = plain(first) === undefined ? (first?.getChildren() ?? []) : [];
  return [first, ...row].map(plain).find((text) => text !== undefined && headings.test(text));
}

function parse(state: unknown): Form | undefined {
  if (!record(state) || !record(state.input) || !Array.isArray(state.input.questions)) return;
  const asks = state.input.questions.map(ask);
  if (asks.length === 0 || !asks.every((item) => item !== undefined)) return;
  if (state.status === "error") return dismissed(state.error) ? { asks } : undefined;
  if (state.status !== "completed" || !record(state.metadata)) return;
  const answers = state.metadata.answers;
  if (!Array.isArray(answers) || answers.length !== asks.length || !answers.every(strings)) return;
  return { asks, answers };
}

function ask(value: unknown): Ask | undefined {
  if (!record(value) || typeof value.question !== "string" || typeof value.header !== "string") return;
  if (!Array.isArray(value.options)) return;
  const options = value.options.map(option);
  if (!options.every((item) => item !== undefined)) return;
  return { header: value.header, question: value.question, options };
}

function option(value: unknown): Option | undefined {
  if (!record(value) || typeof value.label !== "string") return;
  const description = typeof value.description === "string" ? value.description.replace(/\s+/g, " ").trim() : "";
  return {
    label: value.label,
    name: value.label.replace(recommended, ""),
    star: recommended.test(value.label),
    description,
  };
}

function dismissed(error: unknown) {
  return record(error) && typeof error.message === "string" && error.message.includes("user dismissed");
}

function hosts(block: Renderable, form: Form) {
  const title = heading(block);
  if (form.answers) return "border" in block && block.border !== false && title === "# Questions";
  return /^Asked \d+ questions?$/.test(title ?? "");
}

function inset(block: Renderable) {
  const border =
    "border" in block && (block.border === true || (Array.isArray(block.border) && block.border.includes("left")));
  return (border ? 1 : 0) + padding(block, Yoga.Edge.Left);
}

function palette(context: Context): Tones {
  const theme = context.theme;
  return {
    accent: theme.hue.accent[context.themeMode === "light" ? 300 : 200],
    text: theme.text.base,
    muted: theme.text.muted,
    rule: theme.border.base,
    agreed: theme.text.feedback.success.base,
    star: theme.hue.interactive[200],
  };
}

function build({ asks, answers }: Form, paint: Paint) {
  const { renderer, tones } = paint;
  const root = new BoxRenderable(renderer, {
    border: true,
    borderStyle: "rounded",
    borderColor: tones.accent,
    title: ` ${asks.length} ${asks.length === 1 ? "question" : "questions"} `,
    paddingLeft: paint.column - 1,
    paddingRight: 1,
    flexShrink: 0,
  });
  for (const [index, item] of asks.entries()) {
    if (index > 0) root.add(new BoxRenderable(renderer, { border: ["top"], borderColor: tones.rule, flexShrink: 0 }));
    for (const node of section(item, answers?.[index], paint)) root.add(node);
  }
  if (!answers) root.add(new TextRenderable(renderer, { content: "Dismissed", fg: tones.muted, marginTop: 1 }));
  owned.add(root);
  return root;
}

function section({ header, question: text, options }: Ask, answer: Answer, paint: Paint) {
  const { renderer, tones } = paint;
  const nodes: Renderable[] = [];
  if (header.trim()) nodes.push(line(renderer, header.trim(), tones.muted));
  nodes.push(new TextRenderable(renderer, { content: text, fg: tones.accent, attributes: TextAttributes.BOLD }));
  const given = (answer ?? []).filter((value) => value.trim());
  const width = Math.max(...options.map(span));
  for (const item of options) nodes.push(choice(item, given.includes(item.label), width, paint));
  const custom = given.filter((value) => !options.some(({ label }) => label === value));
  for (const value of custom) nodes.push(typed(value.trimEnd(), paint));
  if (answer && given.length === 0) nodes.push(line(renderer, "  No answer", tones.muted));
  return nodes;
}

function choice(item: Option, picked: boolean, width: number, { renderer, tones, room }: Paint) {
  const agreed = picked && item.star;
  const chunks: TextChunk[] = [
    fg(agreed ? tones.agreed : tones.accent)(picked ? "  ✓ " : "    "),
    fg(agreed ? tones.agreed : picked ? tones.text : tones.muted)(item.name),
  ];
  if (item.star) chunks.push(fg(agreed ? tones.agreed : tones.star)(" ★"));
  const gap = " ".repeat(width - span(item) + 2);
  const left = room - 4 - width - 2;
  if (item.description && left > 1) chunks.push(fg(tones.muted)(gap + clip(item.description, left)));
  return new TextRenderable(renderer, { content: new StyledText(chunks), wrapMode: "none" });
}

function typed(value: string, { renderer, tones }: Paint) {
  const box = new BoxRenderable(renderer, { flexDirection: "row", flexShrink: 0 });
  box.add(new TextRenderable(renderer, { content: "  ✎ ", fg: tones.accent, flexShrink: 0 }));
  box.add(new TextRenderable(renderer, { content: value, fg: tones.text, flexGrow: 1, flexShrink: 1 }));
  return box;
}

function span({ name, star }: Option) {
  return Bun.stringWidth(name) + (star ? 2 : 0);
}

function line(renderer: CliRenderer, content: string, color: RGBA) {
  return new TextRenderable(renderer, { content, fg: color, flexShrink: 0 });
}

function revert(row: Renderable, blocks: readonly Renderable[], ledger: Ledger) {
  const known = shown.get(row);
  if (!known) return;
  shown.delete(row);
  unmount(known.node, ledger);
  for (const block of blocks) ledger.restore(block);
}

function unmount(node: Renderable, ledger: Ledger) {
  ledger.restore(node);
  if (!node.isDestroyed) node.destroyRecursively();
}

function plain(node: Renderable | undefined) {
  return node && "plainText" in node && typeof node.plainText === "string" ? node.plainText : undefined;
}

function strings(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
