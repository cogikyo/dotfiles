import type { Plugin } from "@opencode/plugin/tui";
import { threshold } from "../compact/limits.ts";
import { pressure } from "../statusline/palette.ts";

type Context = Plugin.Context;
type Session = ReturnType<Context["data"]["session"]["list"]>[number];
type Message = ReturnType<Context["data"]["session"]["message"]["list"]>[number];
type Assistant = Extract<Message, { type: "assistant" }>;
type Measured = Assistant & { tokens: NonNullable<Assistant["tokens"]> };
type Model = NonNullable<ReturnType<Context["data"]["location"]["model"]["list"]>>[number];

export type Usage = {
  agent: string;
  model: string;
  variant?: string;
  limit?: Model["limit"];
  tokens: number;
  spent: string;
};

const icons: Partial<Record<string, string>> = {
  build: "\u{f1887}",
  review: "\u{f0208}",
  verify: "\u{f0565}",
  scout: "\u{f018b}",
};

const scopes: Partial<Record<string, string>> = {
  general: "\u{265e}",
  owner: "\u{f085a}",
  patch: "\u{f0daf}",
  scribe: "\u{f0d13}",
  git: "\u{f062c}",
  debug: "\u{f00e4}",
  architect: "\u{f04aa}",
  simplify: "\u{f0190}",
  critic: "\u{f029b}",
  copy: "\u{f0284}",
  entropy: "\u{f0f38}",
  design: "\u{f03d8}",
  security: "\u{f033e}",
  profile: "\u{f04c5}",
  context: "\u{f034d}",
  code: "\u{f0174}",
  library: "\u{f0331}",
  dirty: "\u{f0dc8}",
  session: "\u{f02da}",
  web: "\u{f059f}",
  source: "\u{f0169}",
  test: "\u{f0093}",
  browser: "\u{f0cfd}",
};

const families: [RegExp, string][] = [
  [/^opus/, "opus"],
  [/^sonnet/, "sonnet"],
  [/sol/, "sol"],
  [/luna/, "luna"],
  [/astra/, "astra"],
  [/fable/, "fable"],
  [/grok/, "grok"],
  [/deepseek-v4.*-flash/, "v4 Flash"],
];

export function glyphs(agent: string) {
  const [role, ...rest] = agent.split("/");
  return { role: icons[role], scope: rest.length ? scopes[rest.join("/")] : undefined };
}

function label(agent: string) {
  const [, ...rest] = agent.split("/");
  const { role, scope } = glyphs(agent);
  if (!role || !rest.length) return agent;
  return `${role} ${scope ?? rest.join("/")}`;
}

export function usage(context: Context, session: Session): Usage {
  const last = latest(context.data.session.message.list(session.id));
  const used = last?.tokens;
  const tokens = used ? used.input + used.output + used.reasoning + used.cache.read + used.cache.write : 0;
  const model =
    last &&
    context.data.location.model
      .list(session.location)
      ?.find((entry) => entry.providerID === last.model.providerID && entry.id === last.model.id);
  return {
    agent: label(session.agent ?? "unknown"),
    model: family(last?.model.id ?? session.model?.id ?? "unknown"),
    variant: last?.model.variant ?? session.model?.variant,
    limit: model?.limit,
    tokens,
    spent: used ? count(tokens) : "",
  };
}

export function strain({ limit, tokens }: Usage, buffer: number | undefined) {
  const stop = limit ? threshold(limit, buffer, false) : undefined;
  return stop ? pressure((tokens / stop) * 100) : undefined;
}

function latest(messages: readonly Message[]) {
  const compaction = messages.findLastIndex(
    (message) => message.type === "compaction" && message.status === "completed",
  );
  return messages.findLast(
    (message, at): message is Measured =>
      message.type === "assistant" && message.tokens !== undefined && at > compaction,
  );
}

export function family(model: string) {
  const bare = model.replace(/^.*\//, "").replace(/^claude-/, "");
  return families.find(([pattern]) => pattern.test(bare))?.[1] ?? bare;
}

function count(tokens: number) {
  if (tokens >= 999_500) return `${(tokens / 1_000_000).toFixed(1)}M`;
  return tokens < 1_000 ? String(tokens) : `${Math.round(tokens / 1_000)}K`;
}
