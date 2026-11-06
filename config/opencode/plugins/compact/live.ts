import { execFile } from "node:child_process";
import { promisify } from "node:util";
import type { Plugin } from "@opencode/plugin";
import { open } from "../mail/mail.ts";
import { ledger } from "../mail/store.ts";
import { fact } from "../workflow/progress.ts";
import type { Lane, Live } from "./digest.ts";

const run = promisify(execFile);

type Context = Plugin.Context;
type Info = Awaited<ReturnType<Context["session"]["get"]>>;

export async function probe(ctx: Context, sessionID: string, file: string): Promise<Live> {
  const session = await ctx.session.get({ sessionID });
  const [git, children] = await Promise.all([repo(session.location.directory), list(ctx, sessionID)]);
  return {
    at: Date.now(),
    git,
    lanes: children.flatMap(lane),
    mail: open(ledger(sessionID, file)).filter((item) => item.state === "owed" || item.state === "asked"),
  };
}

async function list(ctx: Context, parentID: string, cursor?: string): Promise<Info[]> {
  const page = await ctx.session.list({ parentID, cursor, limit: 200 });
  return page.cursor.next ? [...page.data, ...(await list(ctx, parentID, page.cursor.next))] : page.data;
}

function lane(session: Info): Lane[] {
  const name = session.metadata?.lane;
  if (typeof name !== "string" || session.metadata?.closed !== undefined) return [];
  const known = fact(session);
  const state = known?.lane === "running" ? "running" : (session.outcome ?? "not started");
  return [{ id: session.id, name, agent: session.agent, state }];
}

async function repo(dir: string): Promise<Live["git"]> {
  const git = (...args: string[]) =>
    run("git", ["-C", dir, "--no-optional-locks", ...args], { timeout: 2000, maxBuffer: 1024 * 1024 }).then(
      (out) => out.stdout,
      () => undefined,
    );
  const [branch, head, status] = await Promise.all([
    git("branch", "--show-current"),
    git("log", "-1", "--format=%h %s"),
    git("status", "--porcelain=v1", "--untracked-files=all"),
  ]);
  if (head === undefined || status === undefined) return undefined;
  return {
    branch: branch?.trim() || "(detached)",
    head: head.trim(),
    dirty: status.split("\n").filter(Boolean),
  };
}
