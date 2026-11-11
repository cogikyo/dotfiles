import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { Plugin } from "@opencode/plugin";
import type { Session } from "@opencode/schema/session";
import { Effect, Fiber } from "effect";
import { claudeAccounts } from "./accounts.ts";
import { isLongContextError, LONG_CONTEXT_BETAS } from "./claude-auth/betas.js";
import { config } from "./claude-auth/model-config.js";
import { SYSTEM_IDENTITY, transformBody, transformResponseStream } from "./claude-auth/transforms.js";
import { clearing } from "./clearing.ts";
import { Credentials, type Failure } from "./credentials.ts";
import { sign } from "./headers.ts";
import { capped, ranked, survey } from "./pool.ts";

const cacheControl = { type: "ephemeral", ttl: "1h" };
const renewEvery = 5 * 60_000;

type Account = {
  id: string;
  main: boolean;
  loginCommand: string;
  credentials: Credentials;
  excluded: Map<string, Set<string>>;
  until: number;
  sticky: Map<string, string>;
};

type Signed = {
  url: string;
  base: Headers;
  body: string | undefined;
  model: string;
  account: Account;
  token: string;
  cleared: boolean;
};

type Pending = Signed | { failure: Response };

function apiError(status: number, type: string, message: string, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify({ type: "error", error: { type, message } }), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
}

async function detectVersion() {
  if (process.env.ANTHROPIC_CLI_VERSION) return Effect.void;
  try {
    const { stdout } = await promisify(execFile)("claude", ["--version"], { timeout: 3000 });
    const version = stdout.trim().split(/\s+/)[0];
    if (!/^\d+\.\d+\.\d+/.test(version)) throw new Error(`unexpected output ${JSON.stringify(stdout.trim())}`);
    process.env.ANTHROPIC_CLI_VERSION = version;
    return Effect.logInfo("anthropic: ANTHROPIC_CLI_VERSION from claude --version", { version });
  } catch (error) {
    return Effect.logWarning("anthropic: claude --version failed", {
      ccVersion: config.ccVersion,
      error: String(error),
    });
  }
}

function shape(text: string) {
  const body = JSON.parse(text, (key, value) => (key === "cache_control" && value ? cacheControl : value));
  if (typeof body.system === "string") body.system = [{ type: "text", text: body.system }];
  body.system ??= [];
  if (!body.system.some((entry: { text?: string }) => entry.text?.startsWith(SYSTEM_IDENTITY)))
    body.system.unshift({ type: "text", text: SYSTEM_IDENTITY });
  return { model: String(body.model ?? "unknown"), body: transformBody(JSON.stringify(body)) as string };
}

