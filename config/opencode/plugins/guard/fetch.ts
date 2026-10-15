import dns from "node:dns";
import http from "node:http";
import https from "node:https";
import { BlockList, isIP } from "node:net";
import os from "node:os";
import zlib from "node:zlib";

export type Format = "text" | "markdown" | "html";
export type Request = { url: string; format: Format; timeout?: number };

const maxBytes = 5 * 1024 * 1024;
const maxRedirects = 5;
const defaultTimeout = 30;
const redirects = new Set([301, 302, 303, 307, 308]);
const browser =
  "Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko); compatible; OpenCode-User/1.0; +https://opencode.ai";
const accept: Record<Format, string> = {
  markdown: "text/markdown;q=1.0, text/x-markdown;q=0.9, text/plain;q=0.8, text/html;q=0.7, */*;q=0.1",
  text: "text/plain;q=1.0, text/markdown;q=0.9, text/html;q=0.8, */*;q=0.1",
  html: "text/html;q=1.0, application/xhtml+xml;q=0.9, text/plain;q=0.8, text/markdown;q=0.7, */*;q=0.1",
};

const global = new BlockList();
global.addSubnet("2000::", 3, "ipv6");
const reserved = new BlockList();
for (const [network, prefix] of [
  ["0.0.0.0", 8],
  ["10.0.0.0", 8],
  ["100.64.0.0", 10],
  ["127.0.0.0", 8],
  ["169.254.0.0", 16],
  ["172.16.0.0", 12],
  ["192.0.0.0", 24],
  ["192.0.2.0", 24],
  ["192.88.99.0", 24],
  ["192.168.0.0", 16],
  ["198.18.0.0", 15],
  ["198.51.100.0", 24],
  ["203.0.113.0", 24],
  ["224.0.0.0", 4],
  ["240.0.0.0", 4],
] as const) {
  reserved.addSubnet(network, prefix, "ipv4");
}
for (const [network, prefix] of [
  ["2001::", 23],
  ["2001:db8::", 32],
  ["2002::", 16],
  ["3fff::", 20],
] as const) {
  reserved.addSubnet(network, prefix, "ipv6");
}

function neighbours() {
  const prefixes = new BlockList();
  for (const address of Object.values(os.networkInterfaces()).flatMap((entries) => entries ?? [])) {
    if (!address.cidr) continue;
    const [network, prefix] = address.cidr.split("/");
    prefixes.addSubnet(network.replace(/%.*$/u, ""), Number(prefix), address.family === "IPv6" ? "ipv6" : "ipv4");
  }
  return prefixes;
}

function exposed(address: string) {
  const family = isIP(address);
  if (family === 0) return false;
  if (family === 4 && reserved.check(address, "ipv4")) return false;
  if (family === 6 && (!global.check(address, "ipv6") || reserved.check(address, "ipv6"))) return false;
  return !neighbours().check(address, family === 4 ? "ipv4" : "ipv6");
}

function host(url: URL) {
  return url.hostname.replace(/^\[(.*)\]$/u, "$1");
}

function target(raw: string) {
  const url = URL.parse(raw);
  if (!url) throw new Error("URL is not valid");
  if (url.protocol !== "http:" && url.protocol !== "https:") throw new Error("URL must use http:// or https://");
  if (url.username || url.password) throw new Error("URL must not carry credentials");
  if (url.port) throw new Error("URL must use the default port: 80 for http, 443 for https");
  const name = host(url);
  if (isIP(name) && !exposed(name)) throw new Error(`${name} is not a public address`);
  return url;
}

const lookup: NonNullable<http.RequestOptions["lookup"]> = (hostname, options, callback) => {
  dns.lookup(hostname, { all: true, verbatim: true }, (error, addresses) => {
    if (error) return callback(error, "", 4);
    const refused = addresses.find((entry) => !exposed(entry.address));
    if (refused) return callback(new Error(`${hostname} resolves to non-public address ${refused.address}`), "", 4);
    const usable = addresses.filter((entry) => !options.family || entry.family === options.family);
    if (usable.length === 0) return callback(new Error(`${hostname} has no usable public address`), "", 4);
    if (options.all)
      return (callback as unknown as (error: null, addresses: dns.LookupAddress[]) => void)(null, usable);
    callback(null, usable[0].address, usable[0].family);
  });
};

function get(url: URL, format: Format, agent: string, signal: AbortSignal) {
  return new Promise<http.IncomingMessage>((resolve, reject) => {
    const client = url.protocol === "https:" ? https : http;
    const request = client.request(
      {
        protocol: url.protocol,
        hostname: host(url),
        port: url.protocol === "https:" ? 443 : 80,
        path: `${url.pathname}${url.search}`,
        method: "GET",
        agent: false,
        lookup,
        signal,
        headers: { "User-Agent": agent, Accept: accept[format], "Accept-Language": "en-US,en;q=0.9" },
      },
      (response) => {
        const peer = response.socket?.remoteAddress;
        if (peer && exposed(peer)) return resolve(response);
        response.destroy();
        reject(new Error(`connected to non-public address ${peer ?? "unknown"}`));
      },
    );
    request.on("error", reject);
    request.end();
  });
}

