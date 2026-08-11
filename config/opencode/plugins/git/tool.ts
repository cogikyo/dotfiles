import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { spawn } from "node:child_process";

const id = "git-batch";
const git = "/usr/bin/git";
const maxOperations = 8;
const maxArguments = 8;
const maxArgumentBytes = 256;
const maxOperationOutputBytes = 512 * 1024;
const maxBatchOutputBytes = 2 * 1024 * 1024;
const operationTimeoutMS = 15_000;
const batchTimeoutMS = 60_000;
const printableASCII = /^[\x20-\x7e]+$/;
const refSegment = /^[A-Za-z0-9][A-Za-z0-9._-]*$/;

type Command = "merge-base" | "log" | "diff";
type Operation = { command: Command; argv: string[] };
type StopReason = "aborted" | "timeout" | "output_limit" | "spawn_error";
type Result = {
  operation: number;
  command: Command;
  argv: string[];
  stdout: string;
  stderr: string;
  exitCode: number | null;
  signal: NodeJS.Signals | null;
  error?: string;
};

const operationSchema = tool.schema.object({
  command: tool.schema.enum(["merge-base", "log", "diff"]),
  argv: tool.schema.array(tool.schema.string().max(maxArgumentBytes)).max(maxArguments),
}).strict();

const server: Plugin = async ({ directory, worktree }) => ({
  tool: {
    git_batch: tool({
      description: [
        "Run a bounded ordered batch of read-only Git inspection operations in the current session worktree without a shell.",
        "Supported argv forms are: merge-base [REV, REV]; log [--reverse, --format=%H %s, REV..REV]; diff [--stat, --find-renames, REV...REV], [--name-status, --find-renames, REV...REV], or [--check, REV...REV].",
        "Each result preserves stdout, stderr, and exitCode; ordinary nonzero exits do not stop later operations.",
      ].join(" "),
      args: {
        operations: tool.schema.array(operationSchema).min(1).max(maxOperations).describe("Ordered structured Git operations; no shell strings, cwd, global options, or result interpolation"),
      },
      async execute(args, context) {
        const operations = args.operations.map((operation, index) => validateOperation(operation, index));
        await context.ask({
          permission: "git_batch",
          patterns: [...new Set(operations.map((operation) => operation.command))],
          always: [],
          metadata: {},
        });

        const cwd = context.worktree || worktree || context.directory || directory;
        const deadline = Date.now() + batchTimeoutMS;
        const budget = { used: 0 };
        const results: Result[] = [];
        let stopped: { reason: StopReason; operation: number } | undefined;

        for (const [index, operation] of operations.entries()) {
          if (context.abort.aborted) {
            stopped = { reason: "aborted", operation: index + 1 };
            break;
          }

          const remainingMS = deadline - Date.now();
          if (remainingMS <= 0) {
            stopped = { reason: "timeout", operation: index + 1 };
            break;
          }

          const run = await runGit(cwd, operation, index, context.abort, Math.min(operationTimeoutMS, remainingMS), budget);
          results.push(run.result);
          if (run.stop) {
            stopped = { reason: run.stop, operation: index + 1 };
            break;
          }
        }

        return JSON.stringify({ results, ...(stopped ? { stopped } : {}) }, null, 2);
      },
    }),
  },
});

function validateOperation(operation: Operation, index: number): Operation {
  if (operation.argv.length > maxArguments) fail(index, "has too many arguments");
  for (const argument of operation.argv) {
    if (argument === "" || Buffer.byteLength(argument) > maxArgumentBytes || !printableASCII.test(argument)) {
      fail(index, "contains an empty, control-bearing, non-ASCII, or oversized argument");
    }
    if (argument === "--") fail(index, "must not contain --");
  }

  switch (operation.command) {
    case "merge-base":
      if (operation.argv.length !== 2 || !operation.argv.every(isRevision)) {
        fail(index, "must be merge-base [REV, REV]");
      }
      break;
    case "log":
      if (operation.argv.length !== 3 || operation.argv[0] !== "--reverse" || operation.argv[1] !== "--format=%H %s" || !isRange(operation.argv[2], "..")) {
        fail(index, "must be log [--reverse, --format=%H %s, REV..REV]");
      }
      break;
    case "diff":
      if (!isDiffArguments(operation.argv)) {
        fail(index, "must use an allowed diff stat, name-status, or check argv form");
      }
      break;
    default:
      fail(index, "uses an unsupported command");
  }

  return { command: operation.command, argv: [...operation.argv] };
}

function isDiffArguments(argv: string[]): boolean {
  if (argv.length === 2) return argv[0] === "--check" && isRange(argv[1], "...");
  if (argv.length !== 3 || argv[1] !== "--find-renames" || !isRange(argv[2], "...")) return false;
  return argv[0] === "--stat" || argv[0] === "--name-status";
}

