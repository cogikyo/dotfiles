import fs from "node:fs/promises";
import path from "node:path";
import { usageCachePath, usageLockPath } from "./auth.ts";
import type { ProviderUsage } from "./types.ts";

export type CachedProviderUsage = {
  fetchedAt?: number;
  backoffUntil?: number;
  usage?: ProviderUsage;
  error?: string;
};

const LOCK_STALE_MS = 30_000;

export async function readProviderCache(providerID: string) {
  try {
    return JSON.parse(
      await fs.readFile(usageCachePath(providerID), "utf8"),
    ) as CachedProviderUsage;
  } catch {
    return {};
  }
}

export async function writeProviderCache(
  providerID: string,
  cache: CachedProviderUsage,
) {
  const cachePath = usageCachePath(providerID);
  const tempPath = `${cachePath}.${process.pid}.tmp`;

  await fs.mkdir(path.dirname(cachePath), { recursive: true, mode: 0o700 });
  await fs.writeFile(tempPath, JSON.stringify(cache), "utf8");
  await fs.rename(tempPath, cachePath);
}

export async function withProviderLock<T>(
  providerID: string,
  run: () => Promise<T>,
) {
  const release = await acquireLock(providerID);
  if (!release) return undefined;

  try {
    return await run();
  } finally {
    await release();
  }
}

async function acquireLock(providerID: string) {
  const lockPath = usageLockPath(providerID);
  await fs.mkdir(path.dirname(lockPath), { recursive: true, mode: 0o700 });

  const release = await createLock(lockPath);
  if (release) return release;

  if (await isStaleLock(lockPath)) {
    await fs.rm(lockPath, { force: true }).catch(() => undefined);
    return createLock(lockPath);
  }

  return undefined;
}

async function createLock(lockPath: string) {
  let handle: fs.FileHandle | undefined;
  try {
    handle = await fs.open(lockPath, "wx");
    await handle.writeFile(
      JSON.stringify({ pid: process.pid, createdAt: Date.now() }),
      "utf8",
    );
    await handle.close();

    let released = false;
    return async () => {
      if (released) return;
      released = true;
      await fs.rm(lockPath, { force: true }).catch(() => undefined);
    };
  } catch {
    await handle?.close().catch(() => undefined);
    return undefined;
  }
}

async function isStaleLock(lockPath: string) {
  try {
    const raw = await fs.readFile(lockPath, "utf8");
    const parsed = JSON.parse(raw) as { createdAt?: unknown };
    return (
      typeof parsed.createdAt === "number" &&
      Date.now() - parsed.createdAt > LOCK_STALE_MS
    );
  } catch {
    return false;
  }
}
