import { extname } from "node:path";
import { fileURLToPath } from "node:url";
import { scheme } from "../paths/commit.ts";

const WEB = "\u{f0ac}";
const COMMIT = "\u{f0718}";
const DIRECTORY = "\u{e613}";
const FILE = "\u{ea7b}";

const EXTENSIONS = new Map([
  ["go", "\u{e627}"],
  ["ts", "\u{e628}"],
  ["tsx", "\u{e7ba}"],
  ["js", "\u{e60c}"],
  ["json", "\u{e60b}"],
  ["jsonc", "\u{e60b}"],
  ["md", "\u{f48a}"],
  ["lua", "\u{e620}"],
  ["sh", "\u{e795}"],
  ["zsh", "\u{e795}"],
  ["py", "\u{e606}"],
  ["rs", "\u{e68b}"],
  ["toml", "\u{e6b2}"],
  ["yaml", "\u{e8eb}"],
  ["yml", "\u{e8eb}"],
  ["html", "\u{e736}"],
  ["css", "\u{e6b8}"],
  ["scss", "\u{e603}"],
  ["sql", "\u{e706}"],
  ["png", "\u{e60d}"],
  ["jpg", "\u{e60d}"],
  ["svg", "\u{f0721}"],
  ["pdf", "\u{eaeb}"],
  ["txt", "\u{f0219}"],
  ["log", "\u{f0331}"],
]);

/** Picks the Nerd Font glyph for a link URL. */
export function glyph(url: string, isDirectory: (path: string) => boolean) {
  if (url.startsWith(scheme)) return COMMIT;
  if (!url.startsWith("file:")) return WEB;
  let path: string;
  try {
    path = fileURLToPath(url);
  } catch {
    return FILE;
  }
  if (path.endsWith("/") || isDirectory(path)) return DIRECTORY;
  return EXTENSIONS.get(extname(path).slice(1).toLowerCase()) ?? FILE;
}
