import {
  fg,
  isTextNodeRenderable,
  StyledText,
  TextNodeRenderable,
  TextRenderable,
  Yoga,
  type RGBA,
  type Renderable,
} from "@opentui/core";
import type { Plugin } from "@opencode/plugin/tui";
import { configured } from "../compact/limits.ts";
import { glyphs, strain, usage } from "../lanes/label.ts";
import { roles } from "../statusline/palette.ts";
import { padding, type Item, type Lane, type Mark } from "./lane.ts";
import { owned, type Ledger } from "./ledger.ts";

type Context = Plugin.Context;
type Session = NonNullable<ReturnType<Context["data"]["session"]["get"]>>;
type Data = Context["data"]["session"];
type Source = Pick<Data, "status"> & { message: Pick<Data["message"], "list"> };
type Message = ReturnType<Data["message"]["list"]>[number];
type Scan = { context: Context; ledger: Ledger; error: RGBA };
type Shown = { key: string; full: string; value: string | TextNodeRenderable; label: string };
type Part = [text: string, color: RGBA];
type Side = { parts: Part[]; icon?: { x: number; glyph: string } };

export type Reply = { id: string; created: number; live: boolean; child?: string };

const separator = " · ";
const lanes = new WeakMap<Renderable, Lane>();
const pins = new WeakMap<Renderable, { text: TextRenderable; key: string }>();
const leads = new WeakMap<TextNodeRenderable, { key: string; node: TextNodeRenderable }>();
const shown = new WeakMap<TextNodeRenderable, { full: string; label: string }>();
const buffers = new Map<string, number | undefined>();
const asked = new Set<string>();
const segments = new Intl.Segmenter();

export function notice(row: Renderable, text: Lane["text"]): Item | undefined {
  const [heading, suffix, ...rest] = text.textNode.children;
  if (rest.length > 0 || !isTextNodeRenderable(heading) || !isTextNodeRenderable(suffix)) return;
  const title = whole(heading);
  const summary = whole(suffix);
  if (!/^( ·|…| …|$)/.test(summary)) return;
  const [, glyph, actor, state] = /^([↳!]) (\S+) (\S+)$/u.exec(title) ?? [];
  if (!glyph || !actor || !state) return;
  const role = actor.split("/")[0].toLowerCase();
  const tone = actor.includes("/") ? roles[role] : undefined;
  if (!tone) return { kind: "plain", row };
  const fields = { text, tone, role, actor, state, title, summary };
  const lane = Object.assign(lanes.get(row) ?? fields, fields);
  lanes.set(row, lane);
  return { kind: glyph === "!" ? "failed" : "notice", row, lane };
}

export function match(items: readonly Item[], closed: ReadonlySet<Lane>, session: Source, sessionID?: string) {
  const found = new Map<Lane, Reply>();
  const notes = items.flatMap((item, at) => ("lane" in item ? [{ lane: item.lane, at }] : []));
  if (sessionID === undefined || notes.length === 0) return found;
  const running = session.status(sessionID) === "running";
  const messages = session.message.list(sessionID);
  const idle = messages.findLastIndex((message) => message.type === "idle");
  const positions = new Map(messages.map((message, index) => [message.id, index]));
  const order = items.map(({ row }) => positions.get(/^session-part:([^:]+):/.exec(row.id)?.[1] ?? row.id));
  let end = messages.length;
  for (const { lane, at } of notes.toReversed()) {
    const low = order.findLast((value, index) => index < at && value !== undefined) ?? -1;
    const high = order.find((value, index) => index > at && value !== undefined) ?? messages.length;
    const index = before(messages, Math.min(end, high), low, lane);
    if (index === -1) continue;
    end = index;
    const message = messages[index];
    const child = message.type === "synthetic" ? message.metadata?.childID : undefined;
    found.set(lane, {
      id: message.id,
      created: message.time.created,
      live: running && index > idle && !closed.has(lane),
      child: typeof child === "string" ? child : undefined,
    });
  }
  return found;
}

export function compose(lane: Lane, child: string | undefined, scan: Scan): Mark | undefined {
  const { text, tone, title, summary } = lane;
  const box = text.parent;
  const [heading, suffix, ...rest] = text.textNode.children;
  if (!box || rest.length > 0 || !isTextNodeRenderable(heading) || !isTextNodeRenderable(suffix)) return;
  const { parts, icon } = side(lane, child, scan.context);
  const node = pin(box, parts, scan);
  const room = width(parts) + 1;
  scan.ledger.adjust(box, "room", room, {
    get: () => padding(box, Yoga.Edge.Right),
    set: (value) => (box.paddingRight = value),
  });
  const [value, label] = lead(lane, heading, scan);
  const description = summary.startsWith(separator) ? summary.slice(separator.length) : summary;
  const headline = clip(description, Math.trunc(box.width) - room - Bun.stringWidth(label));
  const edits = [
    put(text, heading, { key: "lead", full: title, value, label }, scan.ledger),
    put(text, suffix, { key: "headline", full: summary, value: headline, label: headline }, scan.ledger),
    scan.ledger.paint(text, suffix, tone),
  ];
  if (edits.includes(true)) text.onLifecyclePass?.();
  return icon && { node, x: icon.x, glyph: icon.glyph };
}

