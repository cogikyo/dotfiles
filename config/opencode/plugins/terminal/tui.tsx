import { Plugin } from "@opencode/plugin/tui";
import { CodeRenderable, type CliRenderer, type Renderable } from "@opentui/core";
import { createRoot, onCleanup } from "solid-js";
import { leaves, Ledger, owned, textual, type Text } from "../byline/ledger.ts";
import { load as language } from "../code-blocks/parsers.ts";
import { load, type Vagari } from "../code-blocks/vagari.ts";
import { framed } from "../statusline/prompt.ts";

type Context = Plugin.Context;
type Leaf = ReturnType<typeof leaves>[number];
type Shell = { renderer: CliRenderer; vagari: Vagari; filetype: string };

export default Plugin.define({
  id: "terminal",
  setup(context) {
    const vagari = load(context.renderer.nativeScene);
    const shell = { renderer: context.renderer, vagari, filetype: language("bash").filetype };
    const unmount = createRoot((dispose) => {
      context.ui.slot({
        append: "session.composer.top",
        render: (input) => <Tint context={context} shell={shell} sessionID={input.sessionID} />,
      });
      return dispose;
    });
    return () => {
      unmount();
      return context.renderer.idle().then(() => vagari.destroy());
    };
  },
});

function Tint(props: { context: Context; shell: Shell; sessionID?: string }) {
  const attach = (anchor: Renderable) => {
    const { renderer } = props.context;
    const ledger = new Ledger();
    const run = () => {
      ledger.prune();
      const docked = locate(props.context, anchor, props.sessionID);
      if (!docked) return ledger.clear();
      const red = props.context.theme.text.feedback.error.base;
      const { frame, title, command } = docked;
      ledger.paint(title, title, red);
      ledger.adjust(frame, "border", red, {
        get: () => frame.borderColor,
        set: (color) => (frame.borderColor = color),
      });
      highlight(ledger, frame, command, props.shell);
    };
    anchor.onLifecyclePass = run;
    renderer.registerLifecyclePass(anchor);
    return () => {
      anchor.onLifecyclePass = null;
      renderer.unregisterLifecyclePass(anchor);
      ledger.clear();
    };
  };

  return <box position="absolute" ref={(box) => onCleanup(attach(box))} />;
}

function locate({ data }: Context, anchor: Renderable, sessionID?: string) {
  if (sessionID === undefined) return;
  const form = data.session.form.list(sessionID)?.[0];
  const command = form && call(data, sessionID, form.metadata);
  if (!command) return;
  for (const frame of anchor.parent?.getChildren() ?? []) {
    if (!framed(frame)) continue;
    const title = find(frame, (value) => value === form.title, 3);
    if (title) return { frame, title: title.text, command };
  }
}

function call(data: Context["data"], sessionID: string, metadata: unknown) {
  if (!record(metadata) || metadata.kind !== "question" || !record(metadata.tool)) return;
  const { messageID, id } = metadata.tool;
  if (typeof messageID !== "string") return;
  const message = data.session.message.get(sessionID, messageID);
  if (message?.type !== "assistant") return;
  const part = message.content.find((item) => item.type === "tool" && item.id === id);
  if (part?.type !== "tool" || part.name !== "terminal") return;
  const command = record(part.state.input) ? part.state.input.command : undefined;
  return typeof command === "string" ? command : undefined;
}

function highlight(ledger: Ledger, frame: Renderable, command: string, shell: Shell) {
  const body = find(frame, (value) => value.endsWith(`\n\n${command}`), 4);
  const box = body?.text.parent;
  if (!body || !box) return;
  const [node, index, value] = body.leaf;
  ledger.write(body.text, node, index, value.slice(0, -command.length).trimEnd());
  if (box.getChildren().some((child) => owned.has(child))) return;
  const code = new CodeRenderable(shell.renderer, {
    content: command,
    filetype: shell.filetype,
    syntaxStyle: shell.vagari.style("bash"),
    fg: shell.vagari.fg,
    drawUnstyledText: true,
    wrapMode: "word",
    marginTop: 1,
    marginLeft: 2,
    flexShrink: 0,
  });
  owned.add(code);
  ledger.mount(box, code, box.getChildren().indexOf(body.text) + 1);
}

function find(
  node: Renderable,
  match: (value: string) => boolean,
  depth: number,
): { text: Text; leaf: Leaf } | undefined {
  for (const child of node.getChildren()) {
    if (owned.has(child)) continue;
    if (textual(child)) {
      const leaf = leaves(child.textNode).find(([, , value]) => match(value));
      if (leaf) return { text: child, leaf };
    } else if (depth > 1) {
      const found = find(child, match, depth - 1);
      if (found) return found;
    }
  }
}

function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
