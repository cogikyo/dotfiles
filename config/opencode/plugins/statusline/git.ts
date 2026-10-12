import { execFile } from "node:child_process";
import { promisify } from "node:util";

const run = promisify(execFile);

export type GitStatus = {
  branch: string;
  ahead: number;
  behind: number;
  staged: number;
  modified: number;
  untracked: number;
  deleted: number;
  stashed: number;
  renamed: number;
  conflicted: number;
};

export async function gitStatus(dir: string): Promise<GitStatus | undefined> {
  try {
    const { stdout } = await run(
      "git",
      ["-C", dir, "--no-optional-locks", "status", "--porcelain=v2", "--branch", "--show-stash"],
      { timeout: 1000, maxBuffer: 256 * 1024 },
    );
    return parse(stdout);
  } catch {
    return undefined;
  }
}

function parse(output: string): GitStatus {
  const status: GitStatus = {
    branch: "",
    ahead: 0,
    behind: 0,
    staged: 0,
    modified: 0,
    untracked: 0,
    deleted: 0,
    stashed: 0,
    renamed: 0,
    conflicted: 0,
  };
  for (const line of output.split("\n")) {
    if (line.startsWith("#")) header(status, line);
    else if (line.length >= 4) entry(status, line);
  }
  return status;
}

function header(status: GitStatus, line: string) {
  if (line.startsWith("# branch.head ")) {
    status.branch = line.slice("# branch.head ".length);
    return;
  }
  if (line.startsWith("# branch.ab ")) {
    const parts = line.split(/\s+/);
    status.ahead = Number.parseInt(parts[2]?.slice(1) ?? "0", 10) || 0;
    status.behind = Number.parseInt(parts[3]?.slice(1) ?? "0", 10) || 0;
    return;
  }
  if (line.startsWith("# stash ")) status.stashed = Number.parseInt(line.slice("# stash ".length), 10) || 0;
}

function entry(status: GitStatus, line: string) {
  const x = line[2];
  const y = line[3];
  switch (line[0]) {
    case "?":
      status.untracked++;
      return;
    case "u":
      status.conflicted++;
      return;
    case "1":
      if (x !== "." && x !== " ") status.staged++;
      if (y === "M" || y === "T") status.modified++;
      if (y === "D") status.deleted++;
      return;
    case "2":
      status.renamed++;
  }
}
