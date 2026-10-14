import { readFileSync } from "node:fs";
import path from "node:path";
import { RGBA, SyntaxStyle, type StyleDefinitionInput } from "@opentui/core";
import { root } from "./parsers.ts";

type Table = Record<string, StyleDefinitionInput>;

export type Change = "add" | "change" | "delete";

type File = {
  default: StyleDefinitionInput & { fg: string };
  base: Table;
  languages: Record<string, Table>;
  tint: Record<Change, { color: string; alpha: number }>;
};

export type Vagari = {
  fg: string;
  style(name: string): SyntaxStyle;
  tint(background: RGBA): Record<Change, RGBA>;
  destroy(): void;
};

export function mix(color: string, alpha: number, background: RGBA): RGBA {
  const [r, g, b] = RGBA.fromHex(color).toInts();
  const [br, bg, bb] = background.toInts();
  const channel = (value: number, under: number) => Math.floor(value * alpha + under * (1 - alpha) + 0.5);
  return RGBA.fromInts(channel(r, br), channel(g, bg), channel(b, bb));
}

export function load(): Vagari {
  const data: File = JSON.parse(readFileSync(path.join(root, "vagari.json"), "utf8"));
  const styles = new Map<string, SyntaxStyle>();
  return {
    fg: data.default.fg,
    style(name) {
      const key = Object.hasOwn(data.languages, name) ? name : "";
      let style = styles.get(key);
      if (!style) {
        style = SyntaxStyle.fromStyles({ default: data.default, ...data.base, ...data.languages[key] });
        styles.set(key, style);
      }
      return style;
    },
    tint(background) {
      const { add, change, delete: remove } = data.tint;
      return {
        add: mix(add.color, add.alpha, background),
        change: mix(change.color, change.alpha, background),
        delete: mix(remove.color, remove.alpha, background),
      };
    },
    destroy() {
      for (const style of styles.values()) style.destroy();
      styles.clear();
    },
  };
}
