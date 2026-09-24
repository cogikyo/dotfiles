/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { join } from "node:path";
import { getOwner, onCleanup, onMount } from "solid-js";
import { z } from "zod";
import { readText, writeJson, writeText } from "../shared/file.ts";

const id = "opencode-pin-model";
const CONFIG_PATH = "/home/cullyn/dotfiles/config/opencode/opencode.json";
const MODEL_LINE = /^(\s*"model":\s*)("[^"]*")/m;

// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ TUI plugin: pin the current model                                                             │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

const tui: TuiPlugin = async (api) => {
  api.keymap.registerLayer({
    priority: 10_000,
    commands: [
      {
        name: "model.pin",
        title: "Pin current model as default",
        category: "Agent",
        namespace: "palette",
        slashName: "pin",
        run: () => pinCurrent(api),
      },
      {
        name: "model.pinned",
        title: "Switch to pinned model",
        category: "Agent",
        namespace: "palette",
        slashName: "pinned",
        run: () => applyPin(api),
      },
    ],
    bindings: [
      {
        key: "<leader>f",
        cmd: "model.pin",
        desc: "Pin current model as default",
      },
      {
        key: "<leader>shift+t",
        cmd: "model.pinned",
        desc: "Switch to pinned model",
      },
    ],
  });

  api.slots.register({
    order: 20,
    slots: {
      app() {
        return <CaptureLocal />;
      },
    },
  });
};

/** TUI plugin that pins the current model as the default and switches back to its saved model and variant. */
export default { id, tui } satisfies TuiPluginModule & { id: string };

// ├─ Commands ────────────────────────────────────────────────────────────────────────────────────┤

type ModelRef = {
  providerID: string;
  modelID: string;
};

function modelKey(model: ModelRef) {
  return `${model.providerID}/${model.modelID}`;
}

function formatPin(model: ModelRef, variant: string | undefined) {
  return variant ? `${modelKey(model)} ${variant}` : modelKey(model);
}

function toast(api: TuiPluginApi, message: string, variant: "info" | "warning" | "error" = "info") {
  api.ui.toast({ message, variant });
}

function currentSelection() {
  const model = captureLocal()?.model.current();
  if (!model) return undefined;
  return {
    model,
    variant: local?.model.variant.current(),
  };
}

async function pinCurrent(api: TuiPluginApi) {
  const selection = currentSelection();
  if (!selection) {
    toast(api, "No current model to pin", "warning");
    return;
  }

  const model = modelKey(selection.model);
  try {
    await writeConfigModel(model);
    const pin: Pin = { model };
    if (selection.variant) pin.variant = selection.variant;
    await writeJson(pinPath(api), pin);
  } catch (error) {
    toast(api, error instanceof Error ? error.message : "Failed to pin model", "error");
    return;
  }

  local?.model.set(selection.model, { recent: true });
  local?.model.variant.set(selection.variant);
  toast(api, `Pinned ${formatPin(selection.model, selection.variant)}`);
}

/** Keeps the current variant if the picker cannot switch to the pinned model. */
async function applyPin(api: TuiPluginApi) {
  const picker = captureLocal();
  if (!picker) {
    toast(api, "Model picker unavailable", "warning");
    return;
  }

  let pin: Pin | undefined;
  try {
    pin = await readPin(api);
  } catch (error) {
    toast(api, error instanceof Error ? error.message : "Failed to read pinned model", "error");
    return;
  }

  const model = pin ? parseModel(pin.model) : undefined;
  if (!pin || !model) {
    toast(api, "No pinned model", "warning");
    return;
  }

  const current = picker.model.current();
  const currentVariant = picker.model.variant.current();
  if (current && modelKey(current) === pin.model && (currentVariant ?? undefined) === pin.variant) {
    toast(api, `Already ${formatPin(model, pin.variant)}`);
    return;
  }

  picker.model.set(model, { recent: true });
  const next = picker.model.current();
  if (!next || modelKey(next) !== pin.model) return;
  picker.model.variant.set(pin.variant);
  toast(api, formatPin(model, pin.variant));
}

