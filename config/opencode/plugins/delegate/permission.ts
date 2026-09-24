import {
  type Agent,
  type Client,
  config as readConfig,
  type Permission,
  type Rule,
  type Session,
} from "../shared/opencode.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Child permissions                                                                             │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Child execution mode stored in delegate session metadata. */
export type Execution = {
  unattended: boolean;
};

const UNATTENDED_FLOOR: Rule = { permission: "*", pattern: "*", action: "deny" };

/** Builds ordered child permissions from agent rules, delegate denies, and inherited parent restrictions. */
// Unattended children turn asks into denies and start with a deny-all floor; `question` is always denied.
export async function deriveChildPermission(
  client: Client,
  parent: Session,
  agent: Agent,
  execution: Execution,
): Promise<Rule[]> {
  const config = await readConfig(client, { label: "delegate read config" });
  const unattended = execution.unattended;
  const agentConfig = config.agent?.[agent.name];
  if (!agentConfig)
    throw new Error(`delegate agent ${agent.name} is missing from config.agent; cannot determine declared permissions`);

  const parentRules = inheritableParentRules(parent.permission ?? [], unattended);
  const inherited = parentRules.filter(
    (rule) => rule.permission === "external_directory" || (unattended && rule.action === "deny"),
  );
  const declaredRules = configRules(agentConfig.permission);
  const defaultRules = defaultAgentRules(agent.name, declaredRules);
  const childDenies: Rule[] = [
    ...(hasPermissionRule(declaredRules, "todowrite") ? [] : [deny("todowrite")]),
    ...(hasPermissionRule(declaredRules, "task") ? [] : [deny("task")]),
    deny("question"),
    ...(config.experimental?.primary_tools ?? []).filter((tool) => !hasPermissionRule(declaredRules, tool)).map(deny),
  ];
  const composed = [...defaultRules, ...agent.permission, ...childDenies, ...inherited];
  if (!unattended) return dedupeRules(composed);
  return [UNATTENDED_FLOOR, ...dedupeRules(composed.map(asBlocker))];
}

/** Requires the same ordered permission envelope before a child can resume. */
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

/** Denies all patterns for a permission; `"*"` seals a child. */
export function deny(permission: string): Rule {
  return { permission, pattern: "*", action: "deny" };
}

// ├─ Rule assembly ───────────────────────────────────────────────────────────────────────────────┤

// Exclude the parent's synthetic deny-all floor so child-specific rules can still apply.
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

function asBlocker(rule: Rule): Rule {
  return rule.action === "ask" ? { ...rule, action: "deny" } : rule;
}

function configRules(permission: Permission | undefined): Rule[] {
  if (!permission || typeof permission === "string") return [];
  return Object.entries(permission).flatMap(([name, entry]) =>
    typeof entry === "string"
      ? [{ permission: name, pattern: "*", action: entry }]
      : Object.entries(entry).map(([pattern, action]) => ({ permission: name, pattern, action })),
  );
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
