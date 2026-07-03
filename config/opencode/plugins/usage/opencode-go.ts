import type { ProviderAdapter, ProviderUsage } from "./types.ts";
import { readOpencodeFirefoxAuthCookie } from "./firefox.ts";

const id = "opencode-go";
const label = "OpenCode";
const ORIGIN = "https://opencode.ai";
const MIN_FETCH_INTERVAL_MS = 60_000;
const ERROR_BACKOFF_MS = 3 * 60_000;
const SIGN_IN_BACKOFF_MS = 60_000;
const RATE_LIMIT_BACKOFF_MS = 10 * 60_000;
const STALE_AFTER_MS = 2 * 60_000;
const DISCOVERY_TTL_MS = 20 * 60_000;
const FETCH_TIMEOUT_MS = 15_000;
const WORKSPACE_USAGE_ROUTES = ["usage", "go"];
const ASSET_MARKERS = [
  "lite.subscription.get",
  "queryLiteSubscription",
  "rollingUsage",
  "weeklyUsage",
  "monthlyUsage",
];

type StatusPayload = {
  account?: unknown;
  current?: unknown;
};

type ServerRef = {
  id: string;
  name: string;
  expiresAt: number;
};

type Discovery = {
  ref?: ServerRef;
  payload?: UsagePayload;
};

type UsagePayload = {
  mine?: boolean;
  useBalance?: boolean;
  region?: string[] | null;
  rollingUsage?: UsageWindowPayload | null;
  weeklyUsage?: UsageWindowPayload | null;
  monthlyUsage?: UsageWindowPayload | null;
} | null;

type UsageWindowPayload = {
  usagePercent?: unknown;
  resetInSec?: unknown;
};

type ServerResponse =
  | { kind: "ok"; payload: UsagePayload }
  | { kind: "sign-in" }
  | { kind: "stale-ref" }
  | { kind: "rate-limit" }
  | { kind: "unavailable" };

let discoveredRef: ServerRef | undefined;

function usage(
  windows: ProviderUsage["windows"],
  note?: string,
  noteKind?: ProviderUsage["noteKind"],
): ProviderUsage {
  return { id, label, windows, note, noteKind };
}

function note(note: string, kind: ProviderUsage["noteKind"] = "warn") {
  return usage([], note, kind);
}

function cookieHeader(auth: string) {
  return `auth=${auth}`;
}

async function withTimeout<T>(run: (signal: AbortSignal) => Promise<T>) {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS);
  try {
    return await run(controller.signal);
  } finally {
    clearTimeout(timeout);
  }
}

async function opencodeFetch(auth: string, path: string, init: RequestInit = {}) {
  const headers = new Headers(init.headers);
  headers.set("Cookie", cookieHeader(auth));
  headers.set("User-Agent", "opencode-usage");

  return withTimeout((signal) =>
    fetch(`${ORIGIN}${path}`, {
      ...init,
      signal,
      headers,
    }),
  );
}

async function isSignedIn(auth: string) {
  const response = await opencodeFetch(auth, "/auth/status", {
    headers: { Accept: "application/json" },
  });
  if (response.status === 401 || response.status === 403) return false;
  if (!response.ok) return undefined;

  const payload = (await response.json().catch(() => ({}))) as StatusPayload;
  return Boolean(payload.account);
}