// ├─ Pin files ───────────────────────────────────────────────────────────────────────────────────┤

type Pin = {
  model: string;
  variant?: string;
};
const StoredPin = z.object({
  model: z.string().refine((value) => parseModel(value) !== undefined),
  variant: z.string().optional().catch(undefined),
});

function parseModel(value: string): ModelRef | undefined {
  const index = value.indexOf("/");
  if (index <= 0 || index === value.length - 1) return undefined;
  return {
    providerID: value.slice(0, index),
    modelID: value.slice(index + 1),
  };
}

function pinPath(api: TuiPluginApi) {
  return join(api.state.path.state, "pin.json");
}

async function readPin(api: TuiPluginApi): Promise<Pin | undefined> {
  const text = await readText(pinPath(api));
  if (text !== undefined) {
    const stored = StoredPin.safeParse(JSON.parse(text));
    if (stored.success) return stored.data;
  }

  const config = await readText(CONFIG_PATH);
  const model = config ? readConfigModel(config) : undefined;
  if (!model || !parseModel(model)) return undefined;
  return { model };
}

function readConfigModel(text: string) {
  const match = text.match(MODEL_LINE);
  if (!match) return undefined;
  try {
    const value = JSON.parse(match[2]);
    return typeof value === "string" ? value : undefined;
  } catch {
    return undefined;
  }
}

/** Updates the first `"model"` string in opencode.json by text match rather than parsed JSON. */
async function writeConfigModel(model: string) {
  const text = await readText(CONFIG_PATH);
  if (text === undefined) throw new Error(`missing ${CONFIG_PATH}`);
  const encoded = JSON.stringify(model);
  if (!MODEL_LINE.test(text)) throw new Error("opencode.json has no top-level model field");
  const next = text.replace(MODEL_LINE, `$1${encoded}`);
  if (next === text) return;
  await writeText(CONFIG_PATH, next);
}

// ├─ Model picker ────────────────────────────────────────────────────────────────────────────────┤

type LocalModel = {
  current: () => ModelRef | undefined;
  set: (model: ModelRef, options?: { recent?: boolean }) => void;
  variant: {
    current: () => string | undefined;
    set: (value: string | undefined) => void;
  };
};

type Local = {
  model: LocalModel;
};

type OwnerNode = {
  context?: object | null;
  owner?: OwnerNode | null;
};

const LocalShape = z.object({
  model: z.object({
    current: z.function(),
    set: z.function(),
    variant: z.object({ current: z.function(), set: z.function() }),
  }),
});

let local: Local | undefined;

function isLocal(value: unknown): value is Local {
  return LocalShape.safeParse(value).success;
}

function contextValues(ctx: object) {
  return [...Object.values(ctx), ...Object.getOwnPropertySymbols(ctx).map((key) => Reflect.get(ctx, key))];
}

/** Finds the model picker in Solid owner context because the TUI plugin API has no model setter. */
function findLocal(owner: OwnerNode | null | undefined) {
  const seen = new Set<object>();
  while (owner) {
    const ctx = owner.context;
    if (ctx && !seen.has(ctx)) {
      seen.add(ctx);
      for (const value of contextValues(ctx)) {
        if (isLocal(value)) return value;
      }
    }
    owner = owner.owner ?? undefined;
  }
  return undefined;
}

function captureLocal() {
  const next = findLocal(getOwner());
  if (next) local = next;
  return local;
}

function CaptureLocal() {
  onMount(() => {
    captureLocal();
    const timer = setInterval(() => {
      if (local) {
        clearInterval(timer);
        return;
      }
      captureLocal();
    }, 250);
    onCleanup(() => clearInterval(timer));
  });
  return <box width={0} height={0} />;
}
