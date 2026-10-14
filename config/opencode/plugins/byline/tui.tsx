import { Plugin } from "@opencode/plugin/tui";
import type { Renderable } from "@opentui/core";
import { createMemo, createRoot, onCleanup } from "solid-js";
import { Ledger, prompt, transcript } from "./rewrite.ts";

type Context = Plugin.Context;
type Pass = (anchor: Renderable, names: () => ReadonlySet<string>, ledger: Ledger, sessionID?: string) => void;

export default Plugin.define({
  id: "byline",
  setup(context) {
    const Prompt = anchor(context, (box, names, ledger, sessionID) => {
      const agent = sessionID ? context.data.session.get(sessionID)?.agent : undefined;
      prompt(box, names, ledger, agent && titlecase(agent));
    });
    const Transcript = anchor(context, (box, names, ledger) => transcript(box, names, ledger, context));
    return createRoot((dispose) => {
      context.ui.slot({
        append: "prompt.footer",
        render: (input) => <Prompt sessionID={input.sessionID} />,
      });
      context.ui.slot({
        append: "session.composer.top",
        render: (input) => <Transcript sessionID={input.sessionID} />,
      });
      return dispose;
    });
  },
});

function anchor(context: Context, pass: Pass) {
  return (props: { sessionID?: string }) => {
    const names = createMemo(() => primaries(context, props.sessionID));

    const attach = (box: Renderable) => {
      const ledger = new Ledger();
      const run = () => pass(box, names, ledger, props.sessionID);
      box.onLifecyclePass = run;
      context.renderer.registerLifecyclePass(box);
      return () => {
        box.onLifecyclePass = null;
        context.renderer.unregisterLifecyclePass(box);
        ledger.clear();
      };
    };

    return <box position="absolute" ref={(box) => onCleanup(attach(box))} />;
  };
}

function primaries(context: Context, sessionID?: string) {
  const data = context.data;
  const location =
    (sessionID ? data.session.get(sessionID)?.location : undefined) ?? context.location ?? data.location.default();
  const agents = data.location.agent.list(location) ?? [];
  return new Set(
    agents.filter((agent) => agent.mode === "primary" && !agent.hidden).map((agent) => titlecase(agent.id)),
  );
}

function titlecase(value: string) {
  return value.replace(/\b\w/g, (char) => char.toUpperCase());
}
