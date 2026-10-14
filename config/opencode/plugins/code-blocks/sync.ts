import { copyFileSync, cpSync, existsSync, mkdirSync, readFileSync, renameSync, rmSync, writeFileSync } from "node:fs";
import { homedir } from "node:os";
import path from "node:path";
import { RGBA, rgbToHex } from "@opentui/core";
import { Language, Parser, Query } from "web-tree-sitter";
import { scan, type Manifest } from "./parsers.ts";
import { convert, type Report } from "./query.ts";
import { mix } from "./vagari.ts";

type Style = { fg?: string; bold?: true; italic?: true; underline?: true };
type Source = { wasm: string; query: string };
type Entry = { filetype: string; wasm: string; aliases: string[]; queries: Source[] };
type Host = { filetype: string; wasm: string; aliases?: string[]; queries: { highlights?: string[] } };
type Resolved = {
  default: Style;
  base: Record<string, Style>;
  languages: Record<string, Record<string, Style>>;
  tint: Record<string, { color?: string; alpha?: number; resolved: string; background: string }>;
};

const home = homedir();
const cacheHome = process.env.XDG_CACHE_HOME ?? path.join(home, ".cache");
const config = path.resolve(import.meta.dirname, "..", "..");
const out = path.join(config, "tree-sitter");
const staging = path.join(config, ".tree-sitter.sync");
const assets = path.join(config, "node_modules", "@opentui", "core", "assets");
const nvimQueries = path.join(home, ".local", "share", "nvim", "lazy", "nvim-treesitter", "runtime", "queries");
const vagari = path.join(home, "vagari", "nvim");
const cache = path.join(cacheHome, "opencode", "code-blocks");

const kept = new Set(["diff", "markdown"]);
const nvimNames = new Map([["csharp", "c_sharp"]]);
const fences = new Map([
  ["bash", ["shell"]],
  ["csharp", ["c#"]],
  ["go", ["golang"]],
  ["hcl", ["tf", "terraform"]],
  ["javascript", ["javascriptreact"]],
  ["json", ["jsonc"]],
  ["python", ["python3"]],
  ["typescript", ["typescriptreact"]],
]);

const renames = new Map([
  ["conditional", "keyword.conditional"],
  ["conditional.ternary", "keyword.conditional.ternary"],
  ["debug", "keyword.debug"],
  ["define", "keyword.directive.define"],
  ["exception", "keyword.exception"],
  ["field", "variable.member"],
  ["float", "number.float"],
  ["include", "keyword.import"],
  ["method", "function.method"],
  ["method.call", "function.method.call"],
  ["namespace", "module"],
  ["namespace.builtin", "module.builtin"],
  ["parameter", "variable.parameter"],
  ["parameter.builtin", "variable.parameter.builtin"],
  ["preproc", "keyword.directive"],
  ["repeat", "keyword.repeat"],
  ["storageclass", "keyword.modifier"],
  ["string.regex", "string.regexp"],
  ["symbol", "string.special.symbol"],
  ["type.qualifier", "keyword.modifier"],
]);

const resolver = `
local input = vim.json.decode(io.open(vim.env.SYNC_IN):read("a"))
local function hex(value) return string.format("#%06x", value) end
local function attrs(name)
  local hl = vim.api.nvim_get_hl(0, { id = vim.api.nvim_get_hl_id_by_name(name), link = false })
  local style = {}
  if hl.fg then style.fg = hex(hl.fg) end
  for _, key in ipairs({ "bold", "italic", "underline" }) do
    if hl[key] then style[key] = true end
  end
  return style
end
local palette = require("vagari.palette")
local output = { default = { fg = hex(vim.api.nvim_get_hl(0, { name = "Normal" }).fg) }, base = {}, languages = {}, tint = {} }
for _, name in ipairs(input.names) do output.base[name] = attrs("@" .. name) end
for _, lang in ipairs(input.langs) do
  output.languages[lang] = {}
  for _, name in ipairs(input.names) do output.languages[lang][name] = attrs("@" .. name .. "." .. lang) end
end
local thalamus = io.open(vim.env.SYNC_THALAMUS):read("a")
for key, group in pairs({ add = "GitSignsAddLn", change = "GitSignsChangeLn", delete = "GitSignsDeleteLn" }) do
  local field = ({ add = "add_ln", change = "dirty_ln", delete = "delete_ln" })[key]
  local color, alpha = thalamus:match(field .. "%s*=%s*{%s*bg%s*=%s*blend%(p%.([%w_]+),%s*([%d.]+)%)")
  output.tint[key] = {
    color = color and palette[color],
    alpha = tonumber(alpha),
    resolved = hex(vim.api.nvim_get_hl(0, { name = group, link = false }).bg),
    background = palette.bg,
  }
end
io.open(vim.env.SYNC_OUT, "w"):write(vim.json.encode(output))
`;

