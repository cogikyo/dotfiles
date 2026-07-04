import fs from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { pathToFileURL } from "node:url";

type CookieRow = {
  path?: unknown;
  value?: unknown;
  expiry?: unknown;
  lastAccessed?: unknown;
  creationTime?: unknown;
};

type CandidateCookie = {
  value: string;
  path: string;
  expiry: number;
  lastAccessed: number;
  creationTime: number;
};

type CookieReadResult = {
  cookies: CandidateCookie[];
  rowCount: number;
};

type SQLiteDatabase = {
  query(sql: string): {
    all(): CookieRow[];
  };
  close(): void;
};

type SQLiteModule = {
  Database: new (
    file: string,
    options?: { readonly?: boolean; strict?: boolean },
  ) => SQLiteDatabase;
};

const COOKIE_SQL = `
  SELECT path, value, expiry, lastAccessed, creationTime
  FROM moz_cookies
  WHERE name = 'auth'
    AND (host = 'opencode.ai' OR host = '.opencode.ai')
  ORDER BY lastAccessed DESC, creationTime DESC
  LIMIT 100
`;

const EMPTY_RESULT: CookieReadResult = { cookies: [], rowCount: 0 };
const COOKIE_PATH_PROBES = ["/auth/status", "/workspace/wrk_probe/usage"];

export async function readOpencodeFirefoxAuthCookie() {
  const databases = await firefoxCookieDatabases();
  const results = await Promise.all(databases.map(readAuthCookies));
  return freshest(results.flatMap((result) => result.cookies))?.value;
}

async function readAuthCookies(databasePath: string): Promise<CookieReadResult> {
  try {
    // @ts-ignore bun:sqlite exists in the Bun plugin runtime; this tsconfig intentionally uses Node types only.
    const { Database } = (await import("bun:sqlite")) as SQLiteModule;
    const direct = queryCookies(Database, databasePath);
    if (direct && hasRows(direct)) return direct;

    const copied = await queryCopiedCookies(Database, databasePath);
    if (copied && hasRows(copied)) return copied;
    if (direct) return direct;
    if (copied) return copied;

    return queryCookies(Database, immutableURI(databasePath)) ?? EMPTY_RESULT;
  } catch {
    return EMPTY_RESULT;
  }
}

function hasRows(result: CookieReadResult | undefined) {
  return Boolean(result?.rowCount);
}

function queryCookies(Database: SQLiteModule["Database"], databasePath: string) {
  let db: SQLiteDatabase | undefined;
  try {
    db = new Database(databasePath, { readonly: true, strict: true });
    const rows = db.query(COOKIE_SQL).all();
    return resultFromRows(rows);
  } catch {
    return undefined;
  } finally {
    db?.close();
  }
}

async function queryCopiedCookies(Database: SQLiteModule["Database"], databasePath: string) {
  const tmp = await fs.mkdtemp(path.join(os.tmpdir(), "opencode-cookies-"));
  try {
    const copy = path.join(tmp, "cookies.sqlite");
    await fs.copyFile(databasePath, copy);
    for (const suffix of ["-wal", "-shm"]) {
      await fs.copyFile(`${databasePath}${suffix}`, `${copy}${suffix}`).catch(() => undefined);
    }
    return queryCookies(Database, copy);
  } catch {
    return undefined;
  } finally {
    await fs.rm(tmp, { recursive: true, force: true }).catch(() => undefined);
  }
}

function immutableURI(databasePath: string) {
  const uri = pathToFileURL(databasePath);
  uri.searchParams.set("mode", "ro");
  uri.searchParams.set("immutable", "1");
  return uri.toString();
}

function resultFromRows(rows: CookieRow[]): CookieReadResult {
  return {
    cookies: rows.map(cookieFromRow).filter((cookie): cookie is CandidateCookie => Boolean(cookie)),
    rowCount: rows.length,
  };
}

