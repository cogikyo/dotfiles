import { TextAttributes } from "@opentui/core";
import { fileURLToPath, pathToFileURL } from "node:url";

export type Span = {
  text: string;
  attributes?: number;
  link?: { url: string };
  code: boolean;
  math?: string;
  target?: string;
};

type Found = { start: number; end: number; url: string };
type Resolve = (token: string) => string | undefined;

const openers = String.raw`[\s([{<"'=*]`;
const opener = new RegExp(openers);
const starts = {
  bounded: new RegExp(`(?:^|${openers})(~?/|file:///)`),
  inner: new RegExp(`${openers}(~?/|file:///)`),
};
const trailer = /[\s.,;:!?)\]}>"'`*]/;
const cut = /["'`<>|]/;
const head = /^(?:~\/|\/)(?!\/)./;
const hex = /^(?=[a-f]*\d)[0-9a-f]{7,40}$/;

export function split<S extends Span>(
  spans: readonly S[],
  home: string,
  exists: (path: string) => boolean,
  commit: Resolve,
): S[] {
  const path: Resolve = (raw) => {
    if (raw.startsWith("file:")) {
      try {
        return exists(fileURLToPath(raw)) ? raw : undefined;
      } catch {
        return undefined;
      }
    }
    const file = raw.startsWith("~/") ? home + raw.slice(1) : raw;
    return exists(file) ? pathToFileURL(file).href : undefined;
  };
  return spans.flatMap((span, index) => {
    if (span.link || span.target !== undefined || span.math !== undefined) return [span];
    const found = span.code
      ? whole(span.text, path, commit)
      : prose(span.text, spans[index - 1]?.text.at(-1), spans[index + 1]?.text[0], path);
    return found.length === 0 ? [span] : pieces(span, found);
  });
}

function pieces<S extends Span>(span: S, found: Found[]): S[] {
  const out: S[] = [];
  let at = 0;
  for (const { start, end, url } of found) {
    if (start > at) out.push({ ...span, text: span.text.slice(at, start) });
    out.push({
      ...span,
      text: span.text.slice(start, end),
      link: { url },
      attributes: (span.attributes ?? 0) | TextAttributes.UNDERLINE,
    });
    at = end;
  }
  if (at < span.text.length) out.push({ ...span, text: span.text.slice(at) });
  return out;
}

function whole(text: string, path: Resolve, commit: Resolve): Found[] {
  const lead = /^`+\s?/.exec(text)?.[0].length ?? 0;
  const inner = text.slice(lead).replace(/\s?`+$/, "");
  if (/\s/.test(inner) || cut.test(inner)) return [];
  const found = token(inner, true, path) ?? sha(inner, commit);
  return found?.start === 0 ? [{ ...found, start: lead, end: lead + found.end }] : [];
}

function prose(text: string, before: string | undefined, after: string | undefined, resolve: Resolve): Found[] {
  return [...text.matchAll(/\S+/g)].flatMap((match) => {
    const start = match.index;
    const word = match[0];
    const bounded = start > 0 || before === undefined || opener.test(before);
    const continued = start + word.length === text.length && after !== undefined && !trailer.test(after);
    const found = continued ? undefined : token(word, bounded, resolve);
    return found ? [{ ...found, start: start + found.start, end: start + found.end }] : [];
  });
}

function sha(text: string, commit: Resolve): Found | undefined {
  const url = hex.test(text) ? commit(text) : undefined;
  return url ? { start: 0, end: text.length, url } : undefined;
}

function token(word: string, bounded: boolean, resolve: Resolve): Found | undefined {
  const match = (bounded ? starts.bounded : starts.inner).exec(word);
  if (!match) return;
  const start = match.index + match[0].length - match[1].length;
  const rest = word.slice(start);
  const file = match[1] === "file:///";
  if (word.slice(0, start).includes("://") || rest.slice(file ? "file://".length : 0).includes("://")) return;
  const stop = rest.search(cut);
  const path = trim(stop === -1 ? rest : rest.slice(0, stop));
  if (!file && !head.test(path)) return;
  const url = resolve(path);
  return url ? { start, end: start + path.length, url } : undefined;
}

function trim(raw: string) {
  return bare(bare(raw).replace(/:\d+(?::\d+)?$/, ""));
}

function bare(raw: string) {
  return raw.replace(/[.,;:!?)\]}*]+$/, "");
}