function tooLarge() {
  return new Error(`Response too large (exceeds ${maxBytes} byte limit)`);
}

function collect(response: http.IncomingMessage, signal: AbortSignal) {
  const declared = Number(response.headers["content-length"]);
  return new Promise<Buffer>((resolve, reject) => {
    const fail = (error: unknown) => {
      signal.removeEventListener("abort", abort);
      response.destroy();
      reject(error);
    };
    const abort = () => fail(signal.reason);
    if (Number.isSafeInteger(declared) && declared > maxBytes) return fail(tooLarge());
    if (signal.aborted) return abort();
    signal.addEventListener("abort", abort, { once: true });
    const chunks: Buffer[] = [];
    let size = 0;
    response.on("data", (chunk: Buffer) => {
      size += chunk.byteLength;
      if (size > maxBytes) return fail(tooLarge());
      chunks.push(chunk);
    });
    response.on("error", fail);
    response.on("end", () => {
      signal.removeEventListener("abort", abort);
      resolve(Buffer.concat(chunks));
    });
  });
}

function inflate(body: Buffer, encoding: string | undefined) {
  const coding = encoding?.trim().toLowerCase();
  const options = { maxOutputLength: maxBytes };
  if (!coding || coding === "identity") return body;
  if (coding === "gzip" || coding === "x-gzip") return zlib.gunzipSync(body, options);
  if (coding === "deflate") return zlib.inflateSync(body, options);
  if (coding === "br") return zlib.brotliDecompressSync(body, options);
  throw new Error(`Unsupported content encoding: ${coding}`);
}

function textual(mime: string) {
  return (
    !mime ||
    mime.startsWith("text/") ||
    mime === "application/json" ||
    mime.endsWith("+json") ||
    mime === "application/xml" ||
    mime.endsWith("+xml") ||
    mime === "application/javascript" ||
    mime === "application/x-javascript"
  );
}

function convert(html: string, format: "text" | "markdown", signal: AbortSignal) {
  return new Promise<string>((resolve, reject) => {
    if (signal.aborted) return reject(signal.reason);
    const worker = new Worker(new URL("./convert.ts", import.meta.url).href);
    const finish = () => {
      signal.removeEventListener("abort", abort);
      worker.terminate();
    };
    const abort = () => {
      finish();
      reject(signal.reason);
    };
    signal.addEventListener("abort", abort, { once: true });
    worker.addEventListener("message", (event: MessageEvent<string>) => {
      finish();
      resolve(event.data);
    });
    worker.addEventListener("error", (event: ErrorEvent) => {
      finish();
      reject(new Error(`HTML conversion failed: ${event.message}`));
    });
    worker.postMessage({ html, format });
  });
}

export function check(raw: string) {
  target(raw);
}

async function follow(start: URL, format: Format, signal: AbortSignal) {
  let url = start;
  for (let hop = 0; hop <= maxRedirects; hop++) {
    let response = await get(url, format, browser, signal);
    if (response.statusCode === 403 && response.headers["cf-mitigated"] === "challenge") {
      response.destroy();
      response = await get(url, format, "opencode", signal);
    }
    const location = response.headers.location;
    if (!redirects.has(response.statusCode ?? 0) || !location) return response;
    response.destroy();
    url = target(new URL(location, url).href);
  }
  throw new Error(`Too many redirects (more than ${maxRedirects})`);
}

function accepted(response: http.IncomingMessage) {
  const status = response.statusCode ?? 0;
  const contentType = response.headers["content-type"] ?? "";
  const mime = contentType.split(";", 1)[0]?.trim().toLowerCase() ?? "";
  const refusal =
    status < 200 || status >= 300
      ? `HTTP status ${status}`
      : mime.startsWith("image/") && mime !== "image/svg+xml" && mime !== "image/vnd.fastbidsheet"
        ? `Unsupported fetched image content type: ${mime}`
        : textual(mime)
          ? undefined
          : `Unsupported fetched file content type: ${mime}`;
  if (!refusal) return contentType;
  response.destroy();
  throw new Error(refusal);
}

export async function fetchPublic(input: Request, signal: AbortSignal) {
  const deadline = AbortSignal.timeout((input.timeout ?? defaultTimeout) * 1000);
  const bound = AbortSignal.any([signal, deadline]);
  try {
    const response = await follow(target(input.url), input.format, bound);
    const contentType = accepted(response);
    const body = inflate(await collect(response, bound), response.headers["content-encoding"]);
    const content = new TextDecoder().decode(body);
    const output =
      contentType.includes("text/html") && input.format !== "html"
        ? await convert(content, input.format, bound)
        : content;
    return { url: input.url, contentType, format: input.format, output };
  } catch (error) {
    if (deadline.aborted && !signal.aborted) throw new Error("Request timed out", { cause: error });
    throw error;
  }
}
