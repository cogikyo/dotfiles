import { mkdir, readFile, rename, rm, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import type { z } from "zod";

/** Reads UTF-8 text, returning undefined only for a missing file. */
export async function readText(path: string): Promise<string | undefined> {
  try {
    return await readFile(path, "utf8");
  } catch (error) {
    if (error instanceof Error && "code" in error && error.code === "ENOENT") return undefined;
    throw error;
  }
}

/** Reads and validates JSON, returning undefined only for a missing file. */
export async function readJson<T>(path: string, schema: z.ZodType<T>): Promise<T | undefined> {
  const text = await readText(path);
  if (text === undefined) return undefined;
  return schema.parse(JSON.parse(text));
}

/** Atomically replaces a file, creating private parent directories and cleaning up on failure. */
export async function writeText(path: string, text: string, mode?: number) {
  await mkdir(dirname(path), { recursive: true, mode: 0o700 });
  const temp = `${path}.${process.pid}.${crypto.randomUUID()}.tmp`;
  try {
    await writeFile(temp, text, { mode });
    await rename(temp, path);
  } catch (error) {
    await rm(temp, { force: true }).catch(() => undefined);
    throw error;
  }
}

/** Atomically writes indented JSON with a trailing newline and optional file mode. */
export async function writeJson(path: string, value: unknown, mode?: number) {
  await writeText(path, `${JSON.stringify(value, null, 2)}\n`, mode);
}
