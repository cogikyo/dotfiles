import { readFile, rename, rm, writeFile } from "node:fs/promises";
import path from "node:path";

export const source = path.join(import.meta.dirname, "..", "..", "themes", "vagari.jsonc");

const STEPS = ["100", "200", "300", "400", "500", "600", "700", "800", "900"];
const MODES = ["dark", "light"];

type Tree = { readonly [key: string]: unknown };

const isTree = (value: unknown): value is Tree => typeof value === "object" && value !== null && !Array.isArray(value);

const family = (name: unknown) => String(name).replace(/_\d+$/, "");

function home(hues: Tree, name: string) {
  const holding = Object.entries(hues).flatMap(([hue, ramp]) => {
    const members = isTree(ramp) ? STEPS.map((step) => ramp[step]) : [];
    return members.includes(name) ? [{ hue, members }] : [];
  });
  const { hue, members } =
    holding.find((ramp) => ramp.members.every((member) => family(member) === family(name))) ?? holding[0] ?? {};
  if (!members) return undefined;
  const edge = members.findIndex(
    (member, i) =>
      member === name && [members[i - 1], members[i + 1]].some((next) => next !== undefined && next !== name),
  );
  return `$hue.${hue}.${STEPS[edge === -1 ? members.indexOf(name) : edge]}`;
}

class Compiler {
  readonly errors: string[] = [];
  readonly defs = new Map<string, string>();
  readonly hues = new Map<string, Tree>();

  constructor(doc: Tree) {
    for (const [name, hex] of Object.entries(isTree(doc.defs) ? doc.defs : {})) {
      if (typeof hex === "string" && /^#[0-9a-f]{6}$/i.test(hex)) this.defs.set(name, hex);
      else this.fail(`defs.${name}`, hex, "is not a hex color");
    }
    for (const mode of MODES) {
      const node = doc[mode];
      if (isTree(node)) this.hues.set(mode, isTree(node.hue) ? node.hue : {});
    }
  }

  fail(where: string, value: unknown, problem: string) {
    this.errors.push(`${where}: ${JSON.stringify(value)} ${problem}`);
  }

  hex(name: unknown, where: string) {
    const hex = typeof name === "string" ? this.defs.get(name) : undefined;
    if (!hex) this.fail(where, name, "is not a vagari color");
    return hex;
  }

  color(value: unknown, where: string, modes: readonly string[]) {
    if (value === "transparent" || (typeof value === "string" && value.startsWith("$"))) return value;
    const hex = this.hex(value, where);
    if (typeof value !== "string" || !hex) return value;
    const refs = new Set(modes.map((mode) => home(this.hues.get(mode) ?? {}, value)));
    const [ref] = refs;
    return refs.size === 1 && ref ? ref : hex;
  }

  hue(name: unknown, where: string, modes: readonly string[]) {
    const missing = modes.filter((mode) => typeof name !== "string" || this.hues.get(mode)?.[name] === undefined);
    if (missing.length > 0) this.fail(where, name, `is not a hue in ${missing.join(", ")}`);
  }

  tokens(node: Tree, where: string, modes: readonly string[]): Tree {
    return Object.fromEntries(
      Object.entries(node).map(([key, value]) => {
        const at = `${where}.${key}`;
        if (isTree(value)) return [key, this.tokens(value, at, modes)];
        if (key !== "categorical" || !Array.isArray(value)) return [key, this.color(value, at, modes)];
        value.forEach((hue, i) => this.hue(hue, `${at}[${i}]`, modes));
        return [key, value];
      }),
    );
  }

  mode(mode: string, node: Tree) {
    const { hue: _, ...rest } = node;
    const hue = Object.fromEntries(
      Object.entries(this.hues.get(mode) ?? {}).map(([key, ramp]) => {
        const where = `${mode}.hue.${key}`;
        if (isTree(ramp))
          return [key, Object.fromEntries(STEPS.map((step) => [step, this.hex(ramp[step], `${where}.${step}`)]))];
        if (typeof ramp === "string" && ramp.startsWith("$hue.")) this.hue(ramp.slice("$hue.".length), where, [mode]);
        else this.fail(where, ramp, "is not a ramp or $hue alias");
        return [key, ramp];
      }),
    );
    return { hue, ...this.tokens(rest, mode, [mode]) };
  }
}

function compile(doc: Tree) {
  const compiler = new Compiler(doc);
  const modes = [...compiler.hues.keys()];
  const theme: Record<string, unknown> = { $schema: "https://opencode.ai/theme.json" };
  for (const [key, value] of Object.entries(doc)) {
    if (key === "defs" || key === "$schema") continue;
    if (key === "base" && isTree(value)) theme[key] = compiler.tokens(value, key, modes);
    else if (modes.includes(key) && isTree(value)) theme[key] = compiler.mode(key, value);
    else theme[key] = value;
  }
  if (compiler.errors.length > 0) throw new Error(compiler.errors.join("\n"));
  return theme;
}

export async function build(from = source, to = from.replace(/\.jsonc$/, ".json")) {
  if (to === from) throw new Error(`${path.basename(from)}: the source must end in .jsonc`);
  let doc: unknown;
  try {
    doc = Bun.JSONC.parse(await readFile(from, "utf8"));
  } catch (error) {
    throw new Error(`${path.basename(from)}: ${error instanceof Error ? error.message : String(error)}`, {
      cause: error,
    });
  }
  if (!isTree(doc)) throw new Error(`${path.basename(from)}: expected an object`);
  const content = `${JSON.stringify(compile(doc), null, 2)}\n`;
  if ((await readFile(to, "utf8").catch(() => undefined)) === content) return { content, written: false };
  const temp = path.join(path.dirname(to), `.${path.basename(to)}.${process.pid}.${crypto.randomUUID()}.tmp`);
  try {
    await writeFile(temp, content);
    await rename(temp, to);
  } finally {
    await rm(temp, { force: true });
  }
  return { content, written: true };
}

if (import.meta.main) {
  const [from, to] = process.argv.slice(2);
  try {
    const { written } = await build(from, to);
    console.log(written ? "wrote theme" : "theme unchanged");
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exit(1);
  }
}
