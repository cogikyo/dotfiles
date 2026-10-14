import { usePlugin, type Plugin } from "@opencode/plugin/tui";
import type { OptimizedBuffer, Renderable } from "@opentui/core";
import { createSignal, onCleanup, onMount } from "solid-js";
import { colors } from "./palette.ts";
import { cell, hook, locate, type Prompt } from "./prompt.ts";

type Context = Plugin.Context;

const glyph = { steer: "\u{f048a}", queue: "\u{f1163}" };
const gap = 2;
const [queued, setQueued] = createSignal(false);

export function Delivery(props: { sessionID?: string; mode: "normal" | "shell" }) {
  const context = usePlugin();
  let host: Renderable | undefined;
  const [editor, setEditor] = createSignal<Prompt["editor"]>();
  const running = () => props.sessionID !== undefined && context.data.session.status(props.sessionID) === "running";

  // Priority 1 beats the native input.submit textarea layer at 0.
  // Omitting mode limits this layer to base mode, so autocomplete, pickers, and dialogs keep Enter.
  context.keymap.layer(() => ({
    target: editor,
    enabled: () => editor() !== undefined,
    priority: 1,
    commands: [
      {
        bind: "ctrl+return",
        title: "Toggle steer or queue",
        run: () => {
          setQueued(!queued());
          context.renderer.requestRender();
        },
      },
      {
        bind: "return",
        title: "Queue prompt",
        enabled: () => queued() && running() && props.mode === "normal" && !!editor()?.plainText.trim(),
        run: () => context.keymap.dispatch("prompt.queue"),
      },
    ],
  }));

  onMount(() => {
    const prompt = host && locate(host);
    if (!prompt) return;
    setEditor(prompt.editor);
    onCleanup(
      hook(prompt.row, (buffer) => {
        if (props.sessionID) tip(context, buffer, prompt.editor);
      }),
    );
  });

  return <box position="absolute" ref={(box) => (host = box)} />;
}

/** First column the send icon reserves; anything drawn at or right of it hides the icon. */
export function edge(editor: Renderable) {
  return editor.screenX + editor.width - Math.max(...Object.values(glyph).map((icon) => Bun.stringWidth(icon))) - gap;
}

function tip(context: Context, buffer: OptimizedBuffer, editor: Renderable) {
  const icon = queued() ? glyph.queue : glyph.steer;
  const width = Bun.stringWidth(icon);
  const x = editor.screenX + editor.width - width;
  const y = editor.screenY;
  if (x - gap < editor.screenX) return;
  for (let column = x - gap; column < x + width; column++) if (cell(buffer, column, y) !== undefined) return;
  buffer.drawText(icon, x, y, queued() ? colors.yellow : context.theme.text.muted);
}
