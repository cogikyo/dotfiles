import { Plugin } from "@opencode/plugin/tui";
import { claudeAccounts } from "./accounts.ts";
import { ranked, reopens, survey } from "./pool.ts";

const models = [
  { id: "claude-fable-5-2", variant: "high" },
  { id: "claude-opus-5-5", variant: "xhigh" },
] as const;

type Ref = { providerID: string; modelID: string };

const mains = Object.values(claudeAccounts).filter((account) => account.main);

const key = (ref?: Ref) => `${ref?.providerID}/${ref?.modelID}`;

async function pick(modelID: string) {
  const list = (await survey(modelID)).filter((usage) => mains.some((account) => account.id === usage.id));
  const open = ranked(list)[0];
  const usage = open ?? list.toSorted((a, b) => reopens(a) - reopens(b))[0];
  return { account: mains.find((account) => account.id === usage.id)!, capped: !open };
}

function reach(context: Plugin.Context, target: Ref) {
  const seen = new Set<string>();
  let current = context.ui.model.current();
  while (key(current) !== key(target)) {
    if (seen.has(key(current))) return false;
    seen.add(key(current));
    context.keymap.dispatch("model.cycle_favorite");
    current = context.ui.model.current();
  }
  return true;
}

export default Plugin.define({
  id: "anthropic",
  setup(context) {
    const where = () => {
      const route = context.ui.router.current();
      return route.type === "session" ? route.sessionID : route.type;
    };

    const toggle = async (route: string) => {
      const start = context.ui.model.current();
      const index = models.findIndex((model) => model.id === start?.modelID);
      const model = models[(index + 1) % models.length];
      const { account, capped } = await pick(model.id);
      if (where() !== route || key(context.ui.model.current()) !== key(start)) return;
      const target = { providerID: account.id, modelID: model.id };
      if (!reach(context, target)) {
        const now = context.ui.model.current();
        const moved = key(now) === key(start) ? "" : `; selection is now ${key(now)}`;
        return context.ui.toast.show({
          variant: "error",
          title: "Model not switched",
          message: `Favorite ${key(target)} to switch to it${moved}`,
        });
      }
      context.ui.model.variant.set(model.variant);
      if (capped)
        context.ui.toast.show({
          variant: "warning",
          message: `Both main accounts are capped for ${model.id}; using ${account.label}, which reopens first`,
        });
    };

    let queue = Promise.resolve();
    context.keymap.layer(() => ({
      commands: [
        {
          id: "anthropic.model.toggle",
          title: "Toggle main model",
          group: "Agent",
          bind: "ctrl+m",
          run() {
            const route = where();
            const next = queue.then(() => toggle(route));
            queue = next.catch(() => undefined);
            return next;
          },
        },
      ],
    }));
  },
});
