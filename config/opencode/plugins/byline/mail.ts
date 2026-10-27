import { isTextNodeRenderable, TextNodeRenderable, TextRenderable, type RGBA, type Renderable } from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import { LayoutSlotRenderable } from "@opentui/solid";
import { glyphs, type Mail, read, sent, source } from "../mail/mail.ts";
import { colors } from "../statusline/palette.ts";
import type { Item, Letter } from "./lane.ts";
import { native, owned, textual, type Ledger } from "./ledger.ts";
import { flatten, locate, put, whole } from "./notice.ts";

type Context = Plugin.Context;
type Data = Context["data"]["session"];
type Kind = "mail" | "sent";
type Found = Extract<Item, { kind: Kind }>;
type Part = [text: string, color: RGBA];
type Scan = { context: Context; ledger: Ledger };
type Extra = { parent: Renderable; key: string; style: Record<string, unknown>; content: string };

export const tone = colors.pink;
const marks = new Set<string>(Object.values(glyphs));
const separator = " · ";
const letters = new WeakMap<Renderable, Letter>();
const heads = new WeakMap<TextNodeRenderable, { key: string; node: TextNodeRenderable }>();
const extras = new WeakMap<Renderable, Map<string, TextRenderable>>();

function inline(row: Renderable) {
  const [box, ...more] = shown(row);
  const [line, ...extra] = box ? shown(box) : [];
  const [icon, text, ...rest] = line ? shown(line) : [];
  if (more.length > 0 || extra.length > 0 || rest.length > 0 || !textual(icon) || !textual(text)) return;
  return { icon, text };
}

export const candidate = (item: Item): item is Found => item.kind === "mail" || item.kind === "sent";

export function letter(row: Renderable): Found | undefined {
  const { icon, text } = inline(row) ?? {};
  if (!icon || !text) return;
  const [node, ...tail] = text.textNode.children;
  if (tail.length > 0 || !isTextNodeRenderable(node)) return;
  const full = whole(node);
  const mark = flatten(icon.textNode);
  const call = full.startsWith(`${source} [`);
  if (!marks.has(mark) && mark !== (call ? "✓" : "◈")) return;
  const fields = { tone, icon, text, full };
  const lane = Object.assign(letters.get(row) ?? fields, fields);
  letters.set(row, lane);
  return { kind: call ? "sent" : "mail", row, lane };
}

export function nested(row: Renderable): Found[] {
  const [anchor] = shown(row);
  const [group, list] = anchor ? shown(anchor) : [];
  const [line] = group ? shown(group) : [];
  const [icon] = line ? shown(line) : [];
  if (!list || !textual(icon) || flatten(icon.textNode) !== "−") return [];
  return shown(list).flatMap((entry) => {
    const found = letter(entry);
    return found?.kind === "sent" ? [found] : [];
  });
}

export function resolve(items: readonly Item[], session: Data, sessionID?: string) {
  const found = new Map<Letter, Mail>();
  if (sessionID === undefined || !items.some(candidate)) return found;
  const messages = session.message.list(sessionID);
  const parked = new Set(session.pending.list(sessionID).map((item) => item.id));
  const order = locate(items, messages);
  let end = messages.length;
  for (let at = items.length - 1; at >= 0; at--) {
    const item = items[at];
    if (item.kind !== "mail") continue;
    const low = order.findLast((value, index) => index < at && value !== undefined) ?? -1;
    const high = order.find((value, index) => index > at && value !== undefined) ?? messages.length;
    for (let index = Math.min(end, high) - 1; index > low; index--) {
      const message = messages[index];
      if (message.type !== "synthetic" || message.description !== item.lane.full) continue;
      end = index;
      const mail = parked.has(message.id) ? undefined : read(message.metadata);
      if (mail) found.set(item.lane, mail);
      break;
    }
  }
  pair(items, messages, found);
  return found;
}

function pair(items: readonly Item[], messages: Parameters<typeof sent>[0], found: Map<Letter, Mail>) {
  const outbox = sent(messages);
  let next = 0;
  for (const item of items) {
    if (item.kind !== "sent") continue;
    const [, messageID, partID] = /^session-part:([^:]+):(.+)$/.exec(item.row.id) ?? [];
    const at = outbox.findIndex(({ message, part, state }, index) => {
      if (summary(part.name, state.input) !== item.lane.full) return false;
      if (partID) return message.id === messageID && part.id === partID;
      return index >= next && (!item.row.id || message.id === item.row.id);
    });
    if (at === -1) continue;
    found.set(item.lane, outbox[at].mail);
    next = at + 1;
  }
}

