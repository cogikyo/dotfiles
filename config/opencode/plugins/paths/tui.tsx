import { Plugin } from "@opencode/plugin/tui";
import type { CliRenderer, MouseEvent, Renderable } from "@opentui/core";
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";
import { createRoot, onCleanup } from "solid-js";
import { locate } from "../byline/scroll.ts";
import { split, type Span } from "./split.ts";

type Context = Plugin.Context;
type Hook = (spans: Span[], context: unknown) => Span[];
type Markdown = Renderable & {
  inlineHook: Hook | undefined;
  inlineHookFilter: readonly string[] | undefined;
  rerunInlineHook(select: (source: string) => boolean): void;
};
type Wiring = {
  wrapper: Hook;
  base: Hook | undefined;
  baseFilter: readonly string[] | undefined;
  filter: readonly string[] | undefined;
};

export default Plugin.define({
  id: "paths",
  setup(context) {
    const wired = new WeakMap<Markdown, Wiring>();
    const live = new Set<WeakRef<Markdown>>();
    const broken = new WeakSet<Markdown>();
    const home = homedir();

    const wire = (markdown: Markdown) => {
      let wiring = wired.get(markdown);
      if (!wiring) {
        const fresh: Wiring = {
          wrapper: (spans, inline) => split(fresh.base ? fresh.base(spans, inline) : spans, home, existsSync),
          base: undefined,
          baseFilter: undefined,
          filter: undefined,
        };
        wiring = fresh;
        wired.set(markdown, wiring);
        prune(live);
        live.add(new WeakRef(markdown));
        markdown.onMouse = (event) => click(context, event);
      }
      sync(markdown, wiring);
    };

    const scan = (anchor: Renderable) => {
      const scroll = locate(anchor);
      if (!scroll) return;
      visit(scroll.content, (markdown) => {
        if (broken.has(markdown)) return;
        try {
          wire(markdown);
        } catch {
          broken.add(markdown);
        }
      });
    };

    const restore = () => {
      for (const ref of live) {
        const markdown = ref.deref();
        const wiring = markdown && wired.get(markdown);
        if (!markdown || !wiring || markdown.isDestroyed) continue;
        if (markdown.inlineHook === wiring.wrapper) markdown.inlineHook = wiring.base;
        if (markdown.inlineHookFilter === wiring.filter) markdown.inlineHookFilter = wiring.baseFilter;
        markdown.onMouse = undefined;
      }
      live.clear();
    };

    const attach = (box: Renderable) => {
      box.onLifecyclePass = () => scan(box);
      context.renderer.registerLifecyclePass(box);
      return () => {
        box.onLifecyclePass = null;
        context.renderer.unregisterLifecyclePass(box);
      };
    };
    const Anchor = () => <box position="absolute" ref={(box) => onCleanup(attach(box))} />;

    return createRoot((dispose) => {
      context.ui.slot({ append: "session.composer.top", render: () => <Anchor /> });
      return () => {
        dispose();
        restore();
      };
    });
  },
});

function sync(markdown: Markdown, wiring: Wiring) {
  const hook = markdown.inlineHook;
  const filter = markdown.inlineHookFilter;
  const rehook = hook !== wiring.wrapper;
  const refilter = filter !== wiring.filter;
  if (!rehook && !refilter) return;
  if (rehook) wiring.base = hook;
  if (refilter) wiring.baseFilter = filter;
  wiring.filter = extend(wiring.baseFilter, wiring.base !== undefined);
  if (markdown.inlineHookFilter !== wiring.filter) markdown.inlineHookFilter = wiring.filter;
  if (rehook) markdown.inlineHook = wiring.wrapper;
}

function extend(filter: readonly string[] | undefined, hooked: boolean) {
  if (hooked && filter === undefined) return undefined;
  if (filter?.includes("/")) return filter;
  return [...(filter ?? []), "/"];
}

function visit(node: Renderable, found: (markdown: Markdown) => void) {
  for (const child of node.getChildren()) {
    if (markdownish(child)) found(child);
    else visit(child, found);
  }
}

function markdownish(node: Renderable): node is Markdown {
  return "inlineHook" in node && "inlineHookFilter" in node && "rerunInlineHook" in node;
}

function prune(live: Set<WeakRef<Markdown>>) {
  for (const ref of live) {
    const markdown = ref.deref();
    if (!markdown || markdown.isDestroyed) live.delete(ref);
  }
}

function click(context: Context, event: MouseEvent) {
  if (event.type !== "up" || !plain(context.renderer, event)) return;
  const url = context.renderer.getLinkAt(event.x, event.y);
  if (!url?.startsWith("file:")) return;
  let file: string;
  try {
    file = fileURLToPath(url);
  } catch {
    return;
  }
  event.stopPropagation();
  open(context, file);
}

function plain(renderer: CliRenderer, event: MouseEvent) {
  if (!event.isDragging || event.modifiers.ctrl || event.modifiers.alt || event.modifiers.shift) return false;
  const selection = renderer.getSelection();
  return selection?.isStart === true && selection.behavior === "cell";
}

function open(context: Context, file: string) {
  const fail = (message: string) => context.ui.toast.show({ title: "Open failed", message, variant: "error" });
  const child = spawn("xdg-open", [file], { detached: true, stdio: "ignore" });
  child.once("error", (error) => fail(error.message || file));
  child.once("exit", (code) => {
    if (code) fail(`xdg-open exited ${code}: ${file}`);
  });
  child.unref();
}
