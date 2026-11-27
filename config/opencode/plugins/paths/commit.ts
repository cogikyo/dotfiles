import { execFile, spawn, spawnSync } from "node:child_process";
import { promisify } from "node:util";

export const scheme = "git-commit:";

export type Commit = { readonly repo: string; readonly sha: string };

const run = promisify(execFile);
const full = /^(?:[0-9a-f]{40}|[0-9a-f]{64})$/;
// A commits-panel row starts with its short hash once lazygit has loaded the log.
const drawn = /^\W?[0-9a-f]{7,} /m;

export function link({ repo, sha }: Commit) {
  return `${scheme}${sha}?${new URLSearchParams({ repo })}`;
}

/** Model-written Markdown can carry this scheme too, so only a full hex sha in the session's own repo reaches lazygit. */
export function parse(url: string, repo: string | undefined): Commit | undefined {
  const parsed = url.startsWith(scheme) ? URL.parse(url) : null;
  const sha = parsed?.pathname ?? "";
  return repo && parsed?.searchParams.get("repo") === repo && full.test(sha) ? { repo, sha } : undefined;
}

/** Resolves a hash token to a commit link in `repo`, caching misses too so redraws never rerun git. */
export function resolver(limit = 1024) {
  const seen = new Map<string, string | null>();
  return (repo: string, token: string) => {
    const key = `${repo}\0${token}`;
    let url = seen.get(key);
    if (url === undefined) {
      const out = spawnSync("git", ["-C", repo, "rev-parse", "--verify", "--quiet", `${token}^{commit}`], {
        encoding: "utf8",
        stdio: ["ignore", "pipe", "ignore"],
      });
      const sha = out.status === 0 ? out.stdout.trim() : "";
      url = full.test(sha) ? link({ repo, sha }) : null;
      if (seen.size >= limit) seen.delete(seen.keys().next().value!);
      seen.set(key, url);
    }
    return url ?? undefined;
  };
}

/**
 * Opens lazygit's log in a floating kitty window, then selects the commit by typing `/<sha>` and Enter through kitty remote control, since lazygit has no flag to start at a commit.
 * A jump that times out leaves the log open, and kitty removes its socket when the window closes.
 */
export async function show({ repo, sha }: Commit, fail: (message: string) => void, polls = 100) {
  if (!Bun.which("lazygit")) {
    fail("lazygit is not on PATH");
    return;
  }
  const socket = `unix:/tmp/opencode/lazygit-${process.pid}-${Date.now()}.sock`;
  const args = ["--title", "terminalfloat", "--listen-on", socket, "-o", "allow_remote_control=socket-only"];
  const child = spawn("kitty", [...args, "-e", "lazygit", "log", "-sm", "normal"], {
    cwd: repo,
    detached: true,
    stdio: "ignore",
  });
  let gone = false;
  child.once("error", (error) => {
    gone = true;
    fail(error.message);
  });
  child.once("exit", (code) => {
    gone = true;
    if (code) fail(`kitty exited ${code}`);
  });
  child.unref();
  const remote = (...rest: string[]) => run("kitty", ["@", "--to", socket, ...rest], { encoding: "utf8" });
  const jump = async (left: number): Promise<void> => {
    if (gone || left === 0) return;
    const screen = await remote("get-text").then(
      (out) => out.stdout,
      () => "",
    );
    if (drawn.test(screen)) return void (await remote("send-text", `/${sha}\r`).catch(() => undefined));
    await Bun.sleep(50);
    return jump(left - 1);
  };
  await jump(polls);
}
