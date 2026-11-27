import {
  MarkdownRenderable,
  SyntaxStyle,
  TextAttributes,
  type CliRenderer,
  type Renderable,
  type RGBA,
} from "@opentui/core";
import type { Paint } from "./paint.ts";

// The host runs OpenTUI 0.9.9; node_modules types are 0.8.0, which predate inline hooks.
export type Span = {
  readonly text: string;
  readonly fg?: RGBA;
  readonly bg?: RGBA;
  readonly attributes?: number;
  readonly link?: { readonly url: string };
  readonly code: boolean;
  readonly math?: string;
  readonly target?: string;
};
export type Context = {
  readonly source: string;
  readonly conceal: boolean;
  readonly syntaxStyle: SyntaxStyle;
  readonly tableCell: boolean;
};
type Hook = (spans: Span[], context: Context) => Span[];
type Rerun = { rerunInlineHook(select: (source: string) => boolean): void };
type Filter = readonly string[];

export type Look = {
  code(text: string, fg: RGBA | undefined): Paint | undefined;
  heading(level: number, fg: RGBA | undefined): Paint | undefined;
  session(fg: RGBA | undefined): Paint | undefined;
};

type Accessor<T> = PropertyDescriptor & {
  get(this: MarkdownRenderable): T | undefined;
  set(this: MarkdownRenderable, value: T | undefined): void;
};

function accessor<T>(name: string): Accessor<T> {
  const descriptor = Object.getOwnPropertyDescriptor(MarkdownRenderable.prototype, name);
  if (!descriptor?.get || !descriptor.set)
    throw new Error(`MarkdownRenderable.${name} is no longer an accessor; recheck the OpenTUI version`);
  return descriptor as Accessor<T>;
}

function pair<T extends object>(make: (value: T) => T) {
  const out = new WeakMap<T, T>();
  const back = new WeakMap<T, T>();
  return {
    wrap(value: T) {
      let result = out.get(value);
      if (!result) {
        result = make(value);
        out.set(value, result);
        back.set(result, value);
      }
      return result;
    },
    unwrap: (value: T | undefined) => (value && back.get(value)) ?? value,
    owns: (value: T | undefined) => value !== undefined && back.has(value),
  };
}

function run(source: string, from: number) {
  let end = from;
  while (source[end] === "`") end++;
  return end;
}

/** Counts the inline code spans in `source` that have their closing backticks, since streaming text can end inside an open one. */
export function closed(source: string) {
  let count = 0;
  let at = 0;
  while (at < source.length) {
    const char = source[at];
    if (char === "\\") at += 2;
    else if (char === "]" && source[at + 1] === "(") {
      const end = source.indexOf(")", at);
      if (end === -1) return count;
      at = end + 1;
    } else if (char === "`") {
      const end = run(source, at);
      let close = source.indexOf("`", end);
      while (close !== -1 && run(source, close) - close !== end - at) close = source.indexOf("`", run(source, close));
      if (close === -1) return count;
      count++;
      at = close + end - at;
    } else at++;
  }
  return count;
}

const embolden = (span: Span, { fg, bold }: Paint): Span => ({
  ...span,
  fg,
  ...(bold && { attributes: (span.attributes ?? 0) | TextAttributes.BOLD }),
});

const idle = (renderer: CliRenderer) => renderer.idle().catch(() => undefined);

class Styles {
  private readonly current = new Map<SyntaxStyle, SyntaxStyle>();
  private readonly origin = new Map<SyntaxStyle, SyntaxStyle>();

  constructor(
    private readonly renderer: CliRenderer,
    private readonly heading: Look["heading"],
    private readonly used: () => Iterable<SyntaxStyle | undefined>,
  ) {}

  derive(value: SyntaxStyle) {
    const shared = this.original(value) ?? value;
    let copy = this.current.get(shared);
    if (copy) return copy;
    const headings = [1, 2, 3, 4, 5, 6].flatMap((level) => {
      const name = `markup.heading.${level}`;
      const base = shared.getStyle(name);
      const paint = this.heading(level, base?.fg);
      return base && paint ? [{ name, style: { ...base, fg: paint.fg, ...(paint.bold && { bold: true }) } }] : [];
    });
    if (headings.length === 0) return shared;
    copy = SyntaxStyle.fromStyles(shared.getAllStyles(), this.renderer.nativeScene);
    for (const { name, style } of headings) copy.registerStyle(name, style);
    this.current.set(shared, copy);
    this.origin.set(copy, shared);
    void this.sweep();
    return copy;
  }

  original(style: SyntaxStyle | undefined) {
    return style && this.origin.get(style);
  }

  refresh() {
    this.current.clear();
    void this.sweep();
  }

  async destroy() {
    const copies = [...this.origin.keys()];
    this.current.clear();
    this.origin.clear();
    await idle(this.renderer);
    for (const copy of copies) copy.destroy();
  }

  private async sweep() {
    await idle(this.renderer);
    const keep = new Set(this.used());
    for (const [copy, shared] of this.origin) {
      if (keep.has(copy)) continue;
      this.origin.delete(copy);
      if (this.current.get(shared) === copy) this.current.delete(shared);
      copy.destroy();
    }
  }
}

