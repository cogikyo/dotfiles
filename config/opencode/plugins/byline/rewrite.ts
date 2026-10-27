import {
  isTextNodeRenderable,
  RGBA,
  TextAttributes,
  TextNodeRenderable,
  TextRenderable,
  Yoga,
  type CliRenderer,
  type MarkdownRenderable,
  type Renderable,
} from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import type { Mail } from "../mail/mail.ts";
import type { Post, Stage } from "./draw.ts";
import { apply, dashes, grow, runs, walk, type Item, type Letter, type Tint } from "./lane.ts";
import { compose, flatten, match, notice } from "./notice.ts";
import { question } from "./question.ts";
import { native, owned, textual, type Ledger, type Text } from "./ledger.ts";
import * as mail from "./mail.ts";

type Frame = Renderable & { readonly border: unknown };
type Scroll = Renderable & { readonly content: Renderable; readonly viewport: Renderable };
type Names = () => ReadonlySet<string>;
type Host = { context: Plugin.Context; sessionID?: string; stage: Stage };
type Scan = { names: Names; ledger: Ledger; renderer: CliRenderer; width: number; error: RGBA };

const separator = " · ";

const tones = {
  rule: RGBA.fromHex("#3f4578"),
  model: RGBA.fromHex("#5a61aa"),
  stats: RGBA.fromHex("#585f8d"),
};

const italic = { attributes: TextAttributes.ITALIC };

const rules = new WeakMap<Renderable, TextRenderable[]>();
const failed = new WeakSet<Ledger>();

export function prompt(anchor: Renderable, names: Names, ledger: Ledger, fallback = "") {
  ledger.prune();
  const row = metadata(anchor) ?? [];
  const group = row.at(-1);
  if (!group || textual(group)) return;
  const nodes = group.getChildren();
  const [dot] = nodes;
  const [agent] = row;
  const labelled = textual(dot) && read(dot) === "·" && textual(agent);
  const model = labelled ? nodes[1] : dot;
  if (!names().has(labelled ? read(agent) : fallback)) {
    for (const node of [...row, ...nodes]) ledger.restore(node);
    return;
  }
  if (!textual(model)) return;
  const edits = [...emphasize(model, model.textNode, ledger), labelled && relabel(row, dot, model, ledger)];
  if (edits.includes(true)) model.onLifecyclePass?.();
}

function relabel(row: Renderable[], dot: Renderable, model: Text, ledger: Ledger) {
  const [agent] = row;
  if (!textual(agent)) return false;
  ledger.hide(agent);
  if (row.some((node) => node !== agent && node.visible && textual(node) && read(node) === "auto")) ledger.restore(dot);
  else ledger.hide(dot);
  return ledger.paint(model, model, agent.fg);
}

function metadata(anchor: Renderable) {
  const frame = anchor.parent?.parent?.getChildren()[0];
  if (!frame || !framed(frame)) return;
  const inner = frame.getChildren()[0]?.getChildren() ?? [];
  const editor = inner.findIndex(editable);
  if (editor === -1) return;
  return inner[editor + 1]?.getChildren()[0]?.getChildren();
}

