import os from "node:os";
import path from "node:path";
import { z } from "zod";
import { readJson } from "../shared/file.ts";
import { lenient } from "./types.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Credential lookup                                                                             │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

// ├─ OpenCode auth.json ──────────────────────────────────────────────────────────────────────────┤

/** Reads OpenCode credentials, omitting invalid provider entries; file and root-parse errors propagate. */
export async function readAuth() {
  return readJson(path.join(dataDir(), "auth.json"), Auth);
}

function dataDir() {
  const xdg = process.env.XDG_DATA_HOME?.trim();
  if (xdg) return path.join(path.resolve(xdg), "opencode");
  return path.join(os.homedir(), ".local", "share", "opencode");
}

const OAuth = z.object({
  type: z.literal("oauth"),
  access: z.string().min(1),
  accountId: lenient(z.string()),
});
const Api = z.object({ type: z.literal("api"), key: z.string().min(1) });

const Auth = z.object({
  anthropic: lenient(OAuth),
  openai: lenient(OAuth),
  cursor: lenient(OAuth),
  "opencode-go": lenient(Api),
});

// ├─ Claude credentials ──────────────────────────────────────────────────────────────────────────┤

/** Reads valid Claude credentials in path order, skipping unreadable files; `CLAUDE_CONFIG_DIR` overrides the default paths. */
export async function readClaudeCredentials() {
  const candidates = await Promise.all(
    claudeCredentialsPaths().map((file) => readJson(file, CredentialsFile).catch(() => undefined)),
  );
  return candidates.filter((candidate) => candidate !== undefined);
}

function claudeCredentialsPaths() {
  const env = process.env.CLAUDE_CONFIG_DIR?.trim();
  if (env) return [path.join(path.resolve(env), ".credentials.json")];

  const xdg = process.env.XDG_CONFIG_HOME?.trim();
  const configDir = xdg ? path.resolve(xdg) : path.join(os.homedir(), ".config");
  return [path.join(configDir, "claude", ".credentials.json"), path.join(os.homedir(), ".claude", ".credentials.json")];
}

const Credentials = z.object({
  accessToken: lenient(z.string()),
  expiresAt: lenient(z.union([z.string(), z.number()])),
});

const CredentialsFile = Credentials.extend({ claudeAiOauth: Credentials.nullish() }).transform(
  ({ claudeAiOauth, ...root }) => claudeAiOauth ?? root,
);

// ├─ Expiry ──────────────────────────────────────────────────────────────────────────────────────┤

/** Treats missing, invalid, or elapsed credential expiry as expired; numeric times are milliseconds. */
export function isExpired(expiresAt: string | number | undefined) {
  const ms = typeof expiresAt === "number" ? expiresAt : Date.parse(expiresAt ?? "");
  return !Number.isFinite(ms) || Date.now() >= ms;
}
