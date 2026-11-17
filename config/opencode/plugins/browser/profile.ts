import { statSync } from "node:fs";
import { cp, mkdir, mkdtemp, readdir, readFile, rm, writeFile } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";

const root = path.join(homedir(), ".config/mozilla/firefox");
const template = path.join(root, "opencode-agent");
const runs = path.join(root, "opencode-agent-runs");

const skipped = new Set([
  "lock",
  ".parentlock",
  "logins.json",
  "logins-backup.json",
  "logins.db",
  "logins.db-shm",
  "logins.db-wal",
  "key4.db",
  "signedInUser.json",
  "weave",
  "synced-tabs.db",
  "synced-tabs.db-shm",
  "synced-tabs.db-wal",
  "sessionstore.jsonlz4",
  "sessionstore-backups",
  "sessionCheckpoints.json",
  "cache2",
  "startupCache",
  "thumbnails",
  "shader-cache",
  "safebrowsing",
  "crashes",
  "minidumps",
  "datareporting",
  "saved-telemetry-pings",
]);

export async function copy(sessionID: string) {
  if (!statSync(template, { throwIfNoEntry: false })?.isDirectory()) return undefined;
  await mkdir(runs, { recursive: true, mode: 0o700 });
  const dir = await mkdtemp(path.join(runs, `${sessionID}-`));
  try {
    await cp(template, dir, {
      recursive: true,
      filter: (source) => !skipped.has(path.relative(template, source)),
    });
    await writeFile(path.join(dir, ".firefox-devtools-mcp"), "");
  } catch (error) {
    await remove(dir);
    throw error;
  }
  return dir;
}

async function held(dir: string) {
  const pids = (await readdir("/proc")).filter((name) => /^\d+$/u.test(name));
  const commands = await Promise.all(pids.map((pid) => readFile(`/proc/${pid}/cmdline`, "utf8").catch(() => "")));
  return commands.some((command) => command.includes(dir));
}

async function released(dir: string, polls: number): Promise<void> {
  if (polls === 0 || !(await held(dir))) return;
  await Bun.sleep(250);
  return released(dir, polls - 1);
}

export async function remove(dir: string) {
  await released(dir, 120);
  await rm(dir, { recursive: true, force: true, maxRetries: 3 });
}