export function compose(item: Found, mail: Mail, scan: Scan) {
  const { context, ledger } = scan;
  const { row, kind, lane } = item;
  const { icon, text, full } = lane;
  const line = text.parent;
  const box = line?.parent;
  const [mark] = icon.textNode.children;
  const [node] = text.textNode.children;
  if (!line || !box || !(text instanceof TextRenderable) || !isTextNodeRenderable(mark) || !isTextNodeRenderable(node))
    return;
  const theme = context.theme.text;
  const lead: Part[] = kind === "sent" ? [["→ ", theme.muted]] : [];
  const prefix: Part[] = [...lead, [kind === "sent" ? mail.recipient : mail.sender, tone], [separator, theme.muted]];
  const postmark = ` ${mail.kind}${separator}${when(mail.sent)}`;
  const used = Bun.stringWidth(prefix.map(([value]) => value).join("")) + Bun.stringWidth(postmark);
  const room = Math.trunc(line.width) - Math.trunc(icon.width) - used;
  const parts: Part[] = [...prefix, [fit(mail.subject, room), theme.base]];
  const label = parts.map(([value]) => value).join("");
  const edits = [
    ledger.write(row, mark, 0, glyphs[mail.kind]),
    ledger.paint(row, icon, tone),
    put(row, node, { key: "mail", full, value: head(node, parts), label }, ledger),
    ledger.adjust(row, "wrap", "none", { get: () => text.wrapMode, set: (value) => (text.wrapMode = value) }),
    attach(
      row,
      {
        parent: line,
        key: "detail",
        style: { fg: theme.muted, flexShrink: 0 },
        content: postmark,
      },
      scan,
    ),
    attach(
      row,
      { parent: box, key: "body", style: { wrapMode: "word", marginLeft: 2, fg: theme.base }, content: mail.body },
      scan,
    ),
  ];
  if (edits.includes(true)) text.onLifecyclePass?.();
  const detail = extras.get(line)?.get("detail");
  const body = extras.get(box)?.get("body");
  if (!detail || !body) return;
  return { mark: { node: icon, x: 0, glyph: glyphs[mail.kind] }, text, detail, body };
}

function head(node: TextNodeRenderable, parts: readonly Part[]) {
  const key = JSON.stringify(parts.map(([value, color]) => [value, color.toInts()]));
  const known = heads.get(node);
  if (known?.key === key) return known.node;
  const made = TextNodeRenderable.fromNodes(parts.map(([value, fg]) => TextNodeRenderable.fromString(value, { fg })));
  owned.add(made);
  heads.set(node, { key, node: made });
  return made;
}

function attach(row: Renderable, { parent, key, style, content }: Extra, { context, ledger }: Scan) {
  const slots = extras.get(parent) ?? new Map<string, TextRenderable>();
  extras.set(parent, slots);
  const known = slots.get(key);
  const text = known && !known.isDestroyed ? known : new TextRenderable(context.renderer, style);
  owned.add(text);
  slots.set(key, text);
  if (text.plainText !== content) text.content = content;
  if (text.parent === parent) return false;
  parent.add(text);
  ledger.record(row, text, () => text.destroy());
  return true;
}

function fit(value: string, room: number) {
  if (room <= 0 || Bun.stringWidth(value) <= room) return value;
  let out = "";
  for (const { segment } of new Intl.Segmenter().segment(value)) {
    if (Bun.stringWidth(out + segment) > room - 1) break;
    out += segment;
  }
  return `${out}…`;
}

function when(time: number) {
  const date = new Date(time);
  const clock = date.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  if (date.toDateString() === new Date().toDateString()) return clock;
  return `${date.toLocaleDateString([], { month: "short", day: "numeric" })} ${clock}`;
}

function shown(node: Renderable) {
  // Hidden slot placeholders are not extra content in the host row's shape.
  return native(node).filter((child) => !(child instanceof LayoutSlotRenderable));
}

function summary(tool: string, input: Record<string, unknown>) {
  const entries = Object.entries(input).filter(([, value]) => ["string", "number", "boolean"].includes(typeof value));
  if (entries.length === 0) return tool;
  return `${tool} ${`[${entries.map(([key, value]) => `${key}=${String(value)}`).join(", ")}]`.replace(/\s+/g, " ")}`;
}
