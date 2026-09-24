import type { Rule } from "../shared/opencode.ts";
import type { Delegate } from "./metadata.ts";
import { deny } from "./permission.ts";

export type Closer = NonNullable<Delegate["closed"]>["by"];

type Closable = { metadata?: Record<string, unknown>; delegate: Delegate };

/** Builds a closed-lane update that preserves metadata and denies all child permissions. */
export function closeBody(session: Closable, by: Closer): { metadata: Record<string, unknown>; permission: Rule[] } {
  return {
    metadata: { ...session.metadata, delegate: { ...session.delegate, closed: { by, at: Date.now() } } },
    permission: [deny("*")],
  };
}
