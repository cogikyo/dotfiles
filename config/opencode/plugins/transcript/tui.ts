import { Plugin } from "@opencode/plugin/tui";
import { RGBA } from "@opentui/core";
import { existsSync } from "node:fs";
import { readdir } from "node:fs/promises";
import { delimiter } from "node:path";
import { Compiler, isTree, load, source, type Tree } from "../vagari/build.ts";
import { kind, kinds, type Probe } from "./kind.ts";
import { patch } from "./markdown.ts";

type Palette = {
  readonly code: RGBA;
  readonly heading: RGBA;
  readonly link: RGBA;
  readonly session?: RGBA;
  readonly kinds: Partial<Record<(typeof kinds)[number], RGBA>>;
  readonly headings: ReadonlyMap<number, RGBA>;
};

const isKind = (key: string): key is (typeof kinds)[number] => (kinds as readonly string[]).includes(key);

function settings(compiler: Compiler, block: Tree) {
  const color = (name: unknown, where: string) => {
    const hex = compiler.hex(name, where);
    return hex ? RGBA.fromHex(hex) : undefined;
  };
  const palette: { kinds: Palette["kinds"]; headings: Map<number, RGBA>; session?: RGBA } = {
    kinds: {},
    headings: new Map(),
  };
  for (const [key, value] of Object.entries(block)) {
    const where = `transcript.${key}`;
    const level = /^h([1-6])$/.exec(key)?.[1];
    if (level) {
      const fg = color(value, where);
      if (fg) palette.headings.set(Number(level), fg);
    } else if (key === "session") palette.session = color(value, where);
    else if (key === "code" && isTree(value)) {
      for (const [name, hex] of Object.entries(value)) {
        if (isKind(name)) palette.kinds[name] = color(hex, `${where}.${name}`);
        else compiler.fail(`${where}.${name}`, hex, `is not a span kind; use ${kinds.join(", ")}`);
      }
    } else compiler.fail(where, value, "is not a transcript setting; use code, session, or h1 through h6");
  }
  return palette;
}

async function read(): Promise<Palette | undefined> {
  const doc = await load(source);
  if (doc.transcript === undefined) return undefined;
  const compiler = new Compiler(doc);
  const start = compiler.errors.length;
  const markdown = isTree(doc.base) && isTree(doc.base.markdown) ? doc.base.markdown : {};
  const anchor = (key: string) => {
    const hex = typeof markdown[key] === "string" ? compiler.defs.get(markdown[key]) : undefined;
    if (!hex) compiler.fail(`base.markdown.${key}`, markdown[key], "must be a vagari name for transcript colors");
    return hex && RGBA.fromHex(hex);
  };
  const code = anchor("code");
  const heading = anchor("heading");
  const link = anchor("link");
  if (!isTree(doc.transcript)) compiler.fail("transcript", doc.transcript, "is not an object");
  const palette = settings(compiler, isTree(doc.transcript) ? doc.transcript : {});
  const errors = compiler.errors.slice(start);
  if (errors.length > 0 || !code || !heading || !link) throw new Error(errors.join("\n"));
  return { ...palette, code, heading, link };
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
    let probe: Probe | undefined;
    let palette: Palette | undefined;
    let disposed = false;
    let queue = Promise.resolve();

    let patched: ReturnType<typeof patch>;
    try {
      patched = patch(context.renderer, {
        code(text, fg) {
          if (!palette || !probe || !fg?.equals(palette.code)) return undefined;
          const found = kind(text, probe);
          if (found === "hex") return RGBA.fromHex(text.trim());
          return palette.kinds[found] ?? (found === "danger" ? palette.kinds.command : undefined);
        },
        heading: (level, fg) => (palette && fg?.equals(palette.heading) ? palette.headings.get(level) : undefined),
        session: (fg) => (palette && fg?.equals(palette.link) ? palette.session : undefined),
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
        probe = { which: (word) => names.has(word), exists: cached(existsSync) };
        patched.refresh();
      } catch (error) {
        if (!disposed) fail(error);
      }
    };
    const signal = () => {
      queue = queue.then(reload);
    };
    signal();
    process.on("SIGUSR2", signal);

    return () => {
      disposed = true;
      process.off("SIGUSR2", signal);
      return patched.restore();
    };
  },
});
