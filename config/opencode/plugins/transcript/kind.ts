export const kinds = ["path", "command", "danger", "identifier", "number", "other"] as const;

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
    "toml yaml yml ini conf cfg txt csv tsv sql db sqlite html css scss svg png jpg jpeg gif webp ico pdf " +
    "wasm scm kdl rasi qml nix vim service timer mount desktop rules xml diff patch gz zst xz tar zip iso " +
    "img ttf otf woff woff2 list lst age pub pem tmpl"
  ).split(" "),
);
const AMBIGUOUS = new Set(["log", "env", "target", "key", "lock", "socket"]);

const NUMBER = [
  /^[-+]?\d[\d_,]*(\.\d+)?(e[-+]?\d+)?(%|[a-z]{1,3})?$/i,
  /^v?\d+(\.\d+)+([-+][0-9a-z.-]+)?$/i,
  /^0x[0-9a-f]+$/i,
  /^(?=[a-f]*\d)[0-9a-f]{7,64}$/,
  /^[0-9a-f]{7,40}\.\.\.?([0-9a-f]{7,40}|HEAD)$/,
  /^:\d+([-:,]\d+)*$/,
  /^#\d+$/,
  /^\d{4}-\d{2}-\d{2}([t ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(z|[+-]\d{2}(:?\d{2})?)?)?$/i,
  /^\d{1,2}:\d{2}(:\d{2})?$/,
  /^U\+[0-9A-F]{4,6}$/,
];
const IDENTIFIER = /^[-$#@]*[a-z_][\w$.:/#@+*-]*(\(\))?$/i;
const DECLARATION =
  /^(export\s+(default|const|let|var|function|class|async|type|interface|enum|declare|abstract)\b|export\s*[{*]|import\s+[^-\s])/;

function path(token: string, probe: Probe) {
  if (/^[a-z][a-z0-9+.-]*:\/\/\S/i.test(token)) return true;
  if (token.startsWith("@")) return false;
  const file = token.replace(/(?<=[^:]):\d+([-:,]\d+)*$/, "");
  if (/^(~|\.\.?|\.\w[\w.-]*)(\/|$)/.test(file) || file.endsWith("/")) return true;
  const slashes = file.split("/").length - 1;
  const name = file.slice(file.lastIndexOf("/") + 1);
  const dot = name.lastIndexOf(".");
  const extension = dot > 0 ? name.slice(dot + 1).toLowerCase() : "";
  if (FILES.has(name) || /^\.\w[\w.-]*$/.test(name) || EXTENSIONS.has(extension)) return true;
  if (AMBIGUOUS.has(extension)) return slashes > 0 || probe.exists(file);
  if (file.startsWith("/")) return slashes > 1 || probe.exists(file);
  if (slashes > 1) return true;
  return slashes === 1 && probe.exists(file);
}

function command(word: string, probe: Probe) {
  if (/^(\.{1,2}|~)\//.test(word)) return true;
  if (word.startsWith("/")) return probe.exists(word);
  return BUILTINS.has(word) || (/^[\w.+-]+$/.test(word) && probe.which(word));
}

type Rule = (args: readonly string[]) => boolean;

const flag =
  (short: string, ...long: string[]): Rule =>
  (args) =>
    args.some(
      (arg) =>
        (/^-[a-z]+$/i.test(arg) && arg.split("").some((letter) => short.includes(letter))) ||
        long.some((name) => arg === name || arg.startsWith(`${name}=`)),
    );
const operands = (args: readonly string[]) => args.filter((arg) => !arg.startsWith("-"));
const verb =
  (...verbs: string[]): Rule =>
  (args) =>
    verbs.includes(operands(args)[0] ?? "");
const any: Rule = (args) => args.length > 0;
const always: Rule = () => true;

const GIT: Record<string, Rule> = {
  push: flag("f", "--force", "--force-with-lease"),
  reset: (args) => args.includes("--hard"),
  clean: flag("f", "--force"),
  checkout: (args) => args.includes("--") || args.includes("."),
  restore: (args) => operands(args).length > 0 && (!flag("S", "--staged")(args) || flag("W", "--worktree")(args)),
  branch: flag("D"),
  rebase: (args) => !args.includes("--abort"),
  stash: verb("drop", "clear"),
};

const DANGER: Record<string, Rule> = {
  sudo: any,
  doas: any,
  pkexec: any,
  su: any,
  run0: any,
  rm: flag("rRf", "--recursive", "--force"),
  shred: any,
  dd: (args) => args.some((arg) => arg.startsWith("of=")),
  mkfs: any,
  wipefs: any,
  truncate: any,
  find: (args) => args.includes("-delete") || args.some((arg, at) => arg === "-exec" && args[at + 1] === "rm"),
  git(args) {
    let at = 0;
    while (args[at]?.startsWith("-")) at += args[at] === "-C" || args[at] === "-c" ? 2 : 1;
    const [sub = "", ...rest] = args.slice(at);
    return GIT[sub]?.(rest) ?? false;
  },
  systemctl: verb("stop", "disable", "mask", "kill", "reboot", "poweroff"),
  kill: (args) => /(^| )-(s )?(9|KILL|SIGKILL)( |$)/.test(args.join(" ")),
  pkill: any,
  killall: any,
  reboot: always,
  poweroff: always,
  shutdown: always,
  dctl(args) {
    const [first, second = ""] = operands(args);
    return first === "install" || (first === "iso" && ["usb", "release"].includes(second));
  },
};
const PREFIXES = new Set(["$", "!", "do", "then", "else", "env", "exec", "command", "nohup", "time", "xargs"]);

function danger(text: string) {
  return text.split(/[|&;(){}\n]/).some((segment) => {
    const parts = segment.trim().split(/\s+/);
    let at = 0;
    while (at < parts.length && (/^[A-Z_][A-Z0-9_]*=/.test(parts[at]) || PREFIXES.has(parts[at]))) {
      at++;
      while (parts[at]?.startsWith("-")) at++;
    }
    const [name = "", ...args] = parts.slice(at);
    return DANGER[name.replace(/^mkfs\..+/, "mkfs")]?.(args) ?? false;
  });
}

export function kind(text: string, probe: Probe): Kind {
  const token = text.trim();
  if (token === "") return "other";
  if (/^#[0-9a-f]{6}$/i.test(token)) return "hex";
  if (LITERALS.has(token) || NUMBER.some((pattern) => pattern.test(token))) return "number";
  const words = token.split(/\s+/);
  if (words.length > 1) {
    if (DECLARATION.test(token)) return "other";
    if (danger(token)) return "danger";
    const word = words.find((part) => !/^[A-Z_][A-Z0-9_]*=/.test(part));
    return word !== undefined && command(word, probe) ? "command" : "other";
  }
  if (path(token, probe)) return "path";
  if (command(token, probe)) return danger(token) ? "danger" : "command";
  if (IDENTIFIER.test(token)) return "identifier";
  return "other";
}
