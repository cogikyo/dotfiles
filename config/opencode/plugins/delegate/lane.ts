import type { SessionUpdateData } from "@opencode-ai/sdk/v2";
import { record } from "../shared/record.ts";
import { closeBody, sessionClosed } from "./closed.ts";
import { type Execution, normalizeRules, type Rule, samePermissionRules } from "./permission.ts";
import { type Client, finite, string, unwrap } from "./sdk.ts";

export async function readChildTaskStatus(client: Client, parentSessionID: string, signal: AbortSignal) {
  const [rawChildren, statuses] = await Promise.all([
    unwrap<unknown[]>(
      client.session.children({ path: { id: parentSessionID }, signal }),
      `list children of session ${parentSessionID}`,
    ),
    unwrap<Record<string, unknown>>(
      client.session.status({ signal }),
      `read child statuses for session ${parentSessionID}`,
    ),
  ]);

  const children = rawChildren
    .map(record)
    .filter((child): child is Record<string, unknown> => !!string(child?.id))
    .toSorted((left, right) => sessionUpdated(right) - sessionUpdated(left));

  if (!children.length) return `No direct task children found for session ${parentSessionID}.`;

  const lines = [`Direct task children for session ${parentSessionID}, newest first:`];
  for (const child of children) {
    const id = string(child.id)!;
    const closed = sessionClosed(child);
    const status = closed ? "closed" : childStatus(record(statuses[id]));
    const limit = sessionContextLimit(child);
    lines.push(
      "",
      `child_session_id: ${id}`,
      ...(sessionLane(child) ? [`lane: ${sessionLane(child)}`] : []),
      `status: ${status}`,
      ...(closed ? [`closed_by: ${closed.by}`] : []),
      ...(limit ? [`context_limit: ${limit.limit}${limit.tokens === undefined ? "" : ` tokens=${limit.tokens}`}`] : []),
      `agent: ${sessionAgent(child) ?? "unknown"}`,
      `title: ${singleLine(string(child.title) ?? "untitled")}`,
      `updated: ${new Date(sessionUpdated(child)).toISOString()}`,
    );
  }
  lines.push(
    "",
    "Only named lanes resume; context-limited and closed lanes create a fresh child on the next call, and a closed lane name may take a different agent.",
  );
  return lines.join("\n");
}

export async function laneChild(client: Client, parentSessionID: string, lane: string, signal: AbortSignal) {
  const children = await unwrap<unknown[]>(
    client.session.children({ path: { id: parentSessionID }, signal }),
    `list lanes for ${parentSessionID}`,
  );
  return children
    .map(record)
    .filter((child): child is Record<string, unknown> => !!child && sessionLane(child) === lane)
    .toSorted((left, right) => sessionCreated(right) - sessionCreated(left))[0];
}

export async function closeLane(client: Client, parentSessionID: string, lane: string, signal: AbortSignal) {
  const child = await laneChild(client, parentSessionID, lane, signal);
  if (!child) return "skipped (no such lane)";
  if (sessionClosed(child)) return "skipped (already closed)";
  const id = string(child.id);
  if (!id) throw new Error(`delegate lane ${lane} child has no id`);
  const statuses = await unwrap<Record<string, unknown>>(
    client.session.status({ signal }),
    `read lane ${lane} status before close`,
  );
  const status = childStatus(record(statuses[id]));
  if (status !== "idle") return `skipped (${status})`;
  const body: NonNullable<SessionUpdateData["body"]> = closeBody(child, "collab");
  await unwrap(client.session.update({ path: { id }, body, signal }), `close lane ${lane} child ${id}`);
  return "closed";
}

export async function assertLaneOpen(client: Client, sessionID: string, lane: string, signal: AbortSignal) {
  const session = await unwrap<Record<string, unknown>>(
    client.session.get({ path: { id: sessionID }, signal }),
    `re-read lane ${lane} child ${sessionID} before prompt`,
  );
  if (sessionClosed(session)) {
    throw new Error(`delegate lane ${lane} was closed; call task again to start a fresh child`);
  }
}

export async function readExistingChild(input: {
  client: Client;
  session: Record<string, unknown>;
  permission: Rule[];
  execution: Execution;
  signal: AbortSignal;
}) {
  const { client, session, permission, execution, signal } = input;
  const id = string(session.id);
  if (!id) throw new Error("delegate lane child did not return an id");
  const statuses = await unwrap<Record<string, unknown>>(
    client.session.status({ signal }),
    `read child session ${id} status before resume`,
  );
  const status = record(statuses[id]);
  if (status && status.type !== "idle") {
    throw new Error(`delegate lane ${sessionLane(session)} is ${String(status.type)}; wait until it is idle`);
  }
  if (!samePermissionRules(normalizeRules(session.permission), permission)) {
    throw new Error(`delegate resumed child permission envelope no longer matches; re-brief a fresh child instead`);
  }
  if (!sameExecution(sessionExecution(session), execution)) {
    throw new Error(`delegate resumed child execution contract no longer matches; re-brief a fresh child instead`);
  }
  return { id };
}

export function sessionExecution(session: Record<string, unknown>): Execution {
  const delegate = record(record(session.metadata)?.delegate);
  if (!delegate) return { unattended: false };

  const execution: Execution = { unattended: false };
  if (Object.hasOwn(delegate, "unattended")) {
    if (typeof delegate.unattended !== "boolean") {
      throw new Error("delegate session metadata.delegate.unattended must be a boolean");
    }
    execution.unattended = delegate.unattended;
  }
  return execution;
}

export function sameExecution(left: Execution, right: Execution) {
  return left.unattended === right.unattended;
}

export function sessionLane(session: Record<string, unknown>) {
  return string(record(record(session.metadata)?.delegate)?.lane);
}

export function sessionAgent(session: Record<string, unknown>) {
  return string(session.agent) ?? string(record(session.agent)?.name);
}

function sessionUpdated(session: Record<string, unknown>) {
  const updated = record(session.time)?.updated;
  return typeof updated === "number" && Number.isFinite(updated) ? updated : 0;
}

function sessionCreated(session: Record<string, unknown>) {
  const created = record(session.time)?.created;
  return typeof created === "number" && Number.isFinite(created) ? created : 0;
}

function childStatus(status: Record<string, unknown> | undefined) {
  const type = string(status?.type) ?? "idle";
  if (type !== "retry") return type;

  const attempt = typeof status?.attempt === "number" ? ` attempt=${status.attempt}` : "";
  const next = typeof status?.next === "number" ? ` next=${new Date(status.next).toISOString()}` : "";
  const message = string(status?.message);
  return [`retry${attempt}${next}`, message].filter(Boolean).join(" ");
}

function singleLine(value: string) {
  return value.replace(/\s+/gu, " ").trim();
}

export function sessionParentID(session: Record<string, unknown>) {
  return string(session.parentID) ?? string(session.parentId) ?? string(record(session.parent)?.id);
}

export function sessionContextLimit(session: Record<string, unknown>) {
  const context = record(record(record(session.metadata)?.delegate)?.context);
  const limit = string(context?.limit);
  if (limit !== "hard" && limit !== "compaction") return undefined;
  return { limit, tokens: finite(context?.tokens) };
}
