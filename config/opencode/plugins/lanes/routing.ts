import { YAML } from "bun";
import path from "node:path";

export type Ref = { providerID: string; id: string; variant?: string };

export const routingPath = path.join(import.meta.dirname, "..", "..", "ROUTING.md");

export function parseRouting(text: string) {
  const match = /^---\r?\n([\s\S]*?)\r?\n---/u.exec(text);
  if (!match) throw new Error("no YAML frontmatter");
  const data = YAML.parse(match[1]) as { routes?: unknown; fallbacks?: unknown };
  return { routes: parseRoutes(data.routes), fallbacks: parseRoutes(data.fallbacks) };
}

export function routeFor(routes: Record<string, Ref>, agent: string) {
  if (Object.hasOwn(routes, agent)) return routes[agent];
  const [key] = Object.keys(routes)
    .filter((pattern) => pattern.includes("*") && glob(pattern).test(agent))
    .toSorted((a, b) => b.length - a.length);
  return key === undefined ? undefined : routes[key];
}

function parseRoute(key: string, value: unknown): Ref {
  const match = typeof value === "string" ? /^([^\s/]+)\/(\S+?)(?:\s+(\S+))?$/u.exec(value.trim()) : null;
  if (!match) throw new Error(`route "${key}" must be "provider/model [effort]"`);
  return { providerID: match[1], id: match[2], variant: match[3] };
}

function parseRoutes(value: unknown) {
  return Object.fromEntries(Object.entries(value ?? {}).map(([key, route]) => [key, parseRoute(key, route)]));
}

function glob(pattern: string) {
  return new RegExp(
    `^${pattern
      .split("*")
      .map((part) => RegExp.escape(part))
      .join(".*")}$`,
    "u",
  );
}
