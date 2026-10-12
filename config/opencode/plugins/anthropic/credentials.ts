import { createHash, randomUUID } from "node:crypto";
import {
  chmodSync,
  mkdirSync,
  openSync,
  closeSync,
  readFileSync,
  realpathSync,
  renameSync,
  statSync,
  unlinkSync,
  writeFileSync,
  writeSync,
} from "node:fs";
import os from "node:os";
import path from "node:path";

const tokenURL = "https://claude.ai/v1/oauth/token";
const clientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e";
const lockDir = path.join(os.homedir(), ".local", "share", "opencode");
const lockTTL = 20_000;
const margin = 60_000;
const terminalErrors = new Set(["invalid_grant", "invalid_client", "unauthorized_client", "unsupported_grant_type"]);

export type Token = { accessToken: string; refreshToken: string; expiresAt: number };
export type Failure = "transient" | "login";

type Exchange = { token: Token } | { failure: "login" } | { failure: "transient"; retryAfter?: number };

const fresh = (token: Token, rejected?: string) =>
  token.accessToken !== rejected && token.expiresAt > Date.now() + margin;

function acquire(file: string) {
  const id = randomUUID();
  try {
    mkdirSync(path.dirname(file), { recursive: true });
  } catch {
    return undefined;
  }
  for (let attempt = 0; attempt < 2; attempt++) {
    try {
      const fd = openSync(file, "wx");
      writeSync(fd, JSON.stringify({ pid: process.pid, ts: Date.now(), id }));
      closeSync(fd);
      return {
        release() {
          try {
            if (readFileSync(file, "utf8").includes(id)) unlinkSync(file);
          } catch {}
        },
      };
    } catch (error) {
      if ((error as NodeJS.ErrnoException).code !== "EEXIST") return undefined;
    }
    try {
      if (Date.now() - statSync(file).mtimeMs <= lockTTL) return undefined;
      unlinkSync(file);
    } catch {}
  }
  return undefined;
}

async function exchange(refreshToken: string): Promise<Exchange> {
  try {
    const response = await fetch(tokenURL, {
      method: "POST",
      headers: { "content-type": "application/x-www-form-urlencoded" },
      body: new URLSearchParams({ grant_type: "refresh_token", client_id: clientID, refresh_token: refreshToken }),
      signal: AbortSignal.timeout(15_000),
    });
    const data = await response.json().catch(() => ({}));
    if (response.ok && typeof data.access_token === "string") {
      const now = Date.now();
      const expiresAt =
        typeof data.expires_at === "number" && data.expires_at > now
          ? data.expires_at
          : now + (data.expires_in ?? 36_000) * 1000;
      return {
        token: {
          accessToken: data.access_token,
          refreshToken: data.refresh_token ?? refreshToken,
          expiresAt: Math.trunc(expiresAt),
        },
      };
    }
    if (terminalErrors.has(typeof data.error === "string" ? data.error : data.error?.type)) return { failure: "login" };
    const seconds = Number.parseInt(response.headers.get("retry-after") ?? "", 10);
    return { failure: "transient", retryAfter: seconds > 0 ? seconds * 1000 : undefined };
  } catch {
    return { failure: "transient" };
  }
}

export class Credentials {
  private token?: Token;
  private dead?: string;
  private inflight?: Promise<Token | Failure>;
  private cooldown = { until: 0, count: 0 };
  private readonly file: string;
  private readonly lock: string;

  constructor(configDir: string) {
    this.file = path.join(configDir, ".credentials.json");
    const digest = createHash("sha256").update(`file:${configDir}`).digest("hex").slice(0, 16);
    this.lock = path.join(lockDir, `claude-auth-refresh-${digest}.lock`);
  }

  async get(): Promise<Token | Failure> {
    return this.token && fresh(this.token) ? this.token : this.renew();
  }

  async renew(rejected?: string): Promise<Token | Failure> {
    const stored = this.read();
    if (!stored) return "login";
    if (fresh(stored, rejected)) return (this.token = stored);
    this.inflight ??= this.refresh(stored.accessToken).finally(() => (this.inflight = undefined));
    return this.inflight;
  }

  private read(): Token | undefined {
    try {
      const data = JSON.parse(readFileSync(this.file, "utf8"));
      const entry = data.claudeAiOauth ?? data;
      if (typeof entry.accessToken !== "string" || typeof entry.refreshToken !== "string") return undefined;
      if (typeof entry.expiresAt !== "number") return undefined;
      return {
        accessToken: entry.accessToken,
        refreshToken: entry.refreshToken,
        expiresAt: Math.trunc(entry.expiresAt),
      };
    } catch {
      return undefined;
    }
  }

  private adopt(stale: string) {
    const stored = this.read();
    if (!stored || !fresh(stored, stale)) return undefined;
    this.cooldown = { until: 0, count: 0 };
    return (this.token = stored);
  }

  private async refresh(stale: string): Promise<Token | Failure> {
    if (Date.now() < this.cooldown.until) return this.adopt(stale) ?? "transient";
    const lock = acquire(this.lock);
    if (!lock) {
      for (let waited = 0; waited < 5_000; waited += 250) {
        const adopted = this.adopt(stale);
        if (adopted) return adopted;
        await Bun.sleep(250);
      }
      return "transient";
    }
    try {
      const current = this.read();
      if (!current) return "login";
      if (fresh(current, stale)) return (this.token = current);
      if (current.refreshToken === this.dead) return "login";
      const result = await exchange(current.refreshToken);
      if ("token" in result) {
        this.cooldown = { until: 0, count: 0 };
        this.write(result.token, current.accessToken);
        return (this.token = result.token);
      }
      if (result.failure === "login") this.dead = current.refreshToken;
      else this.backoff(result.retryAfter);
      return result.failure;
    } finally {
      lock.release();
    }
  }

  private backoff(retryAfter?: number) {
    const count = this.cooldown.count + 1;
    const scheduled = Math.min(60_000, 15_000 * 2 ** (count - 1)) * (0.5 + Math.random() * 0.5);
    this.cooldown = { until: Date.now() + Math.min(60_000, retryAfter ?? scheduled), count };
  }

  private write(token: Token, prior: string) {
    let temp = "";
    try {
      const target = realpathSync(this.file);
      temp = `${target}.${process.pid}.tmp`;
      const data = JSON.parse(readFileSync(target, "utf8"));
      const entry = data.claudeAiOauth ?? data;
      if (entry.accessToken !== prior) return;
      Object.assign(entry, token);
      const mode = statSync(target).mode & 0o777;
      writeFileSync(temp, JSON.stringify(data), { mode });
      chmodSync(temp, mode);
      renameSync(temp, target);
    } catch (error) {
      try {
        if (temp) unlinkSync(temp);
      } catch {}
      console.warn(`anthropic: refreshed token not written to ${this.file}: ${String(error)}`);
    }
  }
}
