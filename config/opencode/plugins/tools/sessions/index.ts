import { tool } from "@opencode-ai/plugin";
import { fileURLToPath } from "node:url";
import { z } from "zod";

const cap = 20;
const timeout = 30_000;

const shape = {
  query: z.enum(["list", "text", "reasoning", "tools", "part", "sql"]),
  session: z.string().optional().describe("Session ID; required for text, reasoning, and tools."),
  part: z.string().optional().describe("Part ID; required for part."),
  directory: z.string().optional().describe("list: exact session.directory."),
  agent: z.string().optional().describe("list: exact session.agent, such as `collab` or `scout/web`."),
  parent: z.string().optional().describe("list: children of this session ID."),
  nested: z.boolean().optional().describe("list: include child sessions at every level."),
  since: z.string().optional().describe("list: earliest time_updated, ISO 8601."),
  until: z.string().optional().describe("list: latest time_updated, ISO 8601."),
  role: z.enum(["user", "assistant"]).optional().describe("text, reasoning: message role."),
  synthetic: z.boolean().optional().describe("text: include harness-injected text; default false."),
  match: z.string().optional().describe("text, reasoning: case-insensitive substring."),
  tool: z.string().optional().describe("tools: exact tool name, such as `bash` or `skill`."),
  from: z.number().int().min(0).optional().describe("part: output character offset for paging long output."),
  sql: z.string().optional().describe("sql: one SELECT or WITH statement with `?` placeholders."),
  params: z
    .array(z.union([z.string(), z.number(), z.null()]))
    .optional()
    .describe("sql: placeholder values."),
  limit: z.number().int().min(1).max(cap).optional().describe("Row limit, at most 20; list defaults to 12."),
  offset: z.number().int().min(0).optional().describe("Rows to skip for paging."),
};

export const Query = z.object({ ...shape, limit: z.number().int().min(1).max(cap) });
export type Query = z.infer<typeof Query>;

const Reply = z.union([z.object({ output: z.string() }), z.object({ error: z.string() })]);

export const sessions = tool({
  description: [
    `Read-only queries over the local OpenCode session store. Each call returns at most ${cap} JSON rows; text is capped at 800 characters.`,
    "- list: sessions by directory, agent, parent, and time, newest first; top-level only unless `parent` or `nested` is set.",
    "- text, reasoning: one session's text or reasoning excerpts, oldest first.",
    "- tools: one session's tool calls with status and a short input excerpt, without outputs.",
    "- part: one part's input, output, and error excerpts; page long output with `from`.",
    "- sql: one read-only SELECT over session, message, part, project, and todo; bind every value through `params`.",
  ].join("\n"),
  args: shape,
  async execute(args, ctx) {
    await ctx.ask({ permission: "sessions", patterns: ["*"], always: ["*"], metadata: { query: args.query } });
    const query: Query = { ...args, limit: args.limit ?? (args.query === "list" ? 12 : cap) };
    const proc = Bun.spawn([process.execPath, fileURLToPath(new URL("./query.ts", import.meta.url))], {
      env: { ...process.env, BUN_BE_BUN: "1" },
      stdin: new Blob([JSON.stringify(query)]),
      stdout: "pipe",
      stderr: "pipe",
      killSignal: "SIGKILL",
      signal: AbortSignal.any([ctx.abort, AbortSignal.timeout(timeout)]),
    });
    const [stdout, stderr] = await Promise.all([
      new Response(proc.stdout).text(),
      new Response(proc.stderr).text(),
      proc.exited,
    ]);
    if (proc.signalCode) throw new Error(`sessions query killed after ${timeout / 1000}s or abort`);
    const parsed = Reply.safeParse(json(stdout));
    if (!parsed.success) throw new Error(`sessions query failed: ${stderr.trim().slice(-1000) || "no output"}`);
    if ("error" in parsed.data) throw new Error(parsed.data.error);
    return parsed.data.output;
  },
});

function json(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}