export default Plugin.define({
  id: "anthropic",
  async setup(ctx) {
    const fiber = Fiber.getCurrent();
    if (!fiber) throw new Error("anthropic: setup started outside the plugin fiber");
    const log = Effect.runForkWith(fiber.context);
    log(await detectVersion());

    const clear = clearing(log);
    const registry = new Map<string, Account>(
      Object.values(claudeAccounts).map(({ id, main, configDir, loginCommand }) => [
        id,
        {
          id,
          main,
          loginCommand,
          credentials: new Credentials(configDir, log),
          excluded: new Map(),
          until: 0,
          sticky: new Map(),
        },
      ]),
    );
    const pending = new WeakMap<Request, Pending>();
    const roots = new Map<string, Promise<boolean>>();

    const root = (sessionID: Session.ID) => {
      let known = roots.get(sessionID);
      if (!known) {
        known = ctx.session.get({ sessionID }).then(
          (session) => !session.parentID,
          (error: unknown) => {
            roots.delete(sessionID);
            log(Effect.logWarning("anthropic: session lookup failed", { sessionID, error: String(error) }));
            return true;
          },
        );
        roots.set(sessionID, known);
      }
      return known;
    };

    const timers = [...registry.values()].map(({ credentials }) => {
      const renewEarly = () =>
        credentials
          .renewEarly()
          .catch((error: unknown) =>
            log(Effect.logWarning("anthropic: early renewal failed", { error: String(error) })),
          );
      void renewEarly();
      return setInterval(() => void renewEarly(), renewEvery);
    });

    const failure = (account: Account, kind: Failure) =>
      kind === "transient"
        ? apiError(429, "rate_limit_error", "Claude token refresh is rate-limited; retry shortly.", {
            "retry-after": "5",
          })
        : apiError(
            401,
            "authentication_error",
            `${account.id}: Claude credentials are unavailable or expired. Run ${account.loginCommand}.`,
          );

    const open = async (model: string, skip: Account, sessionID: Session.ID) => {
      const top = await root(sessionID);
      const now = Date.now();
      const list = (await survey(model, now)).map((usage) => {
        const until = registry.get(usage.id)!.until;
        return until > now ? { ...usage, capped: [...usage.capped, until] } : usage;
      });
      return ranked(list)
        .filter((usage) => usage.id !== skip.id)
        .map((usage) => registry.get(usage.id)!)
        .filter((account) => account.main || !top);
    };

    const route = async (own: Account, sessionID: Session.ID, model: string) => {
      if (own.until <= Date.now()) {
        own.sticky.clear();
        return own;
      }
      const kept = registry.get(own.sticky.get(sessionID) ?? "");
      if (kept && kept.until <= Date.now()) return kept;
      const [next] = await open(model, own, sessionID);
      if (!next) return own;
      own.sticky.set(sessionID, next.id);
      log(
        Effect.logInfo("anthropic: routed around capped account", {
          from: own.id,
          to: next.id,
          sessionID,
          until: new Date(own.until).toISOString(),
        }),
      );
      return next;
    };

    for (const own of registry.values()) {
      await ctx.session.hook(
        "http.request",
        async (event) => {
          const request = event.request;
          const base = new Headers(request.headers);
          base.delete("x-api-key");
          const url = new URL(request.url);
          if (url.pathname === "/v1/messages" && !url.searchParams.has("beta")) url.searchParams.set("beta", "true");
          const text = request.body ? await request.clone().text() : undefined;
          const shaped = text ? shape(text) : { model: "unknown", body: undefined };
          let account = await route(own, event.sessionID, shaped.model);
          let found = await account.credentials.get();
          if (typeof found === "string" && account !== own) {
            log(
              Effect.logWarning("anthropic: failover account unavailable", {
                providerID: account.id,
                failure: found,
              }),
            );
            own.sticky.delete(event.sessionID);
            account = own;
            found = await own.credentials.get();
          }
          if (typeof found === "string") {
            event.request = new Request(request, { headers: base });
            pending.set(event.request, { failure: failure(own, found) });
            return;
          }
          const cleared = clear.inject(event.kind, shaped.body);
          const state = {
            url: url.href,
            base,
            body: cleared ?? shaped.body,
            model: shaped.model,
            account,
            token: found.accessToken,
            cleared: cleared !== undefined,
          };
          event.request = new Request(state.url, {
            method: request.method,
            headers: sign(base, state.token, state.model, account.excluded.get(state.model)),
            body: state.body,
            signal: request.signal,
          });
          pending.set(event.request, state);
        },
        { providerID: own.id },
      );

      await ctx.session.hook(
        "http.response",
        async (event) => {
          const state = pending.get(event.request);
          if (!state) return;
          if ("failure" in state) {
            await event.response.body?.cancel();
            event.response = state.failure;
            return;
          }
          const { sessionID } = event;
          const watch = (account: Account, response: Response) => {
            if (response.status === 429)
              log(
                Effect.logWarning("anthropic: rate limited", {
                  providerID: account.id,
                  sessionID,
                  path: new URL(state.url).pathname,
                  status: response.status,
                  headers: Object.fromEntries(
                    [...response.headers].filter(
                      ([name]) => name === "retry-after" || name.startsWith("anthropic-ratelimit-"),
                    ),
                  ),
                }),
              );
            return response;
          };
          const send = async (account: Account, token: string) =>
            watch(
              account,
              await fetch(state.url, {
                method: "POST",
                headers: sign(state.base, token, state.model, account.excluded.get(state.model)),
                body: state.body,
                signal: event.request.signal,
              }),
            );
          const settle = async (account: Account, token: string, response: Response): Promise<Response | Failure> => {
            if (response.status === 401) {
              await response.body?.cancel();
              const next = await account.credentials.renew(token);
              if (typeof next === "string") return next;
              token = next.accessToken;
              response = await send(account, token);
            }
            if (response.status === 401) {
              await response.body?.cancel();
              return "login";
            }
            for (const beta of LONG_CONTEXT_BETAS) {
              const betas = account.excluded.get(state.model) ?? new Set<string>();
              if (betas.has(beta)) continue;
              if (response.status !== 400 && response.status !== 429) break;
              if (!isLongContextError(await response.clone().text())) break;
              account.excluded.set(state.model, betas.add(beta));
              await response.body?.cancel();
              response = await send(account, token);
            }
            return response;
          };

          const settled = await settle(state.account, state.token, watch(state.account, event.response));
          if (typeof settled === "string") {
            event.response = failure(state.account, settled);
            return;
          }
          const move = async (from: Account) => {
            for (const next of await open(state.model, from, sessionID)) {
              const found = await next.credentials.get();
              const result =
                typeof found === "string"
                  ? found
                  : await settle(next, found.accessToken, await send(next, found.accessToken));
              if (typeof result === "string") {
                log(
                  Effect.logWarning("anthropic: failover account unavailable", {
                    providerID: next.id,
                    failure: result,
                  }),
                );
                continue;
              }
              const until = await capped(result, next.id, state.model);
              if (until === undefined) {
                own.sticky.set(sessionID, next.id);
                log(Effect.logInfo("anthropic: failover", { from: from.id, to: next.id, sessionID }));
                return result;
              }
              next.until = until;
              await result.body?.cancel();
            }
            log(
              Effect.logWarning("anthropic: every Claude account is capped", {
                from: from.id,
                sessionID,
                until: new Date(from.until).toISOString(),
              }),
            );
          };

          let response = settled;
          const until = await capped(response, state.account.id, state.model);
          if (until !== undefined) {
            state.account.until = until;
            const original = new Response(await response.arrayBuffer(), {
              status: response.status,
              statusText: response.statusText,
              headers: response.headers,
            });
            response = (await move(state.account)) ?? original;
          }

          event.response = transformResponseStream(clear.watch(state, sessionID, response));
        },
        { providerID: own.id },
      );
    }
    return () => timers.forEach(clearInterval);
  },
});