function cookieFromRow(row: CookieRow): CandidateCookie | undefined {
  if (typeof row.value !== "string" || row.value.length === 0) return undefined;
  if (!eligiblePath(string(row.path))) return undefined;

  const expiry = number(row.expiry) ?? 0;
  if (expiry > 0 && expiry <= Math.floor(Date.now() / 1000)) return undefined;

  return {
    value: row.value,
    path: string(row.path),
    expiry,
    lastAccessed: number(row.lastAccessed) ?? 0,
    creationTime: number(row.creationTime) ?? 0,
  };
}

function freshest(cookies: CandidateCookie[]) {
  return [...cookies].sort((a, b) => {
    const byAccess = b.lastAccessed - a.lastAccessed;
    if (byAccess !== 0) return byAccess;
    const byCreation = b.creationTime - a.creationTime;
    if (byCreation !== 0) return byCreation;
    return pathScore(b.path) - pathScore(a.path);
  })[0];
}

function eligiblePath(cookiePath: string) {
  if (!cookiePath) return true;
  if (cookiePath === "/") return true;
  if (cookiePath.startsWith("/workspace/")) return true;
  return COOKIE_PATH_PROBES.some((requestPath) => pathMatches(cookiePath, requestPath));
}

function pathMatches(cookiePath: string, requestPath: string) {
  if (!requestPath.startsWith("/")) return false;
  if (requestPath === cookiePath) return true;
  if (!requestPath.startsWith(cookiePath)) return false;
  if (cookiePath.endsWith("/")) return true;
  return requestPath[cookiePath.length] === "/";
}

function pathScore(cookiePath: string) {
  return cookiePath === "/" ? 1 : 0;
}

function number(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function string(value: unknown) {
  return typeof value === "string" ? value : "";
}

async function firefoxCookieDatabases() {
  const roots = [
    path.join(os.homedir(), ".mozilla", "firefox"),
    path.join(os.homedir(), ".config", "mozilla", "firefox"),
  ];

  const paths = new Set<string>();
  for (const root of roots) {
    for (const profile of await profilesFromIni(root)) paths.add(profile);
    for (const profile of await scannedProfiles(root)) paths.add(profile);
  }

  const databases: string[] = [];
  for (const profile of paths) {
    const database = path.join(profile, "cookies.sqlite");
    if (await exists(database)) databases.push(database);
  }
  return databases;
}

async function profilesFromIni(root: string) {
  const ini = await fs.readFile(path.join(root, "profiles.ini"), "utf8").catch(() => "");
  if (!ini) return [];

  const profiles: string[] = [];
  let section = "";
  let profilePath = "";
  let isRelative = true;

  const flush = () => {
    if (!section.startsWith("Profile") || !profilePath) return;
    const resolved = isRelative ? path.join(root, profilePath) : path.resolve(profilePath);
    profiles.push(resolved);
  };

  for (const raw of ini.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line.startsWith(";") || line.startsWith("#")) continue;
    if (line.startsWith("[") && line.endsWith("]")) {
      flush();
      section = line.slice(1, -1);
      profilePath = "";
      isRelative = true;
      continue;
    }

    const [key, value] = splitIni(line);
    switch (key) {
      case "Path":
        profilePath = value;
        break;
      case "IsRelative":
        isRelative = value !== "0";
        break;
    }
  }
  flush();

  return profiles;
}

async function scannedProfiles(root: string) {
  const entries = await fs.readdir(root, { withFileTypes: true }).catch(() => []);
  return entries
    .filter((entry) => entry.isDirectory())
    .map((entry) => path.join(root, entry.name));
}

function splitIni(line: string) {
  const split = line.indexOf("=");
  if (split === -1) return [line.trim(), ""] as const;
  return [line.slice(0, split).trim(), line.slice(split + 1).trim()] as const;
}

async function exists(file: string) {
  try {
    await fs.access(file);
    return true;
  } catch {
    return false;
  }
}
