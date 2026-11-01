import type { Plugin } from "@opencode/plugin/tui";
import { createStore, reconcile } from "solid-js/store";
import { parse, type Workflow } from "./parse.ts";
import { occupant, phases, type Lane, type Phase, type Progress } from "./progress.ts";
import { api, type State } from "./rpc.ts";
import { ready } from "./state.ts";

type Context = Plugin.Context;

export type Live = ReturnType<typeof live>;

export type Shown = {
  workflow: Workflow;
  progress: Omit<Progress, "frame">;
  agents: Record<number, string>;
};

const retry = 5000;

export function live(context: Context) {
  const rpc = context.client.rpc(api);
  const [states, setStates] = createStore<Record<string, State>>({});
  const asked = new Set<string>();
  const failures = new Set<string>();

  const options = (root: string) => {
    const session = context.data.session.get(root);
    return session ? { location: { directory: session.location.directory } } : undefined;
  };

  const fail = (error: unknown) => {
    const message = error instanceof Error ? error.message : JSON.stringify(error);
    if (failures.has(message)) return;
    failures.add(message);
    context.ui.toast.show({ variant: "error", title: "Workflow", message });
  };

  const stop = new AbortController();
  const latest = new Map<string, number>();
  const load = async (root: string): Promise<void> => {
    const ticket = (latest.get(root) ?? 0) + 1;
    latest.set(root, ticket);
    const current = () => latest.get(root) === ticket && !stop.signal.aborted;
    try {
      const state = await rpc.state({ sessionID: root }, options(root));
      if (current()) setStates(root, reconcile(state));
    } catch (error) {
      fail(error);
      await Bun.sleep(retry);
      if (current()) return load(root);
    }
  };

  const follow = async (): Promise<void> => {
    try {
      for await (const event of rpc.events.subscribe("changed", { signal: stop.signal }))
        if (asked.has(event.data.root)) void load(event.data.root);
    } catch (error) {
      if (!stop.signal.aborted) fail(error);
    }
    if (stop.signal.aborted) return;
    await Bun.sleep(retry);
    for (const root of asked) void load(root);
    return follow();
  };

  const synced = new Set<string>();
  const lane = (sessionID: string): Lane | undefined => {
    if (context.data.session.status(sessionID) === "running") return "running";
    const session = context.data.session.get(sessionID);
    if (!session && !synced.has(sessionID)) {
      synced.add(sessionID);
      void context.data.session.sync(sessionID).catch(fail);
    }
    if (session?.outcome) return "idle";
  };

  const state = (sessionID: string) => {
    const root = context.data.session.root(sessionID);
    if (!asked.has(root)) {
      asked.add(root);
      void load(root);
    }
    return states[root];
  };

  const shown = (sessionID: string): Shown | undefined => {
    const flow = state(sessionID)?.flow;
    if (!flow) return;
    const workflow = parse(flow.source);
    const phased = phases(flow, workflow.steps, ready(flow, workflow.steps), lane);
    return {
      workflow,
      progress: { ...flow, phases: phased },
      agents: agents(context, lane, { workflow, links: flow.links, phased }),
    };
  };

  void follow();

  return { rpc, options, fail, shown, dispose: () => stop.abort() };
}

function agents(
  context: Context,
  lane: (sessionID: string) => Lane | undefined,
  { workflow, links, phased }: { workflow: Workflow; links: Record<string, number>; phased: Record<number, Phase> },
) {
  return Object.fromEntries(
    workflow.steps.flatMap(({ n, owner }) => {
      const id = occupant(links, n, phased[n], lane);
      if (!id) return [];
      const agent =
        owner.kind === "lane" && owner.role ? `${owner.role}/${owner.scope}` : context.data.session.get(id)?.agent;
      return agent ? [[n, agent]] : [];
    }),
  );
}
