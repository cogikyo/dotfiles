import type { LanguageModelV3Message, LanguageModelV3Prompt } from "@ai-sdk/provider";

export type Meta = { session: string; owner: string; users: number; digest: string };

export type Plan = {
  known?: { session: string; users: number };
  system: string;
  next: string;
  full: string;
  meta: (session: string) => Meta | undefined;
};

const excerpt = 2000;

export function plan(prompt: LanguageModelV3Prompt, owner: string | undefined): Plan {
  const turns = prompt.filter((message) => message.role !== "system");
  const system = prompt
    .flatMap((message) => (message.role === "system" ? [message.content.trim()] : []))
    .filter(Boolean)
    .join("\n\n");
  const users = turns.flatMap((message) => (message.role === "user" ? [text(message)] : []));
  const last = turns.findLastIndex((message) => message.role === "assistant");
  const latest = turns
    .slice(last + 1)
    .flatMap((message) => (message.role === "user" ? [text(message)] : []))
    .join("\n\n");
  const full = last === -1 ? users.join("\n\n") : replay(turns.slice(0, last + 1), latest);
  const meta = (session: string) =>
    owner === undefined ? undefined : { session, owner, users: users.length, digest: digest(users) };
  const known = owner === undefined ? undefined : find(turns, owner, users);
  if (!known) return { system, next: full, full, meta };
  const next = users.slice(known.users).join("\n\n") || users.at(-1) || "";
  return { known, system, next, full, meta };
}

function find(turns: LanguageModelV3Message[], owner: string, users: string[]) {
  for (const message of turns.toReversed()) {
    if (message.role !== "assistant") continue;
    for (const part of message.content.toReversed()) {
      const meta = part.providerOptions?.gemini as Partial<Meta> | undefined;
      if (meta?.owner !== owner || typeof meta.session !== "string" || typeof meta.users !== "number") continue;
      if (meta.users > users.length || meta.digest !== digest(users.slice(0, meta.users))) return undefined;
      return { session: meta.session, users: meta.users };
    }
  }
  return undefined;
}

function digest(users: string[]) {
  return Bun.hash(users.join("\u0000")).toString(36);
}

function text(message: LanguageModelV3Message) {
  if (message.role === "system") return message.content;
  return message.content
    .flatMap((part) => (part.type === "text" ? [part.text] : []))
    .join("\n\n")
    .trim();
}

function replay(turns: LanguageModelV3Message[], latest: string) {
  const lines = turns.flatMap((message) => {
    if (message.role === "user") return [`[user]\n${text(message)}`];
    if (message.role === "system") return [];
    return message.content.flatMap((part) => {
      if (part.type === "text" && part.text.trim()) return [`[${message.role}]\n${part.text}`];
      if (part.type === "tool-call") return [`[tool call ${part.toolName}]\n${clip(JSON.stringify(part.input))}`];
      if (part.type === "tool-result") return [`[tool result ${part.toolName}]\n${clip(output(part.output))}`];
      return [];
    });
  });
  return [
    "The earlier part of this conversation is replayed below because its agent session could not be resumed.",
    ...lines,
    "[latest message]",
    latest,
  ].join("\n\n");
}

function output(value: { type: string; value?: unknown; reason?: string }) {
  if (typeof value.value === "string") return value.value;
  return JSON.stringify(value.value ?? value.reason ?? value.type);
}

function clip(value: string) {
  return value.length > excerpt ? `${value.slice(0, excerpt)}…` : value;
}
