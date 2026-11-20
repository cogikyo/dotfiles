import { Plugin } from "@opencode/plugin/tui";
import { BoxRenderable, MouseButton, TextRenderable } from "@opentui/core";

export default Plugin.define({
  id: "compact",
  setup(context) {
    context.markdown.registerCodeBlockRenderer("checkpoint", (token, host) =>
      host.conceal ? fold(context, token.text) : host.defaultRender(),
    );
  },
});

export function fold(context: Pick<Plugin.Context, "renderer" | "theme">, body: string) {
  const lines = body.split("\n").length;
  let open = false;
  const label = () => `${open ? "▾" : "▸"} Checkpoint · ${lines} lines`;
  const head = new TextRenderable(context.renderer, {
    content: label(),
    fg: context.theme.text.muted,
    wrapMode: "none",
    onMouseDown(event) {
      if (event.button !== MouseButton.LEFT) return;
      open = !open;
      head.content = label();
      text.visible = open;
    },
  });
  const text = new TextRenderable(context.renderer, {
    content: body,
    fg: context.theme.text.muted,
    wrapMode: "word",
    visible: false,
  });
  const box = new BoxRenderable(context.renderer, {
    width: "100%",
    flexShrink: 0,
    flexDirection: "column",
  });
  box.add(head);
  box.add(text);
  return box;
}
