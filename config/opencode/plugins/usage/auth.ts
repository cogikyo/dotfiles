import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";

export function resolveOpencodeDataDir() {
  const xdg = process.env.XDG_DATA_HOME?.trim();
  if (xdg) return path.join(path.resolve(xdg), "opencode");
  return path.join(os.homedir(), ".local", "share", "opencode");
}

export function authPath() {
  return path.join(resolveOpencodeDataDir(), "auth.json");
}

export function resolveOpencodeCacheDir() {
  const xdg = process.env.XDG_CACHE_HOME?.trim();
  if (xdg) return path.join(path.resolve(xdg), "opencode");
  return path.join(os.homedir(), ".cache", "opencode");
}

export function usageCacheDir() {
  return path.join(resolveOpencodeCacheDir(), "usage-sidebar");
}

export function usageCachePath(providerID: string) {
  return path.join(usageCacheDir(), `${providerID}.json`);
}

export function resolveOpencodeRuntimeDir() {
  const xdg = process.env.XDG_RUNTIME_DIR?.trim();
  if (xdg) return path.join(path.resolve(xdg), "opencode");

  const uid = typeof process.getuid === "function" ? process.getuid() : os.userInfo().uid;
  return path.join("/tmp", `opencode-${uid}`);
}

export function usageLockPath(providerID: string) {
  return path.join(resolveOpencodeRuntimeDir(), `usage-sidebar-${providerID}.lock`);
}

export async function readAuth<T>() {
  return JSON.parse(await fs.readFile(authPath(), "utf8")) as T;
}

export function claudeCredentialsPaths() {
  const env = process.env.CLAUDE_CONFIG_DIR?.trim();
  if (env) return [path.join(path.resolve(env), ".credentials.json")];

  const xdg = process.env.XDG_CONFIG_HOME?.trim();
  const configDir = xdg ? path.resolve(xdg) : path.join(os.homedir(), ".config");
  return [
    path.join(configDir, "claude", ".credentials.json"),
    path.join(os.homedir(), ".claude", ".credentials.json"),
  ];
}

export type ClaudeCredentials = {
  accessToken?: string;
  expiresAt?: string | number;
};

export async function readClaudeCredentials(): Promise<ClaudeCredentials[]> {
  const candidates: ClaudeCredentials[] = [];

  for (const credentialsPath of claudeCredentialsPaths()) {
    try {
      const parsed = JSON.parse(await fs.readFile(credentialsPath, "utf8")) as unknown;
      if (!parsed || typeof parsed !== "object") continue;

      const record = parsed as Record<string, unknown>;
      const credentials = record.claudeAiOauth ?? record;
      if (!credentials || typeof credentials !== "object") continue;
      candidates.push(credentials as ClaudeCredentials);
    } catch {
      continue;
    }
  }

  return candidates;
}