async function hostEntries(): Promise<Entry[]> {
  const version = Bun.spawnSync(["opencode", "--version"]).stdout.toString().trim().split(/\s+/).at(-1) ?? "";
  const repo = `github.com/anomalyco/opencode@${version}`;
  const file = path.join(cacheHome, "src", repo, "packages", "tui", "src", "parsers-config.ts");
  if (!existsSync(file)) throw new Error(`${file} is missing; run: src get ${repo}`);
  const { default: host }: { default: { parsers: Host[] } } = await import(file);
  return host.parsers.map((parser) => ({
    filetype: parser.filetype,
    wasm: parser.wasm,
    aliases: parser.aliases ?? [],
    queries: (parser.queries.highlights ?? []).map((query) => ({ wasm: parser.wasm, query })),
  }));
}

function assetWasm(folder: string): string {
  return path.join(assets, folder, `tree-sitter-${folder}.wasm`);
}

function asset(name: string, extra: string[] = []): Entry {
  return {
    filetype: name,
    wasm: assetWasm(name),
    aliases: [],
    queries: [name, ...extra].map((folder) => ({
      wasm: assetWasm(folder),
      query: path.join(assets, folder, "highlights.scm"),
    })),
  };
}

async function fetchCached(url: string): Promise<string> {
  const file = path.join(cache, `${Bun.hash(url).toString(16)}-${path.basename(new URL(url).pathname)}`);
  if (existsSync(file)) return file;
  const response = await fetch(url);
  if (!response.ok) throw new Error(`${url}: HTTP ${response.status}`);
  mkdirSync(cache, { recursive: true });
  writeFileSync(file, new Uint8Array(await response.arrayBuffer()));
  return file;
}

function grammarMismatch(error: unknown): error is Error {
  return (
    error instanceof Error &&
    error.name === "QueryError" &&
    /^Bad (node name|field name|pattern structure)/.test(error.message)
  );
}

function nvimSource(lang: string, seen = new Set<string>()): string {
  if (seen.has(lang)) return "";
  seen.add(lang);
  const text = readFileSync(path.join(nvimQueries, lang, "highlights.scm"), "utf8");
  const header = /^;+\s*inherits\s*:?\s*(.+)$/m.exec(text.split(/\n(?!;)/, 1)[0]);
  const parents = header?.[1].split(",").map((name) => name.trim().replace(/^\((.*)\)$/, "$1")) ?? [];
  return [...parents.map((parent) => nvimSource(parent, seen)), text].join("\n");
}

function vagariSource(lang: string, base: string): string {
  const file = path.join(vagari, "queries", lang, "highlights.scm");
  if (!existsSync(file)) return base;
  const text = readFileSync(file, "utf8");
  return /^;+\s*extends/m.test(text) ? `${base}\n${text}` : text;
}

function styled(name: string): boolean {
  return !name.startsWith("_") && name !== "spell" && name !== "nospell";
}

function normalize(style: Style = {}): Style {
  return {
    ...(style.fg ? { fg: style.fg } : {}),
    ...(style.bold ? { bold: true } : {}),
    ...(style.italic ? { italic: true } : {}),
    ...(style.underline ? { underline: true } : {}),
  };
}

function empty(style?: Style): boolean {
  return Object.keys(normalize(style)).length === 0;
}

function same(a?: Style, b?: Style): boolean {
  return JSON.stringify(normalize(a)) === JSON.stringify(normalize(b));
}

function head(name: string): string {
  return name.split(".")[0];
}