export function transcript(anchor: Renderable, names: Names, ledger: Ledger, { context, sessionID, stage }: Host) {
  if (stage.broken) halt(ledger, stage);
  const scroll = scroller(anchor.parent?.parent, 2);
  if (!scroll || failed.has(ledger)) {
    stage.clear();
    return false;
  }
  ledger.prune();
  const renderer = context.renderer;
  const theme = context.theme;
  if (renderer.width < 28) {
    ledger.clear();
    stage.clear();
    return false;
  }
  const scan = {
    names,
    ledger,
    renderer,
    width: scroll.content.width,
    error: theme.text.feedback.error.base,
  };
  const rows = scroll.content.getChildren();
  const session = context.data.session;
  const stops = (row: Renderable) => {
    if (sessionID === undefined) return;
    if (session.message.get(sessionID, row.id)?.type === "user") return "prompt";
    return question(row, native(row), { context, sessionID, ledger }) ? "stop" : undefined;
  };
  try {
    const inner = new Set<Renderable>();
    const scanned = rows.flatMap((row, index) => {
      const item = classify(row, scan, stops(row), index === rows.length - 1);
      if (item.kind !== "content") return [item];
      const entries = mail.nested(row);
      for (const entry of entries) inner.add(entry.row);
      return [item as Item].concat(entries);
    });
    const posts = mail.resolve(scanned, session, sessionID);
    const items = scanned.flatMap((item): Item[] => {
      if (!mail.candidate(item) || posts.has(item.lane)) return [item];
      if (!inner.has(item.row)) return [{ kind: "content", row: item.row }];
      ledger.restore(item.row);
      return [];
    });
    const { units, closed, open } = walk(items, mail.tone);
    const found = match(items, closed, session, sessionID);
    grow(open, (lane) => found.get(lane)?.live === true);
    apply(units, ledger);
    const letters = arrivals(items, posts, { context, ledger, closed, sessionID });
    const lanes = items.flatMap((item) => {
      if (item.kind !== "notice" && item.kind !== "failed") return [];
      const reply = found.get(item.lane);
      const mark = compose(item.lane, reply?.child, { context, ledger, error: scan.error });
      return item.kind === "notice" ? [{ lane: item.lane, reply, mark }] : [];
    });
    return stage.update({
      content: scroll.content,
      viewport: scroll.viewport,
      base: theme.background.base,
      runs: runs(units),
      lanes,
      posts: letters,
    });
  } catch {
    halt(ledger, stage);
    return false;
  }
}

function arrivals(
  items: readonly Item[],
  posts: ReadonlyMap<Letter, Mail>,
  {
    context,
    ledger,
    closed,
    sessionID,
  }: { context: Plugin.Context; ledger: Ledger; closed: ReadonlySet<Tint>; sessionID?: string },
): Post[] {
  const session = context.data.session;
  const messages = sessionID === undefined ? [] : session.message.list(sessionID);
  const running = sessionID !== undefined && session.status(sessionID) === "running";
  const idle = messages.findLastIndex((message) => message.type === "idle");
  return items.flatMap((item): Post[] => {
    if (!mail.candidate(item)) return [];
    const post = posts.get(item.lane);
    const shape = post && mail.compose(item, post, { context, ledger });
    if (!post || !shape) return [];
    const { kind, lane } = item;
    const tone = lane.tone;
    if (kind === "sent")
      return [{ ...shape, kind, tone, reply: { id: `sent:${post.id}`, created: post.sent, live: false } }];
    const index = messages.findIndex((message) => message.id === post.id);
    const message = messages[index];
    if (!message) return [];
    const live = running && index > idle && !closed.has(lane);
    // Use delivery time so long-parked mail can still reveal on arrival.
    return [{ ...shape, kind, tone, reply: { id: post.id, created: message.time.created, live } }];
  });
}

function halt(ledger: Ledger, stage: Stage) {
  if (failed.has(ledger)) return;
  failed.add(ledger);
  ledger.clear();
  stage.clear();
}

function classify(row: Renderable, scan: Scan, stop: "prompt" | "stop" | undefined, last: boolean): Item {
  const box = native(row).at(-1);
  if (stop) return { kind: stop, row };
  const letter = mail.letter(row);
  if (letter) return letter;
  const [only, ...extra] = box ? native(box) : [];
  if (!box || extra.length > 0) return { kind: "content", row };
  if (textual(only)) {
    const found = notice(row, only);
    if (found) return found;
    if (byline(only, scan.names, scan.ledger)) {
      rule(box, scan.width - Bun.stringWidth(read(only)), scan.ledger, scan.renderer);
      return { kind: "stop", row };
    }
    if (turned(only)) return { kind: "stop", row };
  }
  if (!marked(only)) return { kind: "content", row };
  const blocks = only._blockStates.flatMap((state) =>
    state.renderable
      ? [
          {
            node: state.renderable,
            hr: state.token.type === "hr",
            tail: (state.token.type === "paragraph" || state.token.type === "heading") && dashes.test(state.token.raw),
          },
        ]
      : [],
  );
  return { kind: "markdown", row, blocks, streaming: last && only.streaming };
}

