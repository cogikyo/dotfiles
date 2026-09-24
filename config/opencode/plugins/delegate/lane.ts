import type { SessionUpdateData } from "@opencode-ai/sdk/v2";
import {
  children,
  type Client,
  type Rule,
  type Session,
  session,
  type Status,
  statuses,
  unwrap,
} from "../shared/opencode.ts";
import { closeBody } from "./closed.ts";
import { type Delegate, delegate } from "./metadata.ts";
import { type Execution, samePermissionRules } from "./permission.ts";

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Lanes                                                                                         │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

export type Child = Session & { delegate: Delegate };

// ├─ Status ──────────────────────────────────────────────────────────────────────────────────────┤

/** Lists direct task children newest first, treating closed children as closed and missing statuses as idle. */
export async function readChildTaskStatus(client: Client, parentSessionID: string, signal: AbortSignal) {
  const [found, live] = await Promise.all([
    readChildren(client, parentSessionID, `delegate list children of session ${parentSessionID}`, signal),
    statuses(client, { label: `delegate read child statuses for session ${parentSessionID}`, signal }),
  ]);

  if (!found.length) return `No direct task children found for session ${parentSessionID}.`;

  const lines = [`Direct task children for session ${parentSessionID}, newest first:`];
  for (const child of found.toSorted((left, right) => right.time.updated - left.time.updated)) {
    const { lane, closed, context } = child.delegate;
    lines.push(
      "",
      `child_session_id: ${child.id}`,
      ...(lane ? [`lane: ${lane}`] : []),
      `status: ${closed ? "closed" : childStatus(live[child.id])}`,
      ...(closed ? [`closed_by: ${closed.by}`] : []),
      ...(context
        ? [`context_limit: ${context.limit}${context.tokens === undefined ? "" : ` tokens=${context.tokens}`}`]
        : []),
      `agent: ${child.agent ?? "unknown"}`,
      `title: ${singleLine(child.title || "untitled")}`,
      `updated: ${new Date(child.time.updated).toISOString()}`,
    );
  }
  lines.push(
    "",
    "Only named lanes resume; context-limited and closed lanes create a fresh child on the next call, and a closed lane name may take a different agent.",
  );
  return lines.join("\n");
}

// ├─ Lookup and close ────────────────────────────────────────────────────────────────────────────┤

/** Finds the newest direct child for a lane, including closed or context-limited children. */
export async function laneChild(client: Client, parentSessionID: string, lane: string, signal: AbortSignal) {
  const found = await readChildren(client, parentSessionID, `delegate list lanes for ${parentSessionID}`, signal);
  return found
    .filter((child) => child.delegate.lane === lane)
    .toSorted((left, right) => right.time.created - left.time.created)[0];
}

/** Closes an idle lane, or reports why it was skipped; a missing live status counts as idle. */
export async function closeLane(client: Client, parentSessionID: string, lane: string, signal: AbortSignal) {
  const child = await laneChild(client, parentSessionID, lane, signal);
  if (!child) return "skipped (no such lane)";
  if (child.delegate.closed) return "skipped (already closed)";
  const live = await statuses(client, { label: `delegate read lane ${lane} status before close`, signal });
  const status = childStatus(live[child.id]);
  if (status !== "idle") return `skipped (${status})`;
  const body: NonNullable<SessionUpdateData["body"]> = closeBody(child, "collab");
  await unwrap(
    client.session.update({ path: { id: child.id }, body, signal }),
    `delegate close lane ${lane} child ${child.id}`,
  );
  return "closed";
}

/** Rejects a lane closed between resume selection and the next prompt. */
export async function assertLaneOpen(client: Client, sessionID: string, lane: string, signal: AbortSignal) {
  const current = await session(client, sessionID, {
    label: `delegate re-read lane ${lane} child ${sessionID} before prompt`,
    signal,
  });
  if (delegate(current).closed) {
    throw new Error(`delegate lane ${lane} was closed; call task again to start a fresh child`);
  }
}

// ├─ Resume ──────────────────────────────────────────────────────────────────────────────────────┤

/** Resumes only an idle child whose ordered permissions and execution mode still match. */
export async function readExistingChild(input: {
  client: Client;
  child: Child;
  permission: Rule[];
  execution: Execution;
  signal: AbortSignal;
}) {
  const { client, child, permission, execution, signal } = input;
  const live = await statuses(client, {
    label: `delegate read child session ${child.id} status before resume`,
    signal,
  });
  const status: Status | undefined = live[child.id];
  if (status && status.type !== "idle") {
    throw new Error(`delegate lane ${child.delegate.lane} is ${status.type}; wait until it is idle`);
  }
  if (!samePermissionRules(child.permission ?? [], permission)) {
    throw new Error(`delegate resumed child permission envelope no longer matches; re-brief a fresh child instead`);
  }
  if (!sameExecution(sessionExecution(child.delegate), execution)) {
    throw new Error(`delegate resumed child execution contract no longer matches; re-brief a fresh child instead`);
  }
  return { id: child.id };
}

/** Treats a missing stored unattended flag as false. */
export function sessionExecution(meta: Delegate): Execution {
  return { unattended: meta.unattended ?? false };
}

export function sameExecution(left: Execution, right: Execution) {
  return left.unattended === right.unattended;
}

// ├─ Child reads ─────────────────────────────────────────────────────────────────────────────────┤

async function readChildren(client: Client, parentSessionID: string, label: string, signal: AbortSignal) {
  const found = await children(client, parentSessionID, { label, signal });
  return found.map((child): Child => Object.assign(child, { delegate: delegate(child) }));
}

function childStatus(status: Status | undefined) {
  if (!status) return "idle";
  if (status.type !== "retry") return status.type;
  const retry = `retry attempt=${status.attempt} next=${new Date(status.next).toISOString()}`;
  return [retry, status.message].filter(Boolean).join(" ");
}

function singleLine(value: string) {
  return value.replace(/\s+/gu, " ").trim();
}
