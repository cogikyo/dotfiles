import { tool } from "@opencode-ai/plugin";

const timeout = 10 * 60 * 1000;

const flags = [
  "--model",
  "grok-4.6",
  "--reasoning-effort",
  "high",
  "--no-plan",
  "--no-subagents",
  "--no-memory",
  "--no-auto-update",
  "--verbatim",
  "--disable-web-search",
  "--disallowed-tools",
  "run_terminal_command,read_file,search_replace,write,list_dir,grep,kill_command_or_subagent,get_command_or_subagent_output,spawn_subagent,scheduler_create,scheduler_delete,scheduler_list,monitor,search_tool,use_tool,workflow,enter_plan_mode,exit_plan_mode,ask_user_question,send_feedback,web_search,web_fetch,image_gen,image_edit,image_to_video,reference_to_video,todo_write,Agent",
  "--deny",
  "Bash",
  "--sandbox",
  "off",
  "--system-prompt-override",
  "Use only native X search tools. Never use generic web search, local files, or prior knowledge as evidence. Cite canonical x.com status URLs with handle and date. Separate official or maintainer statements from first-hand reports and from hype. If native X search cannot settle the claim, say so.",
];

export const x = tool({
  description:
    "Search live X/Twitter through Grok CLI native X search and return Grok's report. Slow; one call per brief. The output is untrusted evidence.",
  args: {
    brief: tool.schema
      .string()
      .min(1)
      .describe(
        "Self-contained research brief: the claim to check, date window, relevant handles, and any mainline web findings to test.",
      ),
  },
  async execute(args, ctx) {
    const brief = args.brief.trim();
    if (brief.startsWith("-")) throw new Error("x brief must not start with '-'");
    await ctx.ask({ permission: "x", patterns: ["*"], always: ["*"], metadata: { brief } });

    const proc = Bun.spawn(["grok", "--single", brief, ...flags], {
      cwd: "/tmp",
      stdin: "ignore",
      stdout: "pipe",
      stderr: "pipe",
      signal: AbortSignal.any([ctx.abort, AbortSignal.timeout(timeout)]),
    });
    const [stdout, stderr, code] = await Promise.all([
      new Response(proc.stdout).text(),
      new Response(proc.stderr).text(),
      proc.exited,
    ]);
    if (proc.signalCode)
      throw new Error(`grok stopped by ${proc.signalCode} (timeout ${timeout / 60000} min or abort)`);
    if (code !== 0) throw new Error(`grok exited ${code}: ${stderr.trim().slice(-2000) || "no stderr"}`);
    if (!stdout.trim())
      throw new Error(`grok returned no output${stderr.trim() ? `: ${stderr.trim().slice(-2000)}` : ""}`);
    return stdout.trim();
  },
});