function byline(text: Text, names: Names, ledger: Ledger) {
  const [agent, model, ...stats] = text.textNode.children;
  if (!isTextNodeRenderable(agent) || !isTextNodeRenderable(model)) return false;
  const label = flatten(agent);
  if (label ? !claimed(label, flatten(model), names) : !ledger.has(text)) return false;
  const edits = [
    ...leaves(agent).map(([node, index]) => ledger.write(text, node, index, "")),
    strip(text, model, ledger),
    ledger.paint(text, model, tones.model),
    ...emphasize(text, model, ledger),
    ...stats.filter(isTextNodeRenderable).map((node) => ledger.paint(text, node, tones.stats)),
  ];
  if (edits.includes(true)) text.onLifecyclePass?.();
  return true;
}

function rule(box: Renderable, room: number, ledger: Ledger, renderer: CliRenderer) {
  if (room < 4) return ledger.restore(box);
  ledger.adjust(box, "direction", "row", {
    get: () => box.primaryAxis,
    set: (value) => (box.flexDirection = value),
  });
  ledger.adjust(box, "padding", 0, {
    get: () => box.getPadding(Yoga.Edge.Left).value,
    set: (value) => (box.paddingLeft = value),
  });
  const [left, right] = sides(box, renderer);
  ledger.mount(box, left, 0);
  ledger.mount(box, right);
}

function sides(box: Renderable, renderer: CliRenderer) {
  const known = rules.get(box);
  const pair =
    known && !known.some((side) => side.isDestroyed)
      ? known
      : [{ marginRight: 1 }, { marginLeft: 1 }].map(
          (margin) =>
            new TextRenderable(renderer, {
              ...margin,
              fg: tones.rule,
              wrapMode: "none",
              flexGrow: 1,
              flexBasis: 0,
              selectable: false,
            }),
        );
  rules.set(box, pair);
  for (const side of pair) owned.add(side);
  for (const side of pair) if (side.plainText.length !== renderer.width) side.content = "─".repeat(renderer.width);
  return pair;
}

function turned(text: Text) {
  const [agent, model] = text.textNode.children;
  if (!isTextNodeRenderable(agent) || !isTextNodeRenderable(model)) return false;
  return !/^[↳!] /.test(flatten(agent)) && flatten(model).startsWith(separator);
}

function emphasize(owner: Renderable, node: TextNodeRenderable, ledger: Ledger) {
  return leaves(node).map(([parent, index, value]) => {
    const parts = value.split(/(\bfast\b)/i);
    if (parts.length === 1) return false;
    const wrapper = TextNodeRenderable.fromNodes(
      parts.flatMap((part, at) => (part ? [TextNodeRenderable.fromString(part, at % 2 ? italic : undefined)] : [])),
    );
    owned.add(wrapper);
    return ledger.write(owner, parent, index, wrapper);
  });
}

function claimed(label: string, rest: string, names: Names) {
  return names().has(label) && rest.startsWith(separator);
}

function strip(text: Text, model: TextNodeRenderable, ledger: Ledger) {
  const lead = leaves(model).find(([, , value]) => value);
  if (!lead?.[2].startsWith(separator)) return false;
  return ledger.write(text, lead[0], lead[1], lead[2].slice(separator.length));
}

function leaves(node: TextNodeRenderable): [TextNodeRenderable, number, string][] {
  return node.children.flatMap((child, index): [TextNodeRenderable, number, string][] =>
    typeof child === "string" ? [[node, index, child]] : owned.has(child) ? [] : leaves(child),
  );
}

function read(node: Text) {
  return flatten(node.textNode);
}

function marked(node: Renderable | undefined): node is MarkdownRenderable {
  return node !== undefined && "_blockStates" in node;
}

function scroller(node: Renderable | null | undefined, depth: number): Scroll | undefined {
  for (const child of node?.getChildren() ?? []) {
    const found = scrollable(child) ? child : depth > 1 ? scroller(child, depth - 1) : undefined;
    if (found) return found;
  }
}

function scrollable(node: Renderable | undefined): node is Scroll {
  return node !== undefined && "content" in node && "viewport" in node;
}

function framed(node: Renderable): node is Frame {
  return "borderColor" in node && "border" in node && Array.isArray(node.border) && node.border.join() === "left";
}

function editable(node: Renderable) {
  return "plainText" in node && "cursorOffset" in node && "extmarks" in node;
}