function sorted<T>(entries: Iterable<[string, T]>): Record<string, T> {
  return Object.fromEntries([...entries].toSorted(([a], [b]) => a.localeCompare(b)));
}

function describe(report: Report): string {
  const entries = Object.entries(report.dropped);
  return entries.length ? `dropped ${entries.map(([name, count]) => `${count}× ${name}`).join(", ")}` : "dropped none";
}

function json(value: unknown): string {
  return `${JSON.stringify(value, null, 2)}\n`;
}

const previous = `${out}.old`;
if (existsSync(out)) rmSync(previous, { recursive: true, force: true });
else if (existsSync(previous)) renameSync(previous, out);

const failures: string[] = [];
const summary: string[] = [];
const names = new Set<string>();
const langs = new Map<string, string>();

const entries = [
  ...(await hostEntries()),
  asset("javascript"),
  asset("typescript"),
  asset("zig"),
  asset("markdown", ["markdown_inline"]),
];
const vendored = scan().flatMap((language) =>
  language.parser && language.filetype === language.name ? [language.parser] : [],
);
const clashes = vendored
  .filter((parser) => entries.some((entry) => entry.filetype === parser.filetype))
  .map((parser) => parser.filetype);
if (clashes.length) throw new Error(`tree-sitter/ folders clash with generated ones: ${clashes.join(", ")}`);
const remote = [
  ...new Set([
    ...vendored.map((parser) => parser.wasm),
    ...entries.flatMap((entry) => [entry.wasm, ...entry.queries.map((source) => source.query)]),
  ]),
].filter((source) => source.startsWith("https://"));
const files = new Map(await Promise.all(remote.map(async (url) => [url, await fetchCached(url)] as const)));
const local = (source: string) => files.get(source) ?? source;
const wasms = [
  ...new Set([
    ...vendored.map((parser) => local(parser.wasm)),
    ...entries.flatMap((entry) => entry.queries.map((source) => local(source.wasm))),
  ]),
];
await Parser.init();
const grammars = new Map(await Promise.all(wasms.map(async (file) => [file, await Language.load(file)] as const)));
const compile = (wasm: string, text: string) => new Query(grammars.get(local(wasm))!, text);

rmSync(staging, { recursive: true, force: true });
mkdirSync(staging);
for (const { filetype } of vendored)
  cpSync(path.join(out, filetype), path.join(staging, filetype), { recursive: true });

for (const entry of entries.toSorted((a, b) => a.filetype.localeCompare(b.filetype))) {
  const { filetype } = entry;
  const lang = nvimNames.get(filetype) ?? filetype;
  const dir = path.join(staging, filetype);
  const aliases = [...entry.aliases, ...(fences.get(filetype) ?? [])];
  const manifest: Manifest = aliases.length ? { aliases } : {};
  langs.set(filetype, lang);
  mkdirSync(dir);
  let reason = "kept";
  if (!kept.has(filetype)) {
    try {
      const { text, report } = convert(vagariSource(lang, nvimSource(lang)));
      const query = compile(entry.wasm, text);
      for (const name of query.captureNames.filter(styled)) names.add(name);
      const remoteWasm = entry.wasm.startsWith("https://");
      writeFileSync(path.join(dir, "highlights.scm"), text);
      writeFileSync(
        path.join(dir, "manifest.json"),
        json({ filetype: `vagari.${filetype}`, ...(remoteWasm ? { wasm: entry.wasm } : {}), ...manifest }),
      );
      if (!remoteWasm) copyFileSync(entry.wasm, path.join(dir, `tree-sitter-${filetype}.wasm`));
      summary.push(`${filetype}\tnvim main\t${describe(report)}`);
      continue;
    } catch (error) {
      if (!grammarMismatch(error)) {
        failures.push(`${filetype}: ${String(error)}`);
        continue;
      }
      reason = error.message;
    }
  }
  for (const source of entry.queries) {
    try {
      const query = compile(source.wasm, readFileSync(local(source.query), "utf8"));
      for (const name of query.captureNames.filter(styled)) names.add(name);
    } catch (error) {
      if (grammarMismatch(error)) reason += `; host query also fails: ${error.message}`;
      else failures.push(`${filetype} host query ${source.query}: ${String(error)}`);
    }
  }
  writeFileSync(path.join(dir, "manifest.json"), json(manifest));
  summary.push(`${filetype}\thost query\t${reason}`);
}

