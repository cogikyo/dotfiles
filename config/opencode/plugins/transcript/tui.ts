import { Plugin } from "@opencode/plugin/tui";
import { RGBA } from "@opentui/core";
import { existsSync, statSync } from "node:fs";
import { readdir } from "node:fs/promises";
import { delimiter } from "node:path";
import { scheme } from "../paths/commit.ts";
import { Compiler, isTree, load, source, type Tree } from "../vagari/build.ts";
import { glyph } from "./icons.ts";
import { kind, kinds, type Probe } from "./kind.ts";
import { patch } from "./markdown.ts";
import { paint, type Paint } from "./paint.ts";

type Palette = {
  readonly code: RGBA;
  readonly heading: RGBA;
  readonly link: RGBA;
  readonly linkText: RGBA;
  readonly body: RGBA;
  readonly session?: Paint;
  readonly web?: Paint;
  readonly icons: boolean;
  readonly kinds: Partial<Record<(typeof kinds)[number], Paint>>;
  readonly headings: ReadonlyMap<number, Paint>;
};

const FALLBACK: Partial<Record<(typeof kinds)[number], (typeof kinds)[number]>> = { danger: "command", key: "symbol" };

const isKind = (key: string): key is (typeof kinds)[number] => (kinds as readonly string[]).includes(key);

function settings(compiler: Compiler, block: Tree) {
  const palette: {
    kinds: Palette["kinds"];
    headings: Map<number, Paint>;
    session?: Paint;
    web?: Paint;
    icons: boolean;
  } = { kinds: {}, headings: new Map(), icons: false };
  for (const [key, value] of Object.entries(block)) {
    const where = `transcript.${key}`;
    const level = /^h([1-6])$/.exec(key)?.[1];
    if (level) {
      const found = paint(compiler, value, where);
      if (found) palette.headings.set(Number(level), found);
    } else if (key === "session") palette.session = paint(compiler, value, where);
    else if (key === "link") palette.web = paint(compiler, value, where);
    else if (key === "icons") {
      if (typeof value === "boolean") palette.icons = value;
      else compiler.fail(where, value, "is not true or false");
    } else if (key === "code" && isTree(value)) {
      for (const [name, hex] of Object.entries(value)) {
        if (isKind(name)) palette.kinds[name] = paint(compiler, hex, `${where}.${name}`);
        else compiler.fail(`${where}.${name}`, hex, `is not a span kind; use ${kinds.join(", ")}`);
      }
    } else compiler.fail(where, value, "is not a transcript setting; use code, session, link, icons, or h1 through h6");
  }
  return palette;
}

async function read(): Promise<Palette | undefined> {
  const doc = await load(source);
  if (doc.transcript === undefined) return undefined;
  const compiler = new Compiler(doc);
  const start = compiler.errors.length;
  const base = isTree(doc.base) ? doc.base : {};
  const anchor = (group: "markdown" | "text", key: string) => {
    const block = isTree(base[group]) ? base[group] : {};
    const hex = typeof block[key] === "string" ? compiler.defs.get(block[key]) : undefined;
    if (!hex) compiler.fail(`base.${group}.${key}`, block[key], "must be a vagari name for transcript colors");
    return hex && RGBA.fromHex(hex);
  };
  const code = anchor("markdown", "code");
  const heading = anchor("markdown", "heading");
  const link = anchor("markdown", "link");
  const linkText = anchor("markdown", "linkText");
  const body = anchor("text", "base");
  if (!isTree(doc.transcript)) compiler.fail("transcript", doc.transcript, "is not an object");
  const palette = settings(compiler, isTree(doc.transcript) ? doc.transcript : {});
  const errors = compiler.errors.slice(start);
  if (errors.length > 0 || !code || !heading || !link || !linkText || !body) throw new Error(errors.join("\n"));
  return { ...palette, code, heading, link, linkText, body };
}

async function commands() {
  const dirs = (process.env.PATH ?? "").split(delimiter).filter(Boolean);
  const names = await Promise.all(dirs.map((dir) => readdir(dir).catch(() => [])));
  return new Set(names.flat());
}

function cached(probe: (key: string) => boolean, limit = 1024) {
  const seen = new Map<string, boolean>();
  return (key: string) => {
    let found = seen.get(key);
    if (found === undefined) {
      found = probe(key);
      if (seen.size >= limit) seen.delete(seen.keys().next().value!);
      seen.set(key, found);
    }
    return found;
  };
}

export default Plugin.define({
  id: "transcript",
  setup(context) {
    const fail = (error: unknown) =>
      context.ui.toast.show({
        variant: "error",
        title: "Vagari theme",
        message: error instanceof Error ? error.message : String(error),
      });
    let probe: (Probe & { directory: (path: string) => boolean }) | undefined;
    let palette: Palette | undefined;
    let disposed = false;
    let queue = Promise.resolve();

    let patched: ReturnType<typeof patch>;
    try {
      // Each callback recolors only text the host drew in the theme's markdown anchor color, so other styling is left alone.
      patched = patch(context.renderer, {
        code(text, fg) {
          if (!palette || !probe || !fg?.equals(palette.code)) return undefined;
          const found = kind(text, probe);
          if (found === "hex") return { fg: RGBA.fromHex(text.trim()), bold: false };
          return palette.kinds[found] ?? palette.kinds[FALLBACK[found] ?? found];
        },
        heading: (level, fg) => (palette && fg?.equals(palette.heading) ? palette.headings.get(level) : undefined),
        session: (fg) => (palette && fg?.equals(palette.link) ? palette.session : undefined),
        link(url, fg) {
          if (!palette || !probe) return undefined;
          const { path, number } = palette.kinds;
          const drawn = [palette.body, palette.link, palette.linkText, path?.fg, number?.fg];
          if (!drawn.some((color) => fg?.equals(color))) return undefined;
          const chosen = url.startsWith("file:") ? path : url.startsWith(scheme) ? number : palette.web;
          return chosen && { paint: chosen, icon: palette.icons ? glyph(url, probe.directory) : undefined };
        },
      });
    } catch (error) {
      fail(error);
      throw error;
    }

    const reload = async () => {
      try {
        const [next, names] = await Promise.all([read(), commands()]);
        if (disposed) return;
        palette = next;
        probe = {
          which: (word) => names.has(word),
          exists: cached(existsSync),
          directory: cached((path) => statSync(path, { throwIfNoEntry: false })?.isDirectory() === true),
        };
        patched.refresh();
      } catch (error) {
        if (!disposed) fail(error);
      }
    };
    const signal = () => {
      queue = queue.then(reload);
    };
    signal();
    // The vagari plugin raises SIGUSR2 after it rebuilds the theme.
    process.on("SIGUSR2", signal);

    return () => {
      disposed = true;
      process.off("SIGUSR2", signal);
      return patched.restore();
    };
  },
});
