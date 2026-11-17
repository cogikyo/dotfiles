const allowed = new Set([
  ":authority",
  ":method",
  ":path",
  ":scheme",
  "a-im",
  "accept-ch",
  "accept-charset",
  "accept-datetime",
  "accept-encoding",
  "accept-language",
  "accept-patch",
  "accept-ranges",
  "accept",
  "access-control-allow-credentials",
  "access-control-allow-headers",
  "access-control-allow-methods",
  "access-control-allow-origin",
  "access-control-expose-headers",
  "access-control-max-age",
  "access-control-request-headers",
  "access-control-request-method",
  "age",
  "allow",
  "alt-svc",
  "cache-control",
  "connection",
  "content-disposition",
  "content-encoding",
  "content-language",
  "content-location",
  "content-range",
  "content-security-policy",
  "content-type",
  "correlation-id",
  "date",
  "delta-base",
  "dnt",
  "expect-ct",
  "expect",
  "expires",
  "forwarded",
  "front-end-https",
  "host",
  "http2-settings",
  "if-modified-since",
  "if-range",
  "if-unmodified-source",
  "im",
  "last-modified",
  "link",
  "location",
  "max-forwards",
  "nel",
  "origin",
  "permissions-policy",
  "pragma",
  "preference-applied",
  "proxy-connection",
  "public-key-pins",
  "range",
  "referer",
  "refresh",
  "report-to",
  "retry-after",
  "save-data",
  "sec-gpc",
  "server",
  "status",
  "strict-transport-security",
  "te",
  "timing-allow-origin",
  "tk",
  "trailer",
  "transfer-encoding",
  "upgrade-insecure-requests",
  "upgrade",
  "user-agent",
  "vary",
  "via",
  "warning",
  "www-authenticate",
  "x-att-deviceid",
  "x-content-duration",
  "x-content-security-policy",
  "x-content-type-options",
  "x-correlation-id",
  "x-forwarded-for",
  "x-forwarded-host",
  "x-forwarded-proto",
  "x-frame-options",
  "x-http-method-override",
  "x-powered-by",
  "x-redirected-by",
  "x-request-id",
  "x-requested-with",
  "x-ua-compatible",
  "x-wap-profile",
  "x-webkit-csp",
  "x-xss-protection",
]);

const headerKeys = new Set(["requestHeaders", "responseHeaders"]);

function headers(value: unknown) {
  if (value === null || typeof value !== "object") return value;
  return Object.fromEntries(
    Object.entries(value).map(([name, inner]) => [
      name,
      name === "__truncated__" || allowed.has(name.toLowerCase()) ? inner : "<redacted>",
    ]),
  );
}

function scrub(value: unknown): unknown {
  if (Array.isArray(value)) return value.map(scrub);
  if (value === null || typeof value !== "object") return value;
  return Object.fromEntries(
    Object.entries(value).map(([key, inner]) => [key, headerKeys.has(key) ? headers(inner) : scrub(inner)]),
  );
}

export function redact(text: string) {
  const start = text.startsWith("[network]") ? text.indexOf("\n") + 1 : 0;
  const footer = text.indexOf("\n[+", start);
  const end = footer < 0 ? text.length : footer;
  const body = text.slice(start, end);
  if (!/^\s*[[{]/u.test(body)) {
    if (/"(?:request|response)Headers"/u.test(body)) throw new Error("browser: network output has unparsed headers");
    return text;
  }
  return text.slice(0, start) + JSON.stringify(scrub(JSON.parse(body)), null, 2) + text.slice(end);
}
