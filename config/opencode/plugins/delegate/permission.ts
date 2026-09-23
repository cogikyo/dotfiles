import { record } from "../shared/record.ts";
import { type Client, string, unwrap } from "./sdk.ts";

export type Rule = {
  permission: string;
  pattern: string;
  action: "allow" | "ask" | "deny";
};

export type Execution = {
  unattended: boolean;
};

const UNATTENDED_FLOOR: Rule = { permission: "*", pattern: "*", action: "deny" }; // This catch-all denial is prepended to each unattended child permission envelope.

export async function deriveChildPermission(
  client: Client,
  parent: Record<string, unknown>,
  agent: { name: string; permission?: unknown },
  execution: Execution,
): Promise<Rule[]> {
  const config = await unwrap<Record<string, unknown>>(client.config.get({}), "read config");
  const unattended = execution.unattended;
  const agentConfig = record(record(config.agent)?.[agent.name]);
  if (!agentConfig)
    throw new Error(`delegate agent ${agent.name} is missing from config.agent; cannot determine declared permissions`);

  const parentRules = inheritableParentRules(normalizeRules(parent.permission), unattended);
  const inherited = parentRules.filter(
    (rule) => rule.permission === "external_directory" || (unattended && rule.action === "deny"),
  );
  const agentRules = normalizeRules(agent.permission);
  const declaredRules = normalizeRules(agentConfig.permission);
  const defaultRules = defaultAgentRules(agent.name, declaredRules);
  const childDenies: Rule[] = [
    ...(hasPermissionRule(declaredRules, "todowrite") ? [] : [deny("todowrite")]),
    ...(hasPermissionRule(declaredRules, "task") ? [] : [deny("task")]),
    deny("question"),
    ...primaryTools(config)
      .filter((tool) => !hasPermissionRule(declaredRules, tool))
      .map(deny),
  ];
  const composed = [...defaultRules, ...agentRules, ...childDenies, ...inherited];
  if (!unattended) return dedupeRules(composed);
  return [UNATTENDED_FLOOR, ...dedupeRules(composed.map(asBlocker))];
}

// Only the leading synthetic floor is removed; later parent rules remain eligible for inheritance.
function inheritableParentRules(rules: Rule[], unattended: boolean) {
  const synthetic = unattended && rules.length > 0 && isUnattendedFloor(rules[0]);
  return synthetic ? rules.slice(1) : rules;
}

function isUnattendedFloor(rule: Rule) {
  return (
    rule.permission === UNATTENDED_FLOOR.permission &&
    rule.pattern === UNATTENDED_FLOOR.pattern &&
    rule.action === UNATTENDED_FLOOR.action
  );
}

// Preserve rule order while converting `ask` rules to `deny`.
function asBlocker(rule: Rule): Rule {
  return rule.action === "ask" ? { ...rule, action: "deny" } : rule;
}

export function normalizeRules(value: unknown): Rule[] {
  if (Array.isArray(value)) return value.flatMap(parseRule);
  const root = record(value);
  if (!root) return [];

  return Object.entries(root).flatMap(([permission, entry]) => {
    if (isAction(entry)) return [{ permission, pattern: "*", action: entry }];
    const patterns = record(entry);
    if (!patterns) return [];
    return Object.entries(patterns).flatMap(([pattern, action]) =>
      isAction(action) ? [{ permission, pattern, action }] : [],
    );
  });
}

export function samePermissionRules(left: Rule[], right: Rule[]) {
  return (
    left.length === right.length &&
    left.every((rule, index) => {
      const candidate = right[index];
      return (
        rule.permission === candidate.permission &&
        rule.pattern === candidate.pattern &&
        rule.action === candidate.action
      );
    })
  );
}

function parseRule(value: unknown): Rule[] {
  const root = record(value);
  const permission = string(root?.permission);
  const pattern = string(root?.pattern);
  const action = root?.action;
  if (!permission || !pattern || !isAction(action)) return [];
  return [{ permission, pattern, action }];
}

function hasPermissionRule(rules: Rule[], permission: string) {
  return rules.some((rule) => rule.permission === permission);
}

function defaultAgentRules(agentName: string, explicitRules: Rule[]) {
  if (!agentName.startsWith("review/")) return [];

  const rules: Rule[] = [];
  for (const permission of ["read", "glob", "grep", "list", "bash", "webfetch", "websearch", "lsp"]) {
    if (!hasPermissionRule(explicitRules, permission)) {
      rules.push(allow(permission));
      if (permission === "grep") rules.push({ permission: "grep", pattern: "/", action: "deny" });
    }
  }
  for (const permission of ["edit", "task", "todowrite", "question"]) {
    if (!hasPermissionRule(explicitRules, permission)) rules.push(deny(permission));
  }
  return rules;
}

function primaryTools(config: Record<string, unknown>) {
  const experimental = record(config.experimental);
  return Array.isArray(experimental?.primary_tools)
    ? experimental.primary_tools.filter((item): item is string => typeof item === "string")
    : [];
}

export function deny(permission: string): Rule {
  return { permission, pattern: "*", action: "deny" };
}

function allow(permission: string): Rule {
  return { permission, pattern: "*", action: "allow" };
}

function dedupeRules(rules: Rule[]) {
  const seen = new Set<string>();
  return rules.filter((rule) => {
    const key = `${rule.permission}\0${rule.pattern}\0${rule.action}`;
    if (seen.has(key)) return false;
    seen.add(key);
    return true;
  });
}

function isAction(value: unknown): value is Rule["action"] {
  return value === "allow" || value === "ask" || value === "deny";
}