export function flatten(node: TextNodeRenderable): string {
  return node.children.map((child) => (typeof child === "string" ? child : flatten(child))).join("");
}

function whole(node: TextNodeRenderable) {
  const value = flatten(node);
  const known = shown.get(node);
  return known?.label === value ? known.full : value;
}

function before(messages: readonly Message[], end: number, low: number, lane: Lane) {
  for (let index = end - 1; index > low; index--) if (answers(messages[index], lane)) return index;
  return -1;
}

function answers(message: Message, { actor, state, summary }: Lane) {
  if (message.type !== "synthetic") return false;
  const { source, state: outcome, agent } = message.metadata ?? {};
  if (source !== "subagent" || typeof outcome !== "string" || typeof agent !== "string") return false;
  if (agent.toLowerCase() !== actor.toLowerCase() || status(outcome) !== state) return false;
  const full = `${separator}${message.description ?? ""}`;
  return summary === full || (summary.endsWith("…") && full.startsWith(summary.slice(0, -1)));
}

function status(outcome: string) {
  if (outcome === "completed") return "finished";
  return outcome === "error" ? "failed" : outcome;
}

function side(lane: Lane, child: string | undefined, context: Context): Side {
  const muted = context.theme.text.muted;
  const [role, ...rest] = lane.actor.split("/");
  const marks = glyphs(lane.actor.toLowerCase());
  const parts: Part[] = [];
  let icon: Side["icon"];
  for (const [name, glyph] of [
    [role, marks.role],
    [rest.join("/"), marks.scope],
  ] as const) {
    if (!name) continue;
    if (parts.length > 0) parts.push([" / ", muted]);
    if (glyph) {
      icon = { x: width(parts), glyph };
      parts.push([glyph, lane.tone], [" ", muted]);
    }
    parts.push([name, muted]);
  }
  const session = child === undefined ? undefined : context.data.session.get(child);
  if (session) parts.push(...stats(context, session));
  return { parts, icon };
}

function stats(context: Context, session: Session): Part[] {
  const directory = session.location.directory;
  if (!asked.has(session.id)) {
    asked.add(session.id);
    void context.data.session.message.sync(session.id);
    void configured(context, directory).then((buffer) => buffers.set(directory, buffer));
  }
  const muted = context.theme.text.muted;
  const entry = usage(context, session);
  const variant = entry.variant && entry.variant !== "default" ? `/${entry.variant}` : "";
  const parts: Part[] = [
    [separator, muted],
    [`${entry.model}${variant}`, muted],
  ];
  if (!entry.spent) return parts;
  const spent = (buffers.has(directory) ? strain(entry, buffers.get(directory)) : undefined) ?? muted;
  return [...parts, [separator, muted], [entry.spent, spent]];
}

function pin(box: Renderable, parts: readonly Part[], { context, ledger }: Scan) {
  const key = JSON.stringify(parts.map(([value, color]) => [value, color.toInts()]));
  const known = pins.get(box);
  const text =
    known && !known.text.isDestroyed
      ? known.text
      : new TextRenderable(context.renderer, {
          position: "absolute",
          top: 0,
          right: 0,
          wrapMode: "none",
          selectable: false,
        });
  owned.add(text);
  if (known?.key !== key || known.text !== text) {
    text.content = new StyledText(parts.map(([value, color]) => fg(color)(value)));
    pins.set(box, { text, key });
  }
  ledger.mount(box, text);
  return text;
}

function lead({ state }: Lane, heading: TextNodeRenderable, { context, error }: Scan) {
  if (state === "finished") return ["", ""] as const;
  const word = `${state.charAt(0).toUpperCase()}${state.slice(1)}`;
  const label = `✗ ${word}${separator}`;
  const muted = context.theme.text.muted;
  const key = JSON.stringify([label, error.toInts(), muted.toInts()]);
  const known = leads.get(heading);
  if (known?.key === key) return [known.node, label] as const;
  const node = TextNodeRenderable.fromNodes([
    TextNodeRenderable.fromString("✗", { fg: error }),
    TextNodeRenderable.fromString(` ${word}`),
    TextNodeRenderable.fromString(separator, { fg: muted }),
  ]);
  owned.add(node);
  leads.set(heading, { key, node });
  return [node, label] as const;
}

function put(owner: Renderable, node: TextNodeRenderable, { key, full, value, label }: Shown, ledger: Ledger) {
  const [current, ...more] = node.children;
  if (current === undefined || more.length > 0) return false;
  shown.set(node, { full, label });
  if (current === value) return false;
  node.replace(value, 0);
  ledger.record(owner, key, () => {
    if (node.children[0] === value) node.replace(full, 0);
  });
  return true;
}

function width(parts: readonly Part[]) {
  return Bun.stringWidth(parts.map(([value]) => value).join(""));
}

export function clip(value: string, space: number) {
  if (Bun.stringWidth(value) <= space) return value;
  let kept = "";
  for (const { segment } of segments.segment(value)) {
    if (Bun.stringWidth(kept + segment) > space - 1) break;
    kept += segment;
  }
  return `${kept.trimEnd()}…`;
}
