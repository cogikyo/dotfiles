import { existsSync, mkdirSync, renameSync, rmSync } from "node:fs";
import path from "node:path";
import { owners, readOnly } from "../policy.ts";
import type * as Policy from "../policy.ts";

export const rev = process.env.BASE ?? "cd7d96c1";
export const out = process.env.OUT ?? path.join(process.env.OPENCODE_SCRATCH ?? "/tmp/opencode", "guard-harness");
const repo = path.resolve(import.meta.dir, "../../../../..");
const files = ["config/opencode/plugins/guard", "config/opencode/plugins/gemini/agy.ts"];

export async function baseline(): Promise<typeof Policy> {
  const root = path.join(out, `base-${rev}`);
  const entry = path.join(root, "config/opencode/plugins/guard/policy.ts");
  if (!existsSync(entry)) {
    const partial = `${root}.partial-${process.pid}`;
    mkdirSync(partial, { recursive: true });
    const archive = Bun.spawn(["git", "-C", repo, "archive", rev, ...files], { stdout: "pipe" });
    const tar = Bun.spawn(["tar", "-x", "-C", partial], { stdin: archive.stdout });
    if ((await archive.exited) !== 0 || (await tar.exited) !== 0) {
      rmSync(partial, { recursive: true, force: true });
      throw new Error(`git archive ${rev} failed in ${repo}`);
    }
    renameSync(partial, root);
  }
  return import(entry);
}

export async function write(name: string, lines: string[]) {
  mkdirSync(out, { recursive: true });
  const file = path.join(out, name);
  await Bun.write(file, lines.length ? `${lines.join("\n")}\n` : "");
  return file;
}

export const flat = (text: string) => text.replaceAll("\n", "\\n");

export function kind(agent: string) {
  if (owners.has(agent)) return "owner";
  return readOnly(agent) ? "read-only" : "builder";
}

export function verdict(result: ReturnType<typeof Policy.inspect>, agent: string) {
  if (result.problems.length) return `deny ${result.problems[0]}`;
  return result.push && kind(agent) !== "read-only" ? `ask ${result.push}` : "allow";
}

export function rank(result: string) {
  if (result.startsWith("deny")) return 2;
  return result.startsWith("ask") ? 1 : 0;
}
