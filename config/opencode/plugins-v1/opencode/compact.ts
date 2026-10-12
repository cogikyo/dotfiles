import { tool, type Plugin, type PluginModule } from "@opencode-ai/plugin";
import type { SessionSummarizeData } from "@opencode-ai/sdk/v2";
import { appendFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { z } from "zod";
import { errorMessage } from "../shared/error.ts";
import { type Client, session, unwrap } from "../shared/opencode.ts";
import { COMPACTION_LIMIT, COMPACTION_NUDGES as TIERS, contextTokenTotal, formatTokens } from "../shared/session.ts";

const id = "opencode-compact";

const SUMMARY = [
  "Summary rules for this session:",
  "- Quote the latest real user message verbatim, and mark each of its questions or requests as answered or open.",
  '- Record whether each plan or workflow is approved, proposed, or pending, and quote the user words that approved it. A harness "Continue if you have next steps" message is not approval.',
  '- Keep user decisions in their own words and at their own firmness; keep hedges such as "maybe" or "idk" as hedges.',
  "- Keep standing user instructions verbatim, especially instructions about what to do after compaction.",
  "- List the remaining workflow steps with their status, and state the exact next step.",
  "- List uncommitted work by path, and mark each path as this session's work to commit, another session's work, or work to leave alone.",
  "- Drop items carried from the previous summary that are finished, superseded, or no longer relevant.",
].join("\n");

const CHILD =
  "- This is a delegate child session. Keep its brief's objective, write scope, exclusions, rules, and acceptance checks, as updated by later prompts.";

type Model = { providerID: string; modelID: string };

type Track = {
  tokens: number;
  model?: Model;
  floor: number;
  pending?: string;
  brief?: string;
};

type Entry = {
  sessionID: string;
  outcome: "approved" | "denied" | "compacted" | "error";
  tokens?: number;
  tier?: number;
  reason?: string;
  error?: string;
};

const Assistant = z.object({
  role: z.literal("assistant"),
  sessionID: z.string(),
  providerID: z.string(),
  modelID: z.string(),
  summary: z.boolean().optional(),
  error: z.unknown().optional(),
  time: z.object({ created: z.number() }),
  tokens: z.object({
    total: z.number().optional(),
    input: z.number(),
    output: z.number(),
    cache: z.object({ read: z.number(), write: z.number() }),
  }),
});

const server: Plugin = async ({ client }) => {
  const tracks = new Map<string, Track>();
  const primaries = new Map<string, Promise<boolean>>();
  const compacted = new Map<string, number>();

  const isPrimary = async (sessionID: string) => {
    let primary = primaries.get(sessionID);
    if (!primary) {
      primary = session(client, sessionID, { label: `${id} read session ${sessionID}` }).then((info) => !info.parentID);
      primary.catch(() => primaries.delete(sessionID));
      primaries.set(sessionID, primary);
    }
    try {
      return await primary;
    } catch (error) {
      console.error(`${id}: ${errorMessage(error)}`);
      return undefined;
    }
  };

  const track = (sessionID: string) => {
    let current = tracks.get(sessionID);
    if (!current) {
      current = { tokens: 0, floor: 0 };
      tracks.set(sessionID, current);
    }
    return current;
  };

  return {
    tool: { compact: compactTool(track) },
    "experimental.chat.system.transform": async (input, output) => {
      if (!input.sessionID) return;
      if (compacted.has(input.sessionID)) output.system.push(rehydrate(input.sessionID));
      const current = tracks.get(input.sessionID);
      if (!current || current.pending !== undefined || current.brief !== undefined) return;
      const tier = tierFor(current.tokens);
      if (tier < 0 || TIERS[tier] < current.floor) return;
      output.system.push(nudge(tier));
    },
    "experimental.session.compacting": async (input, output) => {
      output.context.push(...compaction(await isPrimary(input.sessionID), tracks.get(input.sessionID)));
    },
    event: async ({ event }) => {
      if (event.type === "message.updated") {
        const parsed = Assistant.safeParse(event.properties.info);
        if (!parsed.success) return;
        const info = parsed.data;
        if (info.summary || info.tokens.output <= 0) return;
        settle(compacted, info);
        if (!(await isPrimary(info.sessionID))) return;
        const current = track(info.sessionID);
        current.tokens = contextTokenTotal(info);
        current.model = { providerID: info.providerID, modelID: info.modelID };
        return;
      }
      if (event.type === "session.compacted") {
        const sessionID = event.properties.sessionID;
        const current = tracks.get(sessionID);
        if (current) tracks.set(sessionID, { tokens: 0, floor: 0, model: current.model });
        if (await isPrimary(sessionID)) compacted.set(sessionID, Date.now());
        return;
      }
      if (event.type === "session.deleted") {
        tracks.delete(event.properties.info.id);
        primaries.delete(event.properties.info.id);
        compacted.delete(event.properties.info.id);
        return;
      }
      if (event.type !== "session.idle") return;
      const sessionID = event.properties.sessionID;
      const current = tracks.get(sessionID);
      if (current?.pending === undefined) return;
      current.brief = current.pending;
      current.pending = undefined;
      void summarize(client, sessionID, current);
    },
  };
};

export default { id, server } satisfies PluginModule;

function compactTool(track: (sessionID: string) => Track) {
  return tool({
    description:
      "Ask the user to approve compacting this session. On approval, compaction runs after the current turn ends and the session then waits for the user.",
    args: {
      brief: tool.schema
        .string()
        .describe(
          "Handoff brief: current work, decisions made, open threads, lanes to keep, HEAD, uncommitted paths with their owner (this session or another), and the intended commit grouping.",
        ),
      reason: tool.schema.string().describe("Why now is a good time to compact."),
    },
    async execute(args, ctx) {
      const current = track(ctx.sessionID);
      const tier = tierFor(current.tokens);
      const entry = { sessionID: ctx.sessionID, tokens: current.tokens, tier: tier + 1, reason: args.reason };
      try {
        await ctx.ask({
          permission: "compact",
          patterns: ["*"],
          always: [],
          metadata: { reason: args.reason, tokens: current.tokens },
        });
      } catch (error) {
        current.floor = TIERS[tier + 1] ?? Infinity;
        await log({ ...entry, outcome: "denied" });
        throw error;
      }
      current.pending = args.brief;
      await log({ ...entry, outcome: "approved" });
      return "Compaction approved. It runs when this turn ends; finish the turn without starting new work.";
    },
  });
}

function settle(compacted: Map<string, number>, info: z.infer<typeof Assistant>) {
  const since = compacted.get(info.sessionID);
  if (since !== undefined && !info.error && info.time.created > since) compacted.delete(info.sessionID);
}

function compaction(primary: boolean | undefined, current: Track | undefined) {
  const context = [primary === false ? `${SUMMARY}\n${CHILD}` : SUMMARY];
  if (!current) return context;
  current.tokens = 0;
  const brief = current.brief ?? current.pending;
  current.brief = undefined;
  current.pending = undefined;
  if (brief === undefined) return context;
  context.push(
    `The agent wrote this handoff brief before requesting compaction. Preserve its content in the summary:\n\n${brief}`,
  );
  return context;
}

function tierFor(tokens: number) {
  return TIERS.findLastIndex((threshold) => tokens >= threshold);
}

function nudge(tier: number) {
  const label = `Context passed ${formatTokens(TIERS[tier])} tokens (compaction nudge ${tier + 1}/${TIERS.length}).`;
  if (tier + 1 < TIERS.length) {
    return `${label} At the end of this turn, judge whether now is a good time to compact; if so call compact.`;
  }
  return `${label} Native auto-compaction fires near ${formatTokens(COMPACTION_LIMIT)} and drops your handoff brief. Call compact at the end of this turn at a completed-work boundary; good times are right after a commit lands or after the user deferred the commit. In an armed drive run, commit finished work before compacting. Hold it while edits, lanes, or repairs are in flight, or when this turn delivers findings, a proposal, or open choices.`;
}

function rehydrate(sessionID: string) {
  return `This session (${sessionID}) was just compacted. Before other work, including a drive-mode prompt to continue, dispatch a scout/session leaf with this session ID. It checks the summary against the last turns before compaction and against git status, and returns stale or wrong claims, open user questions, lane states, and the exact files and line ranges worth re-reading; then read only those. The harness "Continue if you have next steps" message is not user approval.`;
}

async function summarize(client: Client, sessionID: string, current: Track) {
  try {
    if (!current.model) throw new Error(`no model recorded for session ${sessionID}`);
    const body: NonNullable<SessionSummarizeData["body"]> = { ...current.model, auto: false };
    await unwrap(client.session.summarize({ path: { id: sessionID }, body }), `${id} summarize session ${sessionID}`);
    await log({ sessionID, outcome: "compacted" });
  } catch (error) {
    current.brief = undefined;
    console.error(`${id}: ${errorMessage(error)}`);
    await log({ sessionID, outcome: "error", error: errorMessage(error) });
  }
}

async function log(entry: Entry) {
  const stateHome = process.env.XDG_STATE_HOME || path.join(process.env.HOME ?? "", ".local", "state");
  const dir = path.join(stateHome, "opencode");
  try {
    await mkdir(dir, { recursive: true });
    await appendFile(
      path.join(dir, "compact.jsonl"),
      `${JSON.stringify({ ts: new Date().toISOString(), ...entry })}\n`,
    );
  } catch (error) {
    console.error(`${id}: log failed: ${errorMessage(error)}`);
  }
}
