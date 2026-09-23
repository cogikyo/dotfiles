import { record } from "../shared/record.ts";
import { deny, type Rule } from "./permission.ts";

export type Closer = "operator" | "collab";
export type Closed = { by: Closer; at: number };

type Marked = { metadata?: unknown };

export function sessionClosed(session: Marked): Closed | undefined {
  const closed = record(record(record(session.metadata)?.delegate)?.closed);
  const by = closed?.by;
  const at = closed?.at;
  if (by !== "operator" && by !== "collab") return undefined;
  if (typeof at !== "number" || !Number.isFinite(at)) return undefined;
  return { by, at };
}

export function closeBody(session: Marked, by: Closer): { metadata: Record<string, unknown>; permission: Rule[] } {
  const metadata = record(session.metadata) ?? {};
  const delegate = record(metadata.delegate) ?? {};
  const closed: Closed = { by, at: Date.now() };
  return {
    metadata: { ...metadata, delegate: { ...delegate, closed } },
    permission: [deny("*")],
  };
}