for (const { filetype, wasm, queries } of vendored) {
  try {
    const query = compile(wasm, readFileSync(queries.highlights[0], "utf8"));
    for (const name of query.captureNames.filter(styled)) names.add(name);
    langs.set(filetype, filetype);
    summary.push(`${filetype}\tvendored`);
  } catch (error) {
    failures.push(`${filetype}: ${String(error)}`);
  }
}

for (const name of names) {
  const current = renames.get(name);
  if (current) names.add(current);
}

const input = path.join(cache, "resolve-in.json");
const output = path.join(cache, "resolve-out.json");
const script = path.join(cache, "resolve.lua");
mkdirSync(cache, { recursive: true });
writeFileSync(input, JSON.stringify({ names: [...names].toSorted(), langs: [...new Set(langs.values())].toSorted() }));
writeFileSync(script, resolver);
rmSync(output, { force: true });
const nvim = Bun.spawnSync(
  [
    "nvim",
    "--headless",
    "--clean",
    "--cmd",
    `set rtp^=${vagari}`,
    "-c",
    "colorscheme vagari",
    "-c",
    `luafile ${script}`,
    "-c",
    "qa!",
  ],
  {
    env: {
      ...process.env,
      SYNC_IN: input,
      SYNC_OUT: output,
      SYNC_THALAMUS: path.join(vagari, "lua", "vagari", "thalamus.lua"),
    },
    stderr: "pipe",
  },
);
const stderr = nvim.stderr.toString().trim();
if (nvim.exitCode !== 0 || stderr || !existsSync(output)) failures.push(`nvim: ${stderr || "no output"}`);

if (failures.length === 0) {
  const resolved: Resolved = JSON.parse(readFileSync(output, "utf8"));
  const resolve = (table: Record<string, Style>, name: string) =>
    Object.hasOwn(table, name) ? table[name] : undefined;
  const base = new Map<string, Style>();
  for (const name of names) {
    const current = renames.get(name);
    const style = resolve(resolved.base, name);
    const effective = empty(style) && current ? resolve(resolved.base, current) : style;
    if (!empty(effective)) base.set(name, normalize(effective));
  }
  const inherited = (name: string) => base.get(name) ?? base.get(head(name));
  const overrides: [string, Record<string, Style>][] = [];
  for (const [filetype, lang] of langs) {
    const table = Object.hasOwn(resolved.languages, lang) ? resolved.languages[lang] : {};
    const differ = [...names]
      .map((name) => [name, resolve(table, name)] as const)
      .filter(([name, style]) => !(empty(style) && renames.has(name)) && !same(style, inherited(name)))
      .map(([name, style]): [string, Style] => [name, normalize(style)]);
    if (differ.length) overrides.push([filetype, sorted(differ)]);
  }
  const tint: [string, { color: string; alpha: number }][] = [];
  for (const [key, entry] of Object.entries(resolved.tint)) {
    if (!entry.color || entry.alpha === undefined) {
      failures.push(`tint ${key}: thalamus.lua no longer matches blend(p.<color>, <alpha>)`);
      continue;
    }
    const blended = rgbToHex(mix(entry.color, entry.alpha, RGBA.fromHex(entry.background)));
    if (blended !== entry.resolved) failures.push(`tint ${key}: blend gives ${blended}, nvim ${entry.resolved}`);
    tint.push([key, { color: entry.color, alpha: entry.alpha }]);
  }
  const vagariJson = {
    default: resolved.default,
    base: sorted(base),
    languages: sorted(overrides),
    tint: sorted(tint),
  };
  writeFileSync(path.join(staging, "vagari.json"), json(vagariJson));
}

console.log(summary.toSorted().join("\n"));
if (failures.length) {
  rmSync(staging, { recursive: true, force: true });
  console.error(`\nUnclassified failures:\n${failures.join("\n")}\n\ntree-sitter/ left unchanged.`);
  process.exit(1);
}
renameSync(out, previous);
renameSync(staging, out);
rmSync(previous, { recursive: true, force: true });
