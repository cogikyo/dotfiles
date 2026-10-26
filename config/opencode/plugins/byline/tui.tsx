import { Plugin } from "@opencode/plugin/tui";
import type { Renderable } from "@opentui/core";
import { createMemo, createRoot, onCleanup } from "solid-js";
import { animate, refresh } from "../statusline/working.tsx";
import { Stage } from "./draw.ts";
import { Ledger } from "./ledger.ts";
import { prompt, transcript } from "./rewrite.ts";

type Context = Plugin.Context;
type Pass = {
  run: (anchor: Renderable, names: () => ReadonlySet<string>, ledger: Ledger, sessionID?: string) => boolean | void;
  dispose?: () => void;
};

export default Plugin.define({
  id: "byline",
  setup(context) {
    void refresh();
    const Prompt = anchor(context, () => ({
      run: (box, names, ledger, sessionID) => {
        const agent = sessionID ? context.data.session.get(sessionID)?.agent : undefined;
        prompt(box, names, ledger, agent && titlecase(agent));
      },
    }));
    const Transcript = anchor(context, () => {
      const stage = new Stage(context.renderer);
      return {
        run: (box, names, ledger, sessionID) => transcript(box, names, ledger, { context, sessionID, stage }),
        dispose: () => stage.dispose(),
      };
    });
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

function anchor(context: Context, make: () => Pass) {
  return (props: { sessionID?: string }) => {
    const names = createMemo(() => primaries(context, props.sessionID));

    const attach = (box: Renderable) => {
      const ledger = new Ledger();
      const pass = make();
      let stop: (() => void) | undefined;
      const run = () => {
        const moving = pass.run(box, names, ledger, props.sessionID) === true;
        if (moving && !stop) stop = animate(context.renderer);
        if (!moving && stop) {
          stop();
          stop = undefined;
        }
      };
      box.onLifecyclePass = run;
      context.renderer.registerLifecyclePass(box);
      return () => {
        box.onLifecyclePass = null;
        context.renderer.unregisterLifecyclePass(box);
        stop?.();
        pass.dispose?.();
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
