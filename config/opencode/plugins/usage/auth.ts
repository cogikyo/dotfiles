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

export function resolveClaudeConfigDir() {
  const env = process.env.CLAUDE_CONFIG_DIR?.trim();
  if (env) return path.resolve(env);
  return path.join(os.homedir(), ".claude");
}

export function claudeCredentialsPath() {
  return path.join(resolveClaudeConfigDir(), ".credentials.json");
}

export type ClaudeCredentials = {
  accessToken?: string;
  expiresAt?: string;
};

export async function readClaudeCredentials(): Promise<ClaudeCredentials | undefined> {
  try {
    const parsed = JSON.parse(await fs.readFile(claudeCredentialsPath(), "utf8")) as unknown;
    if (!parsed || typeof parsed !== "object") return undefined;
    return parsed as ClaudeCredentials;
  } catch {
    return undefined;
  }
}
