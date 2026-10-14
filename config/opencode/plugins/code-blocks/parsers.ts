import { existsSync, readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import type { FiletypeParserOptions } from "@opentui/core";

export const root = path.resolve(import.meta.dirname, "..", "..", "tree-sitter");

export type Manifest = { filetype?: string; wasm?: string; aliases?: string[] };

export type Language = { name: string; filetype: string; aliases: string[]; parser?: FiletypeParserOptions };

export function scan(): Language[] {
  return readdirSync(root, { withFileTypes: true })
    .filter((entry) => entry.isDirectory())
    .map((entry) => load(entry.name));
}

function load(name: string): Language {
  const file = (base: string) => path.join(root, name, base);
  const manifest: Manifest = existsSync(file("manifest.json"))
    ? JSON.parse(readFileSync(file("manifest.json"), "utf8"))
    : {};
  const filetype = manifest.filetype ?? name;
  const aliases = manifest.aliases ?? [];
  const highlights = file("highlights.scm");
  if (!existsSync(highlights)) return { name, filetype, aliases };
  const wasm = manifest.wasm ?? file(`tree-sitter-${name}.wasm`);
  if (!manifest.wasm && !existsSync(wasm)) throw new Error(`tree-sitter/${name} is missing ${path.basename(wasm)}`);
  const injections = file("injections.scm");
  return {
    name,
    filetype,
    aliases,
    parser: {
      filetype,
      ...(manifest.filetype ? {} : { aliases }),
      wasm,
      queries: {
        highlights: [highlights],
        ...(existsSync(injections) ? { injections: [injections] } : {}),
      },
    },
  };
}
