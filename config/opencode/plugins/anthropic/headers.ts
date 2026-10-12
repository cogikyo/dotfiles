import { randomUUID } from "node:crypto";
import { getModelBetas } from "./claude-auth/betas.js";
import { config } from "./claude-auth/model-config.js";

const session = randomUUID();

const stainless = {
  "x-stainless-arch": process.arch,
  "x-stainless-lang": "js",
  "x-stainless-os": process.platform === "darwin" ? "MacOS" : process.platform,
  "x-stainless-package-version": "0.81.0",
  "x-stainless-retry-count": "0",
  "x-stainless-runtime": "node",
  "x-stainless-runtime-version": process.version,
  "x-stainless-timeout": "600",
};

export function sign(base: Headers, token: string, model: string, excluded = new Set<string>()) {
  const headers = new Headers(base);
  const version = process.env.ANTHROPIC_CLI_VERSION ?? config.ccVersion;
  const incoming = (headers.get("anthropic-beta") ?? "")
    .split(",")
    .map((beta) => beta.trim())
    .filter(Boolean);
  const betas = [...new Set([...getModelBetas(model, excluded), ...incoming])].filter((beta) => !excluded.has(beta));
  headers.set("authorization", `Bearer ${token}`);
  headers.set("anthropic-version", "2023-06-01");
  headers.set("anthropic-beta", betas.join(","));
  headers.set("anthropic-dangerous-direct-browser-access", "true");
  headers.set("x-app", "cli");
  headers.set("user-agent", process.env.ANTHROPIC_USER_AGENT ?? `claude-cli/${version} (external, sdk-cli)`);
  headers.set("x-client-request-id", randomUUID());
  headers.set("X-Claude-Code-Session-Id", session);
  for (const [key, value] of Object.entries(stainless)) if (!headers.has(key)) headers.set(key, value);
  headers.delete("x-api-key");
  headers.delete("content-length");
  return headers;
}
