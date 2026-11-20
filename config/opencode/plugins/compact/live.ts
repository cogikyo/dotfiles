import { execFile } from "node:child_process";
import { promisify } from "node:util";
import type { Plugin } from "@opencode/plugin";
import { children, fact } from "../workflow/progress.ts";
import { api as workflow } from "../workflow/rpc.ts";
import { status } from "../workflow/state.ts";

const run = promisify(execFile);
const LINES = 200;
const TIMEOUT = 2000;

type Context = Plugin.Context;
type Info = Awaited<ReturnType<Context["session"]["get"]>>;

export async function within<T>(ms: number, work: Promise<T>) {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const late = new Promise<never>((_, reject) => {
    timer = setTimeout(() => reject(new Error(`timed out after ${ms / 1000}s`)), ms);
  });
  try {
    return await Promise.race([work, late]);
  } finally {
    clearTimeout(timer);
  }
}

export async function tracked(ctx: Context, sessionID: string) {
  const { flow } = await ctx.rpc(workflow).state({ sessionID });
  if (!flow) return "No workflow is stored.";
  return status(flow, await children((id) => ctx.session.get({ sessionID: id }), flow.links));
}

async function list(ctx: Context, parentID: string, cursor?: string): Promise<Info[]> {
  const page = await ctx.session.list({ parentID, cursor, limit: 200 });
  return page.cursor.next ? [...page.data, ...(await list(ctx, parentID, page.cursor.next))] : page.data;
}

export async function lanes(ctx: Context, sessionID: string) {
  const found = await list(ctx, sessionID);
  const open = found.filter(
    (child): child is Info & { metadata: { lane: string } } =>
      typeof child.metadata?.lane === "string" && child.metadata.closed === undefined,
  );
  const closed = found.length - open.length;
  const rows = open.map(
    (child) =>
      `- \`${child.metadata.lane}\` (${child.agent ?? "no agent"}, \`${child.id}\`): ${fact(child)?.lane ?? "not started"}`,
  );
  return [
    rows.length ? rows.join("\n") : "No open lanes.",
    ...(closed ? [`${closed} closed lanes not listed.`] : []),
  ].join("\n");
}

export async function git(directory: string) {
  const call = (...args: string[]) =>
    run("git", ["-C", directory, "--no-optional-locks", ...args], { timeout: TIMEOUT, maxBuffer: 4 * 1024 * 1024 });
  const [changes, worktrees] = await Promise.allSettled([
    call("status", "--short", "--branch"),
    call("worktree", "list"),
  ]);
  if (changes.status === "rejected" && /not a git repository/iu.test(explain(changes.reason)))
    return `Not a Git repository: \`${directory}\`.`;
  return [
    `Directory: \`${directory}\``,
    block("git status --short --branch", changes),
    block("git worktree list", worktrees),
  ].join("\n\n");
}

function block(command: string, result: PromiseSettledResult<{ stdout: string }>) {
  if (result.status === "rejected") return `\`${command}\` failed: ${explain(result.reason)}`;
  const lines = result.value.stdout.trimEnd().split("\n");
  const shown = lines.slice(0, LINES);
  const cut = lines.length > LINES ? [`[omitted: ${lines.length - LINES} more lines]`] : [];
  return [`\`${command}\`:`, "```", ...shown, "```", ...cut].join("\n");
}

function explain(error: unknown) {
  if (typeof error === "object" && error !== null && "killed" in error && error.killed)
    return `timed out after ${TIMEOUT / 1000}s`;
  const stderr = typeof error === "object" && error !== null && "stderr" in error ? String(error.stderr) : "";
  const text = stderr.trim() || (error instanceof Error ? error.message : String(error));
  return text.split("\n").find((line) => line.trim()) ?? text;
}
