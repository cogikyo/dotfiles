export const kinds = ["path", "command", "identifier", "number", "other"] as const;

export type Kind = (typeof kinds)[number] | "hex";

export type Probe = {
  which(word: string): boolean;
  exists(path: string): boolean;
};

const BUILTINS = new Set(["cd", "export", "source", "exec", "eval", "pushd", "popd", "unset", "alias"]);
const LITERALS = new Set(["true", "false", "null"]);
const FILES = new Set(["Makefile", "Dockerfile", "PKGBUILD", "Justfile", "LICENSE"]);
const EXTENSIONS = new Set(
  (
    "ts tsx js jsx mjs cjs json jsonc json5 md mdx go mod sum zig rs c h cc cpp hpp py rb lua sh bash zsh fish nu " +
    "toml yaml yml ini conf cfg env lock log txt csv tsv sql db sqlite html css scss svg png jpg jpeg gif webp ico pdf " +
    "wasm scm kdl rasi qml nix vim service timer socket target mount desktop rules xml diff patch gz zst xz tar zip iso " +
    "img ttf otf woff woff2 list lst age pub key pem tmpl"
  ).split(" "),
);

const NUMBER = [
  /^[-+]?\d[\d_,]*(\.\d+)?(e[-+]?\d+)?(%|[a-z]{1,3})?$/i,
  /^v?\d+(\.\d+)+([-+][0-9a-z.-]+)?$/i,
  /^0x[0-9a-f]+$/i,
  /^(?=[a-f]*\d)[0-9a-f]{7,64}$/,
  /^[0-9a-f]{7,40}\.\.\.?([0-9a-f]{7,40}|HEAD)$/,
  /^:\d+([-:,]\d+)*$/,
  /^#\d+$/,
  /^\d{4}-\d{2}-\d{2}([t ]\d{2}:\d{2}(:\d{2})?)?$/i,
  /^\d{1,2}:\d{2}(:\d{2})?$/,
  /^U\+[0-9A-F]{4,6}$/,
];
const IDENTIFIER = /^[-$#@]*[a-z_][\w$.:/#@+*-]*(\(\))?$/i;

function path(token: string, probe: Probe) {
  if (/^[a-z][a-z0-9+.-]*:\/\/\S/i.test(token)) return true;
  if (token.startsWith("@")) return false;
  const file = token.replace(/(?<=[^:]):\d+([-:,]\d+)*$/, "");
  if (/^(~|\.\.?|\.\w[\w.-]*)(\/|$)/.test(file) || file.endsWith("/")) return true;
  const slashes = file.split("/").length - 1;
  const name = file.slice(file.lastIndexOf("/") + 1);
  const dot = name.lastIndexOf(".");
  if (FILES.has(name) || /^\.\w[\w.-]*$/.test(name) || (dot > 0 && EXTENSIONS.has(name.slice(dot + 1).toLowerCase())))
    return true;
  if (file.startsWith("/")) return slashes > 1 || probe.exists(file);
  if (slashes > 1) return true;
  return slashes === 1 && probe.exists(file);
}

function command(word: string, probe: Probe) {
  return BUILTINS.has(word) || (/^[\w.+-]+$/.test(word) && probe.which(word));
}

export function kind(text: string, probe: Probe): Kind {
  const words = text.trim().split(/\s+/);
  const [token = ""] = words;
  if (words.length > 1) {
    const word = words.find((part) => !/^[A-Z_][A-Z0-9_]*=/.test(part));
    return word !== undefined && command(word, probe) ? "command" : "other";
  }
  if (token === "") return "other";
  if (/^#[0-9a-f]{6}$/i.test(token)) return "hex";
  if (LITERALS.has(token) || NUMBER.some((pattern) => pattern.test(token))) return "number";
  if (path(token, probe)) return "path";
  if (command(token, probe)) return "command";
  if (IDENTIFIER.test(token)) return "identifier";
  return "other";
}
