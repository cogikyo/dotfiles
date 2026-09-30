import { YAML } from "bun";
import { join } from "node:path";
import { z } from "zod";
import { claudeAccounts } from "../anthropic/accounts.ts";
import { errorMessage } from "../shared/error.ts";
import { readText } from "../shared/file.ts";
import { configRoot } from "../shared/root.ts";
import { WEEKLY } from "../usage/anthropic.ts";
import { type ModelRef, parseModel } from "./args.ts";
import { readUsage, type Usage } from "./policy.ts";

export const ROUTING_PATH = join(configRoot, "ROUTING.md");

const POOL = "claude";
const FRONTMATTER = /^---\r?\n([\s\S]*?)\r?\n---(?:\r?\n|$)/u;

type Route = { model: ModelRef; effort?: string };

type Resolved = Route & { notes: string[] };

const Route = z
  .string()
  .trim()
  .regex(/^[^\s/]+\/\S+(?:\s+\S+)?$/u, 'expected "provider/model-id [effort]"')
  .transform((value): Route => {
    const [model, effort] = value.split(/\s+/u);
    return { model: parseModel(model), effort };
  });

const Routing = z.strictObject({
  routes: z.record(z.string(), Route),
  fallbacks: z
    .record(
      z.string(),
      Route.refine((route) => route.model.providerID !== POOL, `a fallback cannot name the ${POOL} pool`),
    )
    .refine(
      (fallbacks) => Object.keys(fallbacks).every((key) => key.startsWith(`${POOL}/`)),
      `fallback keys must be ${POOL}/<model-id>`,
    )
    .default({}),
});

type Routing = z.infer<typeof Routing>;

function parseRouting(text: string): Routing {
  const match = FRONTMATTER.exec(text);
  if (!match) throw new Error("ROUTING.md has no YAML frontmatter");
  let data: unknown;
  try {
    data = YAML.parse(match[1]);
  } catch (error) {
    throw new Error(`ROUTING.md frontmatter is not valid YAML: ${errorMessage(error)}`, { cause: error });
  }
  const result = Routing.safeParse(data);
  if (!result.success) {
    throw new Error(`ROUTING.md frontmatter is invalid:\n${z.prettifyError(result.error)}`, { cause: result.error });
  }
  return result.data;
}

async function readRouting(path: string) {
  const text = await readText(path);
  if (text === undefined) throw new Error(`ROUTING.md not found at ${path}`);
  return text;
}

export async function routingPrompt(path = ROUTING_PATH) {
  const header = `Instructions from: ${path}`;
  let text: string | undefined;
  try {
    text = await readRouting(path);
    parseRouting(text);
    return `${header}\n${text}`;
  } catch (error) {
    const failure = `ROUTING ERROR: ${errorMessage(error)}\nTask calls without model fail until the user fixes ROUTING.md; pass model explicitly meanwhile.`;
    return [header, failure, text].filter(Boolean).join("\n");
  }
}

export async function resolveRoute(
  agent: string,
  requested: { model?: string; effort?: string },
  path = ROUTING_PATH,
): Promise<Resolved | undefined> {
  const explicit = requested.model ? parseModel(requested.model) : undefined;
  if (explicit && explicit.providerID !== POOL) return { model: explicit, effort: requested.effort, notes: [] };

  const routing = parseRouting(await readRouting(path));
  const route = explicit ? { model: explicit } : routeFor(routing.routes, agent);
  if (!route) return undefined;
  const effort = requested.effort ?? route.effort;
  if (route.model.providerID !== POOL) return { model: route.model, effort, notes: [] };
  return pickAccount(route.model.modelID, effort, routing.fallbacks[`${POOL}/${route.model.modelID}`]);
}

function routeFor(routes: Routing["routes"], agent: string) {
  if (Object.hasOwn(routes, agent)) return routes[agent];
  const [pattern] = Object.keys(routes)
    .filter((key) => wildcard(key).test(agent))
    .toSorted((a, b) => b.length - a.length);
  return pattern === undefined ? undefined : routes[pattern];
}

function wildcard(pattern: string) {
  const source = pattern
    .split("*")
    .map((part) => RegExp.escape(part))
    .join(".*");
  return new RegExp(`^${source}$`, "u");
}

type Account = { id: string; usage: Usage };

async function pickAccount(modelID: string, effort: string | undefined, fallback: Route | undefined) {
  const accounts = await Promise.all(
    Object.values(claudeAccounts).map(async ({ id }): Promise<Account> => ({
      id,
      usage: await readUsage({ providerID: id, modelID }),
    })),
  );
  const open = accounts.filter(({ usage }) => !usage.capped.length);
  if (!open.length && fallback) {
    const target = `${fallback.model.providerID}/${fallback.model.modelID}`;
    const dropped = effort && effort !== fallback.effort ? ` without effort ${effort}` : "";
    return { ...fallback, notes: [`delegate routing: both Claude accounts are capped; using ${target}${dropped}`] };
  }
  const [account] = open.length
    ? open.toSorted((a, b) => Number(unknown(a)) - Number(unknown(b)) || weeklyReset(a) - weeklyReset(b))
    : accounts.toSorted((a, b) => cappedUntil(a) - cappedUntil(b));
  return { model: { providerID: account.id, modelID }, effort, notes: [] };
}

function unknown({ usage }: Account) {
  return !usage.windows.length || usage.windows.some((window) => window.postReset || window.usedPercent === undefined);
}

function weeklyReset({ usage }: Account) {
  const weekly = usage.windows.find((window) => window.label === WEEKLY && !window.postReset);
  return weekly?.resetAt ? Date.parse(weekly.resetAt) : Infinity;
}

function cappedUntil({ usage }: Account) {
  return Math.max(...usage.capped.map((window) => (window.resetAt ? Date.parse(window.resetAt) : Infinity)));
}
