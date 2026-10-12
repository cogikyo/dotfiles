import { Plugin } from "@opencode/plugin";
import { claudeAccounts } from "./accounts.ts";
import { createBetaState, isLongContextError, LONG_CONTEXT_BETAS } from "./claude-auth/betas.js";
import { createCredentials } from "./claude-auth/credentials.js";
import { fetchWithRetry } from "./claude-auth/http.js";
import { buildRequestHeaders } from "./claude-auth/index.js";
import { readCredentialsFile } from "./claude-auth/keychain.js";
import { SYSTEM_IDENTITY, transformBody, transformResponseStream } from "./claude-auth/transforms.js";

const disabledVariants: Record<string, string[]> = {
  "claude-opus-5-5": ["low", "medium"],
  "claude-sonnet-5-5": ["low", "medium", "max"],
};

const cacheControl = { type: "ephemeral", ttl: "1h" };

type Signed = { url: string; base: Headers; body: string | undefined; model: string; token: string };

type Pending = Signed | { failure: Response };

function apiError(status: number, type: string, message: string, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify({ type: "error", error: { type, message } }), {
    status,
    headers: { "content-type": "application/json", ...headers },
  });
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
    await ctx.provider.transform((providers) => {
      const source = providers.get("anthropic");
      for (const [id, ids] of Object.entries(disabledVariants)) {
        if (!source?.models.has(id)) continue;
        providers.models.update("anthropic", id, (model) => {
          model.variants = model.variants.filter((variant) => !ids.includes(variant.id));
        });
      }
    });

    for (const { id: providerID, configDir, loginCommand } of Object.values(claudeAccounts)) {
      const betas = createBetaState();
      const credentials = createCredentials({
        readAccounts: () => {
          const found = readCredentialsFile(configDir);
          return found ? [{ label: providerID, source: `file:${configDir}`, configDir, credentials: found }] : [];
        },
        fixed: true,
        resetBetas: betas.resetExcludedBetas,
      });
      const pending = new WeakMap<Request, Pending>();
      const unavailable = () =>
        credentials.getActiveRefreshFailureKind() === "transient"
          ? apiError(429, "rate_limit_error", "Claude token refresh is rate-limited; retry shortly.", {
              "retry-after": "5",
            })
          : apiError(
              401,
              "authentication_error",
              `${providerID}: Claude credentials are unavailable or expired. Run ${loginCommand}.`,
            );
      const sign = ({ url, base, token, model }: Signed) => {
        const excluded = betas.getExcludedBetas(model);
        const headers = buildRequestHeaders(url, { headers: base }, token, model, excluded);
        headers.set(
          "anthropic-beta",
          (headers.get("anthropic-beta") ?? "")
            .split(",")
            .filter((beta) => beta && !excluded.has(beta))
            .join(","),
        );
        headers.delete("content-length");
        return headers;
      };

      await ctx.session.hook(
        "http.request",
        async (event) => {
          const request = event.request;
          const found =
            (await credentials.getCachedCredentials()) ??
            (await credentials.getCredentialsWithBackoff({ signal: request.signal }));
          const base = new Headers(request.headers);
          base.delete("x-api-key");
          if (!found) {
            event.request = new Request(request, { headers: base });
            pending.set(event.request, { failure: unavailable() });
            return;
          }
          const url = new URL(request.url);
          if (url.pathname === "/v1/messages" && !url.searchParams.has("beta")) url.searchParams.set("beta", "true");
          const text = request.body ? await request.clone().text() : undefined;
          const shaped = text ? shape(text) : { model: "unknown", body: undefined };
          const state = { url: url.href, base, body: shaped.body, model: shaped.model, token: found.accessToken };
          event.request = new Request(state.url, {
            method: request.method,
            headers: sign(state),
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
              headers: sign({ ...state, token }),
              body: state.body,
              signal: event.request.signal,
            });
          };

          for (let attempt = 0; response.status === 401 && attempt < 2; attempt++) {
            let next = credentials.reloadCredentialsFromSource();
            if (!next || next.accessToken === token) next = await credentials.forceRefreshActiveAccount();
            if (!next || next.accessToken === token) break;
            token = next.accessToken;
            response = await resend();
          }
          if (response.status === 401) {
            await response.body?.cancel();
            event.response = unavailable();
            return;
          }

          for (let attempt = 0; attempt < LONG_CONTEXT_BETAS.length; attempt++) {
            if (response.status !== 400 && response.status !== 429) break;
            if (!isLongContextError(await response.clone().text())) break;
            const beta = betas.getNextBetaToExclude(state.model);
            if (!beta) break;
            betas.addExcludedBeta(state.model, beta);
            response = await resend();
          }

          event.response = transformResponseStream(response);
        },
        { providerID },
      );
    }
  },
});