/**
 * Wraps the `inlineHook`, `inlineHookFilter`, and `syntaxStyle` accessors on the MarkdownRenderable prototype, so renderables the host creates later get the recoloring too.
 * The wrapped getters return the host's own values and `restore` reinstates the original accessors, so the host sees no change.
 */
export function patch(renderer: CliRenderer, look: Look) {
  const proto = MarkdownRenderable.prototype;
  const hookAccess = accessor<Hook>("inlineHook");
  const filterAccess = accessor<Filter>("inlineHookFilter");
  const styleAccess = accessor<SyntaxStyle>("syntaxStyle");
  const live = new Set<WeakRef<MarkdownRenderable>>();
  const seen = new WeakSet<MarkdownRenderable>();
  const dead = new FinalizationRegistry<WeakRef<MarkdownRenderable>>((ref) => live.delete(ref));

  const recolor = (spans: Span[], context: Context) => {
    let settled = closed(context.source);
    return spans.map((span) => {
      if (!span.code || settled-- <= 0 || span.link) return span;
      const paint = look.code(context.conceal ? span.text : span.text.replace(/^`+\s?|\s?`+$/g, ""), span.fg);
      return paint ? embolden(span, paint) : span;
    });
  };
  const relink = (spans: Span[]) =>
    spans.map((span) => {
      if (span.link || !span.target?.startsWith("ses_")) return span;
      const paint = look.session(span.fg);
      return paint ? embolden(span, paint) : span;
    });
  const hooks = pair<Hook>((base) => (spans, context) => relink(base(recolor(spans, context), context)));
  // The host runs the hook only on blocks containing a filter string; add a backtick so inline code reaches `recolor`.
  const filters = pair<Filter>((base) => (base.length === 0 || base.includes("`") ? base : [...base, "`"]));

  const instances = () =>
    [...live].flatMap((ref) => {
      const markdown = ref.deref();
      if (markdown && !markdown.isDestroyed) return [markdown];
      live.delete(ref);
      return [];
    });
  const styles = new Styles(
    renderer,
    (level, fg) => look.heading(level, fg),
    () => instances().map((markdown) => styleAccess.get.call(markdown)),
  );
  const style = (markdown: MarkdownRenderable, value: SyntaxStyle | undefined) => {
    const shared = styles.original(value) ?? value;
    const hooked = hooks.owns(hookAccess.get.call(markdown));
    styleAccess.set.call(markdown, shared && hooked ? styles.derive(shared) : shared);
  };
  const hook = (markdown: MarkdownRenderable, value: Hook | undefined) => {
    hookAccess.set.call(markdown, value && hooks.wrap(value));
    style(markdown, styleAccess.get.call(markdown));
    if (!value || seen.has(markdown)) return;
    const ref = new WeakRef(markdown);
    live.add(ref);
    dead.register(markdown, ref);
    seen.add(markdown);
  };
  const filter = (markdown: MarkdownRenderable, value: Filter | undefined) =>
    filterAccess.set.call(markdown, value && filters.wrap(value));

  const define = <T>(
    name: string,
    access: Accessor<T>,
    set: (markdown: MarkdownRenderable, value: T | undefined) => void,
    get: (value: T | undefined) => T | undefined,
  ) =>
    Object.defineProperty(proto, name, {
      ...access,
      get(this: MarkdownRenderable) {
        return get(access.get.call(this));
      },
      set(this: MarkdownRenderable, value: T | undefined) {
        set(this, value);
      },
    });

  const restore = () => {
    Object.defineProperty(proto, "inlineHook", hookAccess);
    Object.defineProperty(proto, "inlineHookFilter", filterAccess);
    Object.defineProperty(proto, "syntaxStyle", styleAccess);
    for (const markdown of instances()) {
      hookAccess.set.call(markdown, hooks.unwrap(hookAccess.get.call(markdown)));
      filterAccess.set.call(markdown, filters.unwrap(filterAccess.get.call(markdown)));
      const shared = styles.original(styleAccess.get.call(markdown));
      if (shared) styleAccess.set.call(markdown, shared);
    }
    live.clear();
    return styles.destroy();
  };

  const visit = (node: Renderable) => {
    if (node instanceof MarkdownRenderable && hookAccess.get.call(node)) {
      hook(node, hookAccess.get.call(node));
      filter(node, filterAccess.get.call(node));
    }
    for (const child of node.getChildren()) visit(child);
  };
  try {
    define("inlineHook", hookAccess, hook, hooks.unwrap);
    define("inlineHookFilter", filterAccess, filter, filters.unwrap);
    define("syntaxStyle", styleAccess, style, (value) => styles.original(value) ?? value);
    visit(renderer.root);
  } catch (error) {
    void restore();
    throw error;
  }

  return {
    refresh() {
      styles.refresh();
      for (const markdown of instances()) {
        const before = styleAccess.get.call(markdown);
        style(markdown, before);
        if (styleAccess.get.call(markdown) === before)
          (markdown as MarkdownRenderable & Rerun).rerunInlineHook(() => true);
      }
    },
    restore,
  };
}
