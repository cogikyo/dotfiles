import { parseArgs } from "node:util";
import { migrateV1, resolveThemeDocument } from "@opencode/theme/tui";
import type { RGBA } from "@opentui/core";
import { parseRouting, routingPath } from "../lanes/routing.ts";
import { card } from "./card.ts";
import { parse } from "./parse.ts";
import { palette, type Row } from "./style.ts";

const inset = 2;

const { values, positionals } = parseArgs({
  args: Bun.argv.slice(2),
  allowPositionals: true,
  options: { width: { type: "string", default: String(process.stdout.columns || 100) } },
});
const width = Number(values.width);
if (!Number.isInteger(width) || width <= 2 * inset) throw new Error(`--width needs an integer above ${2 * inset}`);
const [path] = positionals;
const source = path ? await Bun.file(path).text() : await Bun.stdin.text();
const document = await Bun.file(new URL("../../themes/vagari.json", import.meta.url)).json();
const theme = resolveThemeDocument(migrateV1(document), "dark");
const tones = palette(theme, "dark");
const ground = color(tones.card, 48);
const routes = await Bun.file(routingPath)
  .text()
  .then((text) => parseRouting(text).routes)
  .catch(() => undefined);

console.log(
  blocks(source)
    .map(({ text, partial }) => paint(card(parse(text, partial), width - 2 * inset, routes)))
    .join("\n\n"),
);

function blocks(input: string) {
  const lines = input.split("\n");
  const found: { text: string; partial: boolean }[] = [];
  for (let at = 0; at < lines.length; at++) {
    const fence = /^ {0,3}(`{3,}|~{3,})workflow\b/.exec(lines[at])?.[1];
    if (!fence) continue;
    const close = lines.findIndex(
      (line, index) => index > at && /^ {0,3}(`{3,}|~{3,})\s*$/.test(line) && line.trim().startsWith(fence),
    );
    const stop = close === -1 ? lines.length : close;
    found.push({ text: lines.slice(at + 1, stop).join("\n"), partial: close === -1 });
    at = stop;
  }
  return found.length ? found : [{ text: input, partial: false }];
}

function paint(rows: readonly Row[]) {
  const inner = width - 2 * inset;
  const body = rows.map((row) => {
    const painted = row.map(({ text, tone, bg, bold, italic }) => {
      const style = `${color(tones[tone], 38)}${bg ? color(tones[bg], 48) : ""}${bold ? "\x1b[1m" : ""}${italic ? "\x1b[3m" : ""}`;
      return `${style}${text}\x1b[22;23m${bg ? ground : ""}`;
    });
    return `${painted.join("")}${pad(inner - Bun.stringWidth(row.map((segment) => segment.text).join("")))}`;
  });
  return [pad(inner), ...body, pad(inner)]
    .map((line) => `${ground}${pad(inset)}${line}${pad(inset)}\x1b[0m`)
    .join("\n");
}

function pad(size: number) {
  return " ".repeat(Math.max(0, size));
}

function color(rgba: RGBA, layer: 38 | 48) {
  const [r, g, b] = rgba.toInts();
  return `\x1b[${layer};2;${r};${g};${b}m`;
}
