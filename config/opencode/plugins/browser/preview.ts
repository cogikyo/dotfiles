import { hostname } from "node:os";
import { Schema } from "effect";
import type { MCP } from "./mcp.ts";

type Result = Awaited<ReturnType<MCP["call"]>>;

export interface Account {
  readonly origins: readonly string[];
  readonly user: string;
  readonly secret: () => Promise<string>;
}

const entry = "leadpier-preview";

export const leadpier: Account = {
  origins: ["https://lpadmin.cullyn.dev", "https://lpuser.cullyn.dev"],
  user: "cullyn@trendcapital.com",
  secret: keyring,
};

const Navigation = Schema.Struct({
  type: Schema.optional(Schema.String),
  url: Schema.String,
  initScript: Schema.optional(Schema.String),
  timeout: Schema.optional(Schema.Number),
});

const Reply = Schema.Struct({
  errorCode: Schema.String,
  data: Schema.Unknown,
});

const Session = Schema.Struct({ token: Schema.NonEmptyString });

const codes = new Set(["ERROR", "DENIED_ACCESS", "NOT_IMPLEMENTED", "UNAUTHORIZED"]);

interface Lane {
  tail: Promise<unknown>;
  auth?: string;
}

export class Preview {
  private readonly origins: ReadonlySet<string>;
  private readonly lanes = new Map<string, Lane>();

  constructor(private readonly account: Account) {
    this.origins = new Set(account.origins);
  }

  call(sessionID: string, browser: () => Promise<MCP>, name: string, input: unknown) {
    const lane = this.lanes.get(sessionID) ?? { tail: Promise.resolve() };
    this.lanes.set(sessionID, lane);
    // Closing during authentication must not reopen the browser when the login resolves.
    const live = () => {
      if (this.lanes.get(sessionID) !== lane) throw new Error("browser session closed");
    };
    const target = this.target(name, input);
    // Keep the two-call new_page sequence atomic; MCP serialization covers only individual calls.
    const next = lane.tail.then(async () => {
      live();
      if (!target) {
        const mcp = await browser();
        live();
        return mcp.call(name, input);
      }
      lane.auth ??= await this.authenticate(target.origin);
      const seed = script(target.origin, lane.auth);
      live();
      const mcp = await browser();
      live();
      if (name === "new_page") return open(mcp, target.input, seed);
      const initScript = target.input.initScript === undefined ? seed : `${seed}\n${target.input.initScript}`;
      return mcp.call(name, { ...target.input, initScript });
    });
    lane.tail = next.catch(() => undefined);
    return next;
  }

  forget(sessionID: string) {
    this.lanes.delete(sessionID);
  }

  clear() {
    this.lanes.clear();
  }

  private target(name: string, input: unknown) {
    if (name !== "navigate_page" && name !== "new_page") return;
    if (!Schema.is(Navigation)(input)) return;
    if (name === "navigate_page" && (input.type ?? "url") !== "url") return;
    const origin = URL.parse(input.url)?.origin;
    if (origin === undefined || !this.origins.has(origin)) return;
    return { input, origin };
  }

  private async authenticate(origin: string) {
    const password = await this.account.secret();
    let response: Response;
    try {
      response = await fetch(`${origin}/v1/frontend/authenticate`, {
        method: "POST",
        redirect: "manual",
        headers: {
          Accept: "application/json",
          "Content-Type": "application/json",
          "User-Agent": "opencode-browser-qa",
        },
        body: JSON.stringify({ username: this.account.user, password }),
        signal: AbortSignal.timeout(15_000),
      });
    } catch (error) {
      throw unreachable(error instanceof Error ? error.message : "network error");
    }
    if (response.status >= 300 && response.status < 400) throw unreachable(`HTTP ${response.status} redirect`);

    let body: unknown;
    try {
      body = JSON.parse(await response.text());
    } catch {
      throw new Error(`preview sign-in failed: HTTP ${response.status}, not JSON`);
    }
    if (!Schema.is(Reply)(body))
      throw new Error(`preview sign-in failed: HTTP ${response.status} with an unexpected response shape`);
    // Unknown backend error codes may contain reflected passwords or tokens.
    if (body.errorCode !== "NO_ERROR" && !codes.has(body.errorCode))
      throw new Error(`preview sign-in failed: unrecognized error code (HTTP ${response.status})`);
    if (body.errorCode !== "NO_ERROR")
      throw new Error(
        `preview sign-in failed as ${this.account.user}: ${body.errorCode} (HTTP ${response.status}); if the password changed, Cullyn needs to re-store it with \`cogictl secret set ${entry}\``,
      );
    if (!Schema.is(Session)(body.data)) throw new Error("preview sign-in failed: the response carried no token");
    return JSON.stringify({ ...body.data, isAuthenticated: true });
  }
}

export async function keyring() {
  const timeout = AbortSignal.timeout(10_000);
  let proc: Bun.Subprocess<"ignore", "pipe", "pipe">;
  try {
    proc = Bun.spawn(["cogictl", "--plain", "secret", "get", entry, "--for", "browser QA"], {
      stdin: "ignore",
      stdout: "pipe",
      stderr: "pipe",
      signal: timeout,
    });
  } catch (error) {
    throw new Error(
      `preview sign-in unavailable: cannot run cogictl (${error instanceof Error ? error.message : "spawn failed"}); is cogictl installed on PATH?`,
      { cause: error },
    );
  }
  const [stdout, stderr, code] = await Promise.all([
    new Response(proc.stdout).text(),
    new Response(proc.stderr).text(),
    proc.exited,
  ]);
  if (code === 0) return stdout;
  if (timeout.aborted) throw new Error("preview sign-in unavailable: cogictl timed out; is the keyring unlocked?");
  if (stderr.includes("secret not stored"))
    throw new Error(
      `preview sign-in unavailable: no \`${entry}\` secret is stored; Cullyn needs to run \`cogictl secret set ${entry}\``,
    );
  throw new Error(`preview sign-in unavailable: cogictl exited ${code}: ${stderr.trim().slice(-500) || "no stderr"}`);
}

async function open(mcp: MCP, input: typeof Navigation.Type, seed: string) {
  const opened = await mcp.call("new_page", { ...input, url: "about:blank" });
  if (opened.isError) return opened;
  const result = await mcp.call("navigate_page", {
    type: "url",
    url: input.url,
    initScript: seed,
    ...(input.timeout === undefined ? {} : { timeout: input.timeout }),
  });
  return { ...result, isError: failed(result) };
}

function script(origin: string, auth: string) {
  // A process-side seeded flag goes stale after Chromium relaunches or new browser contexts.
  return `if (location.origin === ${JSON.stringify(origin)} && localStorage.getItem("authentication") === null) localStorage.setItem("authentication", ${JSON.stringify(auth)});`;
}

function failed(result: Result) {
  return (
    result.isError || result.content.some((part) => part.type === "text" && part.text.includes("Unable to navigate"))
  );
}

function unreachable(detail: string) {
  return new Error(`preview unreachable from ${hostname()} (${detail}); is the Trend VPN up?`);
}
