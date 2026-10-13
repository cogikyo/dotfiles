import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { Plugin } from "@opencode/plugin";
import { Effect, Fiber } from "effect";
import { claudeAccounts } from "./accounts.ts";
import { isLongContextError, LONG_CONTEXT_BETAS } from "./claude-auth/betas.js";
import { fetchWithRetry } from "./claude-auth/http.js";
import { config } from "./claude-auth/model-config.js";
import { SYSTEM_IDENTITY, transformBody, transformResponseStream } from "./claude-auth/transforms.js";
import { Credentials, type Failure } from "./credentials.ts";
import { sign } from "./headers.ts";

const disabledVariants: Record<string, string[]> = {
  "claude-opus-5-5": ["low", "medium"],
  "claude-sonnet-5-5": ["low", "medium", "max"],
};

const cacheControl = { type: "ephemeral", ttl: "1h" };
const renewEvery = 5 * 60_000;

type Signed = { url: string; base: Headers; body: string | undefined; model: string; token: string };

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
    await ctx.provider.transform((providers) => {
      const source = providers.get("anthropic");
      for (const [id, ids] of Object.entries(disabledVariants)) {
        if (!source?.models.has(id)) continue;
        providers.models.update("anthropic", id, (model) => {
          model.variants = model.variants.filter((variant) => !ids.includes(String(variant.id)));
        });
      }
    });

    const timers: ReturnType<typeof setInterval>[] = [];
    for (const { id: providerID, configDir, loginCommand } of Object.values(claudeAccounts)) {
      const credentials = new Credentials(configDir, log);
      const renewEarly = () =>
        credentials
          .renewEarly()
          .catch((error: unknown) =>
            log(Effect.logWarning("anthropic: early renewal failed", { error: String(error) })),
          );
      void renewEarly();
      timers.push(setInterval(() => void renewEarly(), renewEvery));
      const excluded = new Map<string, Set<string>>();
      const pending = new WeakMap<Request, Pending>();
      const failure = (kind: Failure) =>
        kind === "transient"
          ? apiError(429, "rate_limit_error", "Claude token refresh is rate-limited; retry shortly.", {
              "retry-after": "5",
            })
          : apiError(
              401,
              "authentication_error",
              `${providerID}: Claude credentials are unavailable or expired. Run ${loginCommand}.`,
            );

      await ctx.session.hook(
        "http.request",
        async (event) => {
          const request = event.request;
          const found = await credentials.get();
          const base = new Headers(request.headers);
          base.delete("x-api-key");
          if (typeof found === "string") {
            event.request = new Request(request, { headers: base });
            pending.set(event.request, { failure: failure(found) });
            return;
          }
          const url = new URL(request.url);
          if (url.pathname === "/v1/messages" && !url.searchParams.has("beta")) url.searchParams.set("beta", "true");
          const text = request.body ? await request.clone().text() : undefined;
          const shaped = text ? shape(text) : { model: "unknown", body: undefined };
          const state = { url: url.href, base, body: shaped.body, model: shaped.model, token: found.accessToken };
          event.request = new Request(state.url, {
            method: request.method,
            headers: sign(base, state.token, state.model, excluded.get(state.model)),
            body: state.body,
            signal: request.signal,
          });
          pending.set(event.request, state);
        },
        { providerID },
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
          let response = event.response;
          let token = state.token;
          const resend = async () => {
            await response.body?.cancel();
            return fetchWithRetry(state.url, {
              method: "POST",
              headers: sign(state.base, token, state.model, excluded.get(state.model)),
              body: state.body,
              signal: event.request.signal,
            });
          };

          if (response.status === 401) {
            const next = await credentials.renew(token);
            if (typeof next === "string") {
              await response.body?.cancel();
              event.response = failure(next);
              return;
            }
            token = next.accessToken;
            response = await resend();
          }
          if (response.status === 401) {
            await response.body?.cancel();
            event.response = failure("login");
            return;
          }

          for (const beta of LONG_CONTEXT_BETAS) {
            const betas = excluded.get(state.model) ?? new Set<string>();
            if (betas.has(beta)) continue;
            if (response.status !== 400 && response.status !== 429) break;
            if (!isLongContextError(await response.clone().text())) break;
            excluded.set(state.model, betas.add(beta));
            response = await resend();
          }

          event.response = transformResponseStream(response);
        },
        { providerID },
      );
    }
    return () => timers.forEach(clearInterval);
  },
});
