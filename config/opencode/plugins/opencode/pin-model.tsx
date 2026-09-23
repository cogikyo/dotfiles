/** @jsxImportSource @opentui/solid */
import type { TuiPlugin, TuiPluginApi, TuiPluginModule } from "@opencode-ai/plugin/tui";
import { mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";
import { getOwner, onCleanup, onMount } from "solid-js";

const id = "opencode-pin-model";
const CONFIG_PATH = "/home/cullyn/dotfiles/config/opencode/opencode.json";
const MODEL_LINE = /^(\s*"model":\s*)("[^"]*")/m;

type ModelRef = {
  providerID: string;
  modelID: string;
};

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

type Pin = {
  model: string;
  variant?: string;
};

let local: Local | undefined;

function isLocal(value: unknown): value is Local {
  if (!value || typeof value !== "object") return false;
  const model = (value as { model?: Partial<LocalModel> }).model;
  return (
    typeof model?.current === "function" &&
    typeof model?.set === "function" &&
    typeof model.variant?.current === "function" &&
    typeof model.variant?.set === "function"
  );
}

function contextValues(ctx: object) {
  const record = ctx as Record<PropertyKey, unknown>;
  return [...Object.values(record), ...Object.getOwnPropertySymbols(record).map((key) => record[key])];
}

// The model picker is stored in Solid owner context; the TUI plugin API has no model setter.
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

function parseModel(value: string): ModelRef | undefined {
  const index = value.indexOf("/");
  if (index <= 0 || index === value.length - 1) return undefined;
  return {
    providerID: value.slice(0, index),
    modelID: value.slice(index + 1),
  };
}

function modelKey(model: ModelRef) {
  return `${model.providerID}/${model.modelID}`;
}

function formatPin(model: ModelRef, variant: string | undefined) {
  return variant ? `${modelKey(model)} ${variant}` : modelKey(model);
}

function pinPath(api: TuiPluginApi) {
  return join(api.state.path.state, "pin.json");
}

function toast(api: TuiPluginApi, message: string, variant: "info" | "warning" | "error" = "info") {
  api.ui.toast({ message, variant });
}

async function readText(path: string) {
  try {
    return await readFile(path, "utf8");
  } catch (error) {
    if ((error as { code?: string }).code === "ENOENT") return undefined;
    throw error;
  }
}

async function writeAtomic(path: string, content: string) {
  await mkdir(dirname(path), { recursive: true });
  const temporary = `${path}.${process.pid}.${crypto.randomUUID()}.tmp`;
  try {
    await writeFile(temporary, content);
    await rename(temporary, path);
  } catch (error) {
    await rm(temporary, { force: true }).catch(() => undefined);
    throw error;
  }
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

async function writeConfigModel(model: string) {
  const text = await readText(CONFIG_PATH);
  if (text === undefined) throw new Error(`missing ${CONFIG_PATH}`);
  const encoded = JSON.stringify(model);
  if (!MODEL_LINE.test(text)) throw new Error("opencode.json has no top-level model field");
  const next = text.replace(MODEL_LINE, `$1${encoded}`);
  if (next === text) return;
  await writeAtomic(CONFIG_PATH, next);
}

async function readPin(api: TuiPluginApi): Promise<Pin | undefined> {
  const text = await readText(pinPath(api));
  if (text !== undefined) {
    const value = JSON.parse(text) as Pin;
    if (typeof value?.model === "string" && parseModel(value.model)) {
      return {
        model: value.model,
        variant: typeof value.variant === "string" ? value.variant : undefined,
      };
    }
  }

  const config = await readText(CONFIG_PATH);
  const model = config ? readConfigModel(config) : undefined;
  if (!model || !parseModel(model)) return undefined;
  return { model };
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
    await writeAtomic(pinPath(api), `${JSON.stringify(pin)}\n`);
  } catch (error) {
    toast(api, error instanceof Error ? error.message : "Failed to pin model", "error");
    return;
  }

  local?.model.set(selection.model, { recent: true });
  local?.model.variant.set(selection.variant);
  toast(api, `Pinned ${formatPin(selection.model, selection.variant)}`);
}

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

/** Registers a TUI app slot and keymap commands for pinning or switching models. */
export default { id, tui } satisfies TuiPluginModule & { id: string };
