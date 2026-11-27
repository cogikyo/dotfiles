import { Plugin } from "@opencode/plugin/tui";
import type { CliRenderer, MouseEvent, Renderable } from "@opentui/core";
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { homedir } from "node:os";
import { fileURLToPath } from "node:url";
import { createRoot, onCleanup } from "solid-js";
import { locate } from "../byline/scroll.ts";
import { parse, resolver, show } from "./commit.ts";
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
    const commit = resolver();

    const wire = (markdown: Markdown, session: string) => {
      let wiring = wired.get(markdown);
      if (!wiring) {
        const fresh: Wiring = {
          wrapper: (spans, inline) => {
            const repo = directory(context, session);
            const sha = (token: string) => (repo ? commit(repo, token) : undefined);
            return split(fresh.base ? fresh.base(spans, inline) : spans, home, existsSync, sha);
          },
          base: undefined,
          baseFilter: undefined,
          filter: undefined,
        };
        wiring = fresh;
        wired.set(markdown, wiring);
        prune(live);
        live.add(new WeakRef(markdown));
        markdown.onMouse = (event) => click(context, session, event);
      }
      sync(markdown, wiring);
    };

    const scan = (anchor: Renderable, session: string) => {
      const scroll = locate(anchor);
      if (!scroll) return;
      visit(scroll.content, (markdown) => {
        if (broken.has(markdown)) return;
        try {
          wire(markdown, session);
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

    const attach = (box: Renderable, session: () => string) => {
      box.onLifecyclePass = () => scan(box, session());
      context.renderer.registerLifecyclePass(box);
      return () => {
        box.onLifecyclePass = null;
        context.renderer.unregisterLifecyclePass(box);
      };
    };
    const Anchor = (props: { sessionID: string }) => (
      <box position="absolute" ref={(box) => onCleanup(attach(box, () => props.sessionID))} />
    );

    return createRoot((dispose) => {
      context.ui.slot({ append: "session.composer.top", render: (input) => <Anchor sessionID={input.sessionID} /> });
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

// Every linked path holds a slash, and every linked hash sits in a code span.
const needles = ["/", "`"];

function extend(filter: readonly string[] | undefined, hooked: boolean) {
  if (hooked && filter === undefined) return undefined;
  const missing = needles.filter((needle) => !filter?.includes(needle));
  return missing.length === 0 ? filter : [...(filter ?? []), ...missing];
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

function directory(context: Context, session: string) {
  return context.data.session.get(session)?.location.directory;
}

function click(context: Context, session: string, event: MouseEvent) {
  if (event.type !== "up" || !plain(context.renderer, event)) return;
  const url = context.renderer.getLinkAt(event.x, event.y);
  const commit = url ? parse(url, directory(context, session)) : undefined;
  if (commit) {
    event.stopPropagation();
    show(commit, (message) => fail(context, message)).catch((error) =>
      fail(context, error instanceof Error ? error.message : String(error)),
    );
    return;
  }
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
  const child = spawn("xdg-open", [file], { detached: true, stdio: "ignore" });
  child.once("error", (error) => fail(context, error.message || file));
  child.once("exit", (code) => {
    if (code) fail(context, `xdg-open exited ${code}: ${file}`);
  });
  child.unref();
}

function fail(context: Context, message: string) {
  context.ui.toast.show({ title: "Open failed", message, variant: "error" });
}
