import type { Plugin } from "@opencode/plugin";
import type { Step } from "./parse.ts";
import type { Flow } from "./state.ts";

export type Phase = "done" | "skipped" | "failed" | "running" | "returned" | "ready" | "waiting";

export type Lane = "running" | "idle";

export type Child = { lane: Lane; name?: string };

type Info = Awaited<ReturnType<Plugin.Context["session"]["get"]>>;

export type Progress = Flow & { phases: Readonly<Record<number, Phase>>; frame: string };

export const symbols: Record<Phase, string> = {
  done: "✓",
  skipped: "–",
  failed: "✗",
  running: "◐",
  returned: "↩",
  ready: "▸",
  waiting: "·",
};

export function phases(
  flow: Flow,
  steps: readonly Step[],
  ready: readonly number[],
  lane: (sessionID: string) => Lane | undefined,
): Record<number, Phase> {
  const seen = new Map<number, Lane>();
  for (const [child, n] of Object.entries(flow.links)) {
    const state = lane(child);
    if (state && seen.get(n) !== "running") seen.set(n, state);
  }
  return Object.fromEntries(
    steps.map(({ n }): [number, Phase] => {
      const mark = flow.marks[n];
      if (mark) return [n, mark];
      const state = seen.get(n);
      if (state) return [n, state === "running" ? "running" : "returned"];
      return [n, ready.includes(n) ? "ready" : "waiting"];
    }),
  );
}

const shows: Partial<Record<Phase, Lane>> = { running: "running", returned: "idle" };

export function occupant(
  links: Readonly<Record<string, number>>,
  n: number,
  phase: Phase | undefined,
  lane: (sessionID: string) => Lane | undefined,
) {
  const want = phase && shows[phase];
  return want && Object.entries(links).find(([id, step]) => step === n && lane(id) === want)?.[0];
}

export function fact({ time, outcome, metadata }: Info): Child | undefined {
  const name = typeof metadata?.lane === "string" ? metadata.lane : undefined;
  if (time.busy !== undefined && time.busy > (time.idle ?? -1)) return { lane: "running", name };
  if (outcome) return { lane: "idle", name };
}

export async function children(
  get: (sessionID: string) => Promise<Info>,
  links: Readonly<Record<string, number>>,
): Promise<Record<string, Child>> {
  const found = await Promise.all(
    Object.keys(links).map(async (id) => {
      const info = await get(id).catch(() => undefined);
      const known = info && fact(info);
      return known ? [[id, known] as const] : [];
    }),
  );
  return Object.fromEntries(found.flat());
}

export function complete(states: Readonly<Record<number, Phase>>) {
  return Object.values(states).filter((phase) => phase === "done" || phase === "skipped").length;
}