function isRange(value: string, separator: ".." | "..."): boolean {
  const index = value.indexOf(separator);
  if (index <= 0 || index !== value.lastIndexOf(separator)) return false;
  if (separator === ".." && value.includes("...")) return false;
  return isRevision(value.slice(0, index)) && isRevision(value.slice(index + separator.length));
}

function isRevision(value: string): boolean {
  if (value.length === 0 || value.length > 200 || value.startsWith("-") || value.endsWith("/") || value.endsWith(".")) return false;
  if (value.includes("..") || value.includes("//") || value.includes("@{") || value.includes("\\")) return false;
  const segments = value.split("/");
  return segments.every((segment) => refSegment.test(segment) && !segment.endsWith(".lock"));
}

function fail(index: number, message: string): never {
  throw new Error(`git_batch operation ${index + 1} ${message}`);
}

function runGit(
  cwd: string,
  operation: Operation,
  index: number,
  abort: AbortSignal,
  timeoutMS: number,
  budget: { used: number },
): Promise<{ result: Result; stop?: StopReason }> {
  const callerArguments = operation.command === "diff"
    ? ["--no-ext-diff", "--no-textconv", ...operation.argv]
    : operation.argv;
  const gitArguments = [
    "--no-pager",
    "--no-replace-objects",
    "-c", "core.fsmonitor=false",
    "-c", "core.hooksPath=/dev/null",
    "-c", "core.pager=cat",
    "-c", "pager.log=false",
    "-c", "pager.diff=false",
    "-c", "color.ui=false",
    "-c", "diff.external=",
    "-c", "diff.trustExitCode=false",
    operation.command,
    ...callerArguments,
  ];

  return new Promise((resolve) => {
    const outputAllowance = Math.min(maxOperationOutputBytes, maxBatchOutputBytes - budget.used);
    let stdoutBytes = 0;
    let stderrBytes = 0;
    let operationBytes = 0;
    let stop: StopReason | undefined;
    let spawnError: Error | undefined;
    const stdout: Buffer[] = [];
    const stderr: Buffer[] = [];
    const child = spawn(git, gitArguments, {
      cwd,
      env: gitEnvironment,
      stdio: ["ignore", "pipe", "pipe"],
    });

    const stopChild = (reason: StopReason) => {
      if (stop) return;
      stop = reason;
      child.kill("SIGKILL");
    };
    const capture = (target: Buffer[], chunk: Buffer) => {
      if (stop === "output_limit") return;
      const available = Math.max(0, outputAllowance - operationBytes);
      const kept = Math.min(chunk.length, available);
      if (kept > 0) {
        target.push(chunk.subarray(0, kept));
        operationBytes += kept;
        budget.used += kept;
      }
      if (kept < chunk.length) stopChild("output_limit");
    };

    child.stdout.on("data", (chunk: Buffer) => {
      stdoutBytes += chunk.length;
      capture(stdout, chunk);
    });
    child.stderr.on("data", (chunk: Buffer) => {
      stderrBytes += chunk.length;
      capture(stderr, chunk);
    });
    child.once("error", (error) => {
      spawnError = error;
      stop = "spawn_error";
    });

    const onAbort = () => stopChild("aborted");
    abort.addEventListener("abort", onAbort, { once: true });
    if (abort.aborted) onAbort();
    const timer = setTimeout(() => stopChild("timeout"), timeoutMS);
    timer.unref();

    child.once("close", (exitCode, signal) => {
      clearTimeout(timer);
      abort.removeEventListener("abort", onAbort);
      resolve({
        result: {
          operation: index + 1,
          command: operation.command,
          argv: operation.argv,
          stdout: Buffer.concat(stdout).toString("utf8"),
          stderr: Buffer.concat(stderr).toString("utf8"),
          exitCode,
          signal,
          ...(spawnError ? { error: spawnError.message } : {}),
          ...(stop === "output_limit" ? { error: `output exceeded the bounded ${outputAllowance} byte allowance (${stdoutBytes + stderrBytes} bytes observed before termination)` } : {}),
        },
        ...(stop ? { stop } : {}),
      });
    });
  });
}

const gitEnvironment: NodeJS.ProcessEnv = {
  PATH: "/usr/bin:/bin",
  HOME: "/dev/null",
  XDG_CONFIG_HOME: "/dev/null",
  LANG: "C.UTF-8",
  LC_ALL: "C.UTF-8",
  GIT_CONFIG_NOSYSTEM: "1",
  GIT_CONFIG_SYSTEM: "/dev/null",
  GIT_CONFIG_GLOBAL: "/dev/null",
  GIT_ATTR_NOSYSTEM: "1",
  GIT_OPTIONAL_LOCKS: "0",
  GIT_NO_LAZY_FETCH: "1",
  GIT_TERMINAL_PROMPT: "0",
  GIT_ASKPASS: "/bin/false",
  GIT_PAGER: "cat",
  PAGER: "cat",
};

export default { id, server } satisfies PluginModule;
