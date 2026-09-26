import {
  type Agent,
  type Client,
  config as readConfig,
  type Permission,
  type Rule,
  type Session,
} from "../shared/opencode.ts";
import { armed } from "../shared/drive.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Child permissions                                                                             │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Child execution mode stored in delegate session metadata. */
export type Execution = {
  unattended: boolean;
};

export type Envelope = { basis: Rule[]; blocked: Rule[] };

const UNATTENDED_FLOOR: Rule = { permission: "*", pattern: "*", action: "deny" };

/** Builds ordered child permissions from agent rules, delegate denies, and inherited parent restrictions.
 * Unattended children start with a deny-all floor and, outside drive mode, turn asks into denies; `question` is always denied. */
export async function deriveChildPermission(
  client: Client,
  parent: Session,
  agent: Agent,
  execution: Execution,
): Promise<{ permission: Rule[]; envelope: Envelope }> {
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
  const basis = dedupeRules(composed);
  if (!unattended) return { permission: basis, envelope: { basis, blocked: basis } };
  const envelope = {
    basis: [UNATTENDED_FLOOR, ...basis],
    blocked: [UNATTENDED_FLOOR, ...dedupeRules(composed.map(asBlocker))],
  };
  const permission = (await armed(client, parent.id)) ? envelope.basis : envelope.blocked;
  return { permission, envelope };
}

export function sameEnvelope(left: Rule[], right: Rule[]) {
  return samePermissionRules(canonical(left), canonical(right));
}

function canonical(rules: Rule[]) {
  const runs: Rule[][] = [];
  for (const rule of rules) {
    const run = runs.at(-1);
    if (run && reorderable(run[0], rule)) run.push(rule);
    else runs.push([rule]);
  }
  return runs.flatMap((run) => run.toSorted((left, right) => left.pattern.localeCompare(right.pattern)));
}

function reorderable(left: Rule, right: Rule) {
  return (
    left.pattern !== "*" &&
    right.pattern !== "*" &&
    left.permission === right.permission &&
    left.action === right.action
  );
}

/** Requires the same ordered permission envelope before a child can resume. */
function samePermissionRules(left: Rule[], right: Rule[]) {
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