async function workspaceID(auth: string) {
  const response = await opencodeFetch(auth, "/auth", {
    redirect: "manual",
  });
  const location = response.headers.get("location") ?? "";
  const pathname = locationPathname(location);
  if (pathname.startsWith("/auth/authorize")) return "sign-in" as const;

  const match = pathname.match(/^\/workspace\/([^/?#]+)/);
  if (match) return decodeURIComponent(match[1]);

  return undefined;
}

function locationPathname(location: string) {
  try {
    return new URL(location, ORIGIN).pathname;
  } catch {
    return location;
  }
}

async function discoverUsage(auth: string, workspace: string): Promise<Discovery> {
  if (discoveredRef && Date.now() < discoveredRef.expiresAt) return { ref: discoveredRef };

  let payload: UsagePayload | undefined;
  for (const route of WORKSPACE_USAGE_ROUTES) {
    const htmlResponse = await opencodeFetch(auth, `/workspace/${encodeURIComponent(workspace)}/${route}`, {
      headers: { Accept: "text/html" },
    });
    if (htmlResponse.status === 401 || htmlResponse.status === 403) continue;
    if (!htmlResponse.ok) continue;

    const html = await htmlResponse.text();
    payload ??= decodeUsagePayload(html);

    const ref = await discoverServerRefFromAssets(auth, html);
    if (!ref) continue;

    discoveredRef = { ...ref, expiresAt: Date.now() + DISCOVERY_TTL_MS };
    return { ref: discoveredRef, payload };
  }

  return { payload };
}

async function discoverServerRefFromAssets(auth: string, html: string) {
  const assets = javascriptAssets(html);
  for (const asset of assets) {
    const response = await opencodeFetch(auth, asset, {
      headers: { Accept: "application/javascript,text/javascript,*/*" },
    });
    if (!response.ok) continue;

    const source = await response.text();
    if (!ASSET_MARKERS.some((marker) => source.includes(marker))) continue;

    const ref = extractServerRef(source);
    if (ref) return ref;
  }

  return undefined;
}

function javascriptAssets(html: string) {
  const assets = new Set<string>();
  for (const match of html.matchAll(/(?:src|href)=["']([^"']+\.js(?:\?[^"']*)?)["']/g)) {
    const value = match[1];
    if (!value.includes("/_build/")) continue;
    assets.add(value.startsWith("http") ? new URL(value).pathname : value);
  }
  return [...assets];
}

function extractServerRef(source: string) {
  const calls = [...source.matchAll(/createServerReference\s*\(/g)].map((match) => match.index ?? 0);
  if (calls.length === 0) return undefined;

  const markers = ASSET_MARKERS.flatMap((marker) => indexesOf(source, marker));
  const scored = calls
    .map((index) => ({ index, distance: nearestDistance(index, markers) }))
    .sort((a, b) => a.distance - b.distance);

  for (const { index } of scored) {
    const call = callExpression(source, index) ?? source.slice(index, Math.min(source.length, index + 2_000));
    const strings = jsStrings(call);
    if (strings.length < 2) continue;

    const [serverID, name] = strings.slice(-2);
    if (!serverID || !name) continue;
    return { id: serverID, name };
  }

  return undefined;
}

function callExpression(source: string, start: number) {
  const open = source.indexOf("(", start);
  if (open === -1) return undefined;

  let depth = 0;
  let quote = "";
  let escaped = false;
  for (let i = open; i < source.length; i++) {
    const char = source[i];

    if (quote) {
      if (escaped) {
        escaped = false;
      } else if (char === "\\") {
        escaped = true;
      } else if (char === quote) {
        quote = "";
      }
      continue;
    }

    if (char === '"' || char === "'" || char === "`") {
      quote = char;
      continue;
    }
    if (char === "(") depth++;
    if (char === ")") depth--;
    if (depth === 0) return source.slice(start, i + 1);
  }

  return undefined;
}

function indexesOf(source: string, needle: string) {
  const indexes: number[] = [];
  let index = source.indexOf(needle);
  while (index !== -1) {
    indexes.push(index);
    index = source.indexOf(needle, index + needle.length);
  }
  return indexes;
}

function nearestDistance(index: number, markers: number[]) {
  if (markers.length === 0) return Number.MAX_SAFE_INTEGER;
  return Math.min(...markers.map((marker) => Math.abs(marker - index)));
}

function jsStrings(source: string) {
  const strings: string[] = [];
  for (const match of source.matchAll(/(["'])(?:\\.|(?!\1).)*\1/g)) {
    const raw = match[0];
    try {
      strings.push(raw.startsWith('"') ? JSON.parse(raw) : unquoteSingle(raw));
    } catch {
      continue;
    }
  }
  return strings;
}

function unquoteSingle(raw: string) {
  return raw
    .slice(1, -1)
    .replace(/\\'/g, "'")
    .replace(/\\\\/g, "\\");
}

async function queryUsage(auth: string, ref: ServerRef, workspace: string): Promise<ServerResponse> {
  const serverID = `${ref.id}#${ref.name}`;
  const attempts = [...new Set([encodeURIComponent(serverID), serverID])];

  for (const attempt of attempts) {
    const response = await opencodeFetch(auth, "/_server", {
      method: "POST",
      headers: {
        Accept: "application/json,text/javascript,*/*",
        "Content-Type": "application/json",
        "X-Server-Id": attempt,
        "X-Server-Instance": "opencode-usage:0",
      },
      body: JSON.stringify([workspace]),
    });

    if (response.status === 401 || response.status === 403) return { kind: "sign-in" };
    if (response.status === 404) continue;
    if (response.status === 429) return { kind: "rate-limit" };
    if (!response.ok || response.headers.has("x-error")) continue;

    const text = await response.text();
    const payload = decodeUsagePayload(text);
    if (payload !== undefined) return { kind: "ok", payload };
  }

  return { kind: "stale-ref" };
}

function decodeUsagePayload(text: string): UsagePayload | undefined {
  const parsed = parseJSON(text);
  const fromJSON = usagePayloadIn(parsed);
  if (fromJSON !== undefined) return fromJSON;

  return usagePayloadFromText(text);
}

function parseJSON(text: string) {
  try {
    return JSON.parse(text) as unknown;
  } catch {
    return undefined;
  }
}

function usagePayloadIn(value: unknown): UsagePayload | undefined {
  if (value === null) return null;
  if (Array.isArray(value)) {
    for (const item of value) {
      const payload = usagePayloadIn(item);
      if (payload !== undefined) return payload;
    }
    return undefined;
  }
  if (!value || typeof value !== "object") return undefined;

  const object = value as Exclude<UsagePayload, null>;
  if (object.rollingUsage || object.weeklyUsage || object.monthlyUsage) return object;

  for (const item of Object.values(value)) {
    const payload = usagePayloadIn(item);
    if (payload !== undefined) return payload;
  }
  return undefined;
}

function usagePayloadFromText(text: string): UsagePayload | undefined {
  if (!ASSET_MARKERS.some((marker) => text.includes(marker))) return undefined;

  const rollingUsage = textWindow(text, "rollingUsage");
  const weeklyUsage = textWindow(text, "weeklyUsage");
  const monthlyUsage = textWindow(text, "monthlyUsage");
  if (!rollingUsage && !weeklyUsage && !monthlyUsage) return undefined;

  return { rollingUsage, weeklyUsage, monthlyUsage };
}

function textWindow(text: string, key: string): UsageWindowPayload | null {
  const index = text.indexOf(key);
  if (index === -1) return null;

  const chunk = text.slice(index, index + 1_000);
  const usagePercent = numberAfter(chunk, "usagePercent");
  const resetInSec = numberAfter(chunk, "resetInSec");
  if (usagePercent === undefined && resetInSec === undefined) return null;

  return { usagePercent, resetInSec };
}

function numberAfter(text: string, key: string) {
  const match = text.match(new RegExp(`["']?${key}["']?\\s*[:=,]\\s*(-?\\d+(?:\\.\\d+)?)`));
  if (!match) return undefined;
  const value = Number(match[1]);
  return Number.isFinite(value) ? value : undefined;
}

function interpret(payload: UsagePayload) {
  if (!payload) return note("no usage", "warn");

  const windows = [
    window("H", payload.rollingUsage),
    window("W", payload.weeklyUsage),
    window("M", payload.monthlyUsage),
  ].filter((entry): entry is NonNullable<typeof entry> => Boolean(entry));

  if (windows.length === 0) return note("no usage", "warn");
  return usage(windows);
}

function window(label: string, payload: UsageWindowPayload | null | undefined) {
  if (!payload) return undefined;

  const usedPercent = percent(payload.usagePercent);
  if (usedPercent === undefined) return undefined;

  const resetInSec = numeric(payload.resetInSec);
  return {
    label,
    usedPercent,
    resetAt:
      resetInSec === undefined || resetInSec < 0
        ? undefined
        : new Date(Date.now() + resetInSec * 1000).toISOString(),
  };
}

function percent(value: unknown) {
  const numericValue = numeric(value);
  if (numericValue === undefined) return undefined;
  const expanded = numericValue > 0 && numericValue < 1 ? numericValue * 100 : numericValue;
  return Math.max(0, Math.min(100, expanded));
}

function numeric(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

async function load(): Promise<ProviderUsage> {
  const auth = await readOpencodeFirefoxAuthCookie();
  if (!auth) return note("sign in", "warn");

  const signedIn = await isSignedIn(auth).catch(() => undefined);
  if (signedIn === false) return note("sign in", "warn");
  if (signedIn === undefined) return note("unavailable", "error");

  const workspace = await workspaceID(auth).catch(() => undefined);
  if (workspace === "sign-in") return note("sign in", "warn");
  if (!workspace) return note("unavailable", "error");

  let discovery = await discoverUsage(auth, workspace).catch<Discovery>(() => ({}));
  if (!discovery.ref && discovery.payload !== undefined) return interpret(discovery.payload);
  if (!discovery.ref) return note("unavailable", "error");

  let response = await queryUsage(auth, discovery.ref, workspace).catch<ServerResponse>(() => ({
    kind: "unavailable",
  }));
  if (response.kind === "stale-ref") {
    discoveredRef = undefined;
    discovery = await discoverUsage(auth, workspace).catch<Discovery>(() => ({}));
    response = discovery.ref
      ? await queryUsage(auth, discovery.ref, workspace).catch<ServerResponse>(() => ({
          kind: "unavailable",
        }))
      : { kind: "unavailable" };
  }

  switch (response.kind) {
    case "ok":
      return interpret(response.payload);
    case "stale-ref":
    case "unavailable":
      if (discovery.payload !== undefined) return interpret(discovery.payload);
      return note("unavailable", "error");
    case "sign-in":
      return note("sign in", "warn");
    case "rate-limit":
      return note("429", "error");
    default:
      return note("unavailable", "error");
  }
}

export const opencodeGoUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["H", "W", "M"],
  poll: {
    minFetchIntervalMS: MIN_FETCH_INTERVAL_MS,
    errorBackoffMS: ERROR_BACKOFF_MS,
    warnBackoffMS: SIGN_IN_BACKOFF_MS,
    rateLimitBackoffMS: RATE_LIMIT_BACKOFF_MS,
    staleAfterMS: STALE_AFTER_MS,
  },
  load,
};
