import { readdirSync, realpathSync } from "node:fs";
import { lstat, readlink, realpath } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";

export const home = homedir();
export const scratch = path.join(home, ".local/state/opencode/scratch");

const stores = [".local/share", ".local/state"].map((store) => path.join(home, store));
const opencodeSecret = /^(?:auth\.json|account\.json|opencode\.db(?:-wal|-shm)?|service(?:-[\w.-]+)?\.json)$/u;
const claudeStores = [
  ".credentials.json",
  ".claude.json",
  "projects",
  "sessions",
  "history.jsonl",
  "file-history",
  "shell-snapshots",
  "session-env",
  "backups",
  "debug",
  "daemon/control.key",
];
const fixed = [
  ".ssh",
  ".gnupg",
  ".password-store",
  ".auth",
  ".local/share/keyrings",
  ".local/.owm_api_key",
  ".local/share/dotfiles",
  ".local/share/mkcert/rootCA-key.pem",
  ".local/share/dev-certs/localhost-key.pem",
  ".local/share/vitest/.vitest-secret-token",
  ".local/share/atuin",
  ".local/share/Trash",
  ".local/share/hyprd/firefox-profiles",
  ".local/state/hyprd/firefox-profiles",
  ".local/state/hyprd/browser-sessions",
  ".local/state/firefox-sessions",
  ".local/state/dotfiles/secrets-manifest.approved",
  ".docker/config.json",
  ".histfile",
  ".pki",
  ".config/gh",
  ".mozilla/firefox",
  ".config/mozilla/firefox",
  ".config/chromium",
  ".config/BraveSoftware",
  ".config/Slack",
  ".config/Grok Bot",
  ".config/tea",
  ".zoom",
  ".tableplus/settings/connections.json",
  ".claude.json",
  ".config/claude",
  ".local/state/grok-bot",
  ".cache/spotify",
  ".codex",
  ".grok",
  ".grokbot",
  "LeadPier/.pier/secrets.yaml",
  "LeadPier/frontend/.env.local",
  "downloads/test-project-1782174228707-78cbdca843f8.json",
  "downloads/Addrawtech CAPI Firing - Sheet1.csv",
].map((secret) => path.join(home, secret));
const system = ["/proc", "/sys", "/dev"];
const churned = new Set(["tmp", "cache"]);
const maxLinks = 40;

function entries(dir: string) {
  try {
    return readdirSync(dir, { withFileTypes: true });
  } catch (error) {
    const code = (error as NodeJS.ErrnoException).code;
    if (code === "ENOENT" || code === "ENOTDIR") return [];
    throw error;
  }
}

export function secrets() {
  const dirs = [path.join(home, ".config/opencode")];
  for (const store of stores) {
    for (const { name } of entries(store).filter((entry) => entry.name.startsWith("opencode"))) {
      dirs.push(path.join(store, name), path.join(store, name, "opencode"));
    }
  }
  const files = dirs.flatMap((dir) =>
    entries(dir)
      .filter((entry) => opencodeSecret.test(entry.name))
      .map((entry) => path.join(dir, entry.name)),
  );
  const claude = entries(home)
    .filter((entry) => entry.isDirectory() && entry.name.startsWith(".claude"))
    .flatMap((entry) => claudeStores.map((store) => path.join(home, entry.name, store)));
  return [...fixed, ...claude, ...files];
}

function canonical(target: string) {
  try {
    return realpathSync(target);
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return undefined;
    throw error;
  }
}

function within(child: string, parent: string) {
  return child === parent || child.startsWith(parent === "/" ? "/" : `${parent}/`);
}

function churn(target: string, roots: string[]) {
  return roots.some((root) => {
    if (!within(target, root) || target === root) return false;
    const [, dir] = path.relative(root, target).split(path.sep);
    return churned.has(dir);
  });
}

type Fence = { hidden: string[]; roots: string[] };

function fence(): Fence {
  const lexical = secrets();
  const hidden = [...lexical, ...lexical.flatMap((secret) => canonical(secret) ?? [])];
  const real = canonical(scratch);
  return { hidden, roots: real && real !== scratch ? [scratch, real] : [scratch] };
}

function refusal(target: string, rules: Fence) {
  if (system.some((dir) => within(target, dir))) return `${target} is under /proc, /sys, or /dev`;
  if (churn(target, rules.roots)) {
    return `${target} is under a session scratch tmp/ or cache/ directory, whose entries can be swapped while a host tool reads them`;
  }
  if (rules.hidden.some((secret) => within(target, secret))) return `${target} is hidden from read-only agents`;
  return undefined;
}

function sweep(target: string, rules: Fence) {
  if (target === home) return `${target} is the home directory`;
  if (rules.hidden.some((secret) => within(secret, target)))
    return `${target} contains paths hidden from read-only agents`;
  if (rules.roots.some((root) => within(root, target) || path.dirname(target) === root)) {
    return `${target} contains session scratch tmp/ or cache/ directories`;
  }
  return undefined;
}

export function expand(target: string) {
  if (target === "~") return home;
  return target.startsWith("~/") ? path.join(home, target.slice(2)) : target;
}

export async function confine(target: string, base: string, recursive: boolean) {
  const lexical = path.resolve(base, expand(target));
  const rules = fence();
  const check = (candidate: string) => {
    const reason = refusal(candidate, rules) ?? (recursive ? sweep(candidate, rules) : undefined);
    if (reason) throw new Error(reason);
  };
  check(lexical);
  const resolved = await trace(lexical, rules);
  check(resolved);
  if ((await realpath(lexical)) !== resolved) throw new Error(`${lexical} changed while it was being resolved`);
  return resolved;
}

async function trace(lexical: string, rules: Fence) {
  const pending = lexical.split(path.sep).filter(Boolean);
  let current = "/";
  let links = 0;
  while (pending.length > 0) {
    const part = pending.shift() as string;
    if (part === ".") continue;
    const next = part === ".." ? path.dirname(current) : path.join(current, part);
    const reason = refusal(next, rules);
    if (reason) throw new Error(reason);
    if (part === ".." || !(await lstat(next)).isSymbolicLink()) {
      current = next;
      continue;
    }
    if (++links > maxLinks) throw new Error(`${lexical} has too many levels of symbolic links`);
    const link = await readlink(next);
    if (path.isAbsolute(link)) current = "/";
    pending.unshift(...link.split(path.sep).filter(Boolean));
  }
  return current;
}
