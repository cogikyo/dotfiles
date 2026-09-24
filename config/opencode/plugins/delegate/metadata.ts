import { z } from "zod";

/** Parses delegate session metadata, dropping invalid optional markers but rejecting invalid `unattended`. */
export const Delegate = z.looseObject({
  lane: z.string().min(1).optional().catch(undefined),
  unattended: z.boolean().optional(),
  closed: z
    .object({ by: z.enum(["operator", "collab"]), at: z.number() })
    .optional()
    .catch(undefined),
  context: z
    .object({ limit: z.enum(["hard", "compaction"]), tokens: z.number().optional().catch(undefined) })
    .optional()
    .catch(undefined),
});

export type Delegate = z.infer<typeof Delegate>;

type Marked = { id: string; metadata?: Record<string, unknown> };

/** Reads delegate metadata, returning an empty object when absent and rejecting invalid stored values. */
export function delegate(session: Marked): Delegate {
  const result = Delegate.optional().safeParse(session.metadata?.delegate);
  if (!result.success) {
    throw new Error(`delegate session ${session.id} metadata.delegate is malformed: ${z.prettifyError(result.error)}`);
  }
  return result.data ?? {};
}
