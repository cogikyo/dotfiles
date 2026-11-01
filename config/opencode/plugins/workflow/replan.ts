import type { Plugin } from "@opencode/plugin/tui";
import { latest } from "../lanes/label.ts";
import { cold, loaded } from "../lanes/step.ts";
import type { Live } from "./live.ts";
import { status } from "./state.ts";

type Context = Plugin.Context;
type Session = ReturnType<Context["data"]["session"]["list"]>[number];

export async function replan(context: Context, live: Live, sessionID: string) {
  const root = context.data.session.root(sessionID);
  const { flow } = await live.rpc.pause({ sessionID: root }, live.options(root));
  if (!flow) return context.ui.toast.show({ variant: "warning", message: "No active workflow to replan" });
  const children = context.data.session
    .list()
    .filter((session) => session.parentID === root && session.metadata?.closed === undefined)
    .filter((session) => lane(session) !== undefined || session.id in flow.links);
  await Promise.all(children.map((session) => context.data.session.message.sync(session.id)));
  const facts = children.map((session) => fact(context, session, flow.links[session.id]));
  await context.client.session.prompt({ sessionID: root, text: brief(status(flow), facts) });
}

function lane(session: Session) {
  const value = session.metadata?.lane;
  return typeof value === "string" ? value : undefined;
}

function fact(context: Context, session: Session, step: number | undefined) {
  const running = context.data.session.status(session.id) === "running";
  const messages = context.data.session.message.list(session.id);
  const last = messages.at(-1);
  const idle = !running && last?.type === "idle" ? Date.now() - last.time.created : undefined;
  const size = loaded(latest(messages)?.tokens);
  return [
    lane(session) ?? `unnamed "${session.title ?? session.id}"`,
    step === undefined ? "no step" : `step ${step}`,
    running ? "running" : (session.outcome ?? "idle"),
    ...(idle === undefined ? [] : [`idle ${Math.floor(idle / 60_000)}m`]),
    `${Math.round(size / 1000)}K context`,
    `cache ${idle !== undefined && cold(idle, size) ? "cold" : "warm"}`,
  ].join(" · ");
}

function brief(report: string, facts: readonly string[]) {
  return [
    "Replan requested. The workflow is paused.",
    "",
    report,
    "",
    "Lanes:",
    ...(facts.length ? facts.map((line) => `- ${line}`) : ["- none"]),
    "",
    "Load `recommended`. For each lane, decide whether to resume it, start it fresh, or close it, with a reason. Ask your questions, then compact; this replan pre-approves that compaction. After compacting, write the next workflow revision and call `workflow` with `start`.",
  ].join("\n");
}
