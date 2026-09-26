import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import type { SessionPromptAsyncData } from "@opencode-ai/sdk/v2";
import { z } from "zod";
import { arm, armed, disarm, isRoot } from "../shared/drive.ts";
import { errorMessage } from "../shared/error.ts";
import { type Client, messages, session, unwrap } from "../shared/opencode.ts";

const id = "opencode-drive";

const ARMED = "Drive mode armed; /drive off disarms.";
const DISARMED = "Drive mode disarmed; attended boundaries apply again.";
const QUESTION =
  "Drive mode is armed and the user is away. Decide this yourself, record the decision and your reasons, and continue.";
const CONTINUE = "Drive mode is armed; continue the approved workflow from the compaction summary.";

const RETRY_MS = 2_000;
const RETRY_MAX_MS = 60_000;

const Asked = z.object({ id: z.string(), sessionID: z.string() });
const Compacted = z.object({ sessionID: z.string() });

const server: Plugin = async ({ client }) => ({
  "command.execute.before": async (input, output) => {
    if (input.command !== "drive") return;
    const info = await session(client, input.sessionID, { label: `${id} read session ${input.sessionID}` });
    if (info.parentID) return;
    const text = input.arguments.trim();
    const mode = /^(on|off)(?:\s|$)/iu.exec(text)?.[1].toLowerCase();
    const task = mode ? text.slice(mode.length).trim() : text;
    const off = mode === "off";
    if (off) disarm(input.sessionID);
    else arm(input.sessionID);
    const files = output.parts.filter((part) => part.type !== "text");
    const parts: unknown[] = output.parts;
    parts.splice(
      0,
      parts.length,
      note(off ? DISARMED : ARMED),
      ...(task ? [{ type: "text", text: task }] : []),
      ...files,
    );
    await toast(client, off ? "Drive mode disarmed" : "Drive mode armed");
  },
  "tool.execute.before": async (input) => {
    if (input.tool !== "question") return;
    if (await armed(client, input.sessionID)) throw new Error(QUESTION);
  },
  event: async ({ event }) => {
    const type: string = event.type;
    try {
      if (type === "permission.asked") await approve(client, Asked.parse(event.properties));
      if (type === "session.compacted") await resume(client, Compacted.parse(event.properties).sessionID);
    } catch (error) {
      console.error(`${id}: ${errorMessage(error)}`);
    }
  },
});

export default { id, server } satisfies PluginModule;

function note(text: string) {
  return { type: "text", text, synthetic: true };
}

async function toast(client: Client, message: string, variant: "info" | "error" = "info") {
  try {
    await unwrap(client.tui.showToast({ body: { message, variant } }), `${id} show toast`);
  } catch (error) {
    console.error(`${id}: ${errorMessage(error)}`);
  }
}

async function approve(client: Client, ask: z.infer<typeof Asked>, delay = RETRY_MS): Promise<void> {
  try {
    if (!(await armed(client, ask.sessionID))) return;
    const result = await client.postSessionIdPermissionsPermissionId({
      path: { id: ask.sessionID, permissionID: ask.id },
      body: { response: "once" },
    });
    if (result.error === undefined) return;
    const status = result.response?.status ?? 0;
    const failure = `${id}: approve permission ${ask.id} failed with ${status}: ${errorMessage(result.error)}`;
    console.error(failure);
    if (status >= 400 && status < 500 && status !== 429) {
      if (status !== 404)
        await toast(client, `Drive mode could not approve a permission (${status}); it waits for you`, "error");
      return;
    }
  } catch (error) {
    console.error(`${id}: approve permission ${ask.id} failed: ${errorMessage(error)}`);
  }
  await Bun.sleep(delay);
  return approve(client, ask, Math.min(delay * 2, RETRY_MAX_MS));
}

async function resume(client: Client, sessionID: string) {
  if (!isRoot(sessionID)) return;
  const history = await messages(client, sessionID, { label: `${id} read messages ${sessionID}` });
  if (history.at(-1)?.info.role !== "assistant") return;
  const user = history.findLast((message) => message.info.role === "user")?.info;
  if (!user || user.role !== "user") throw new Error(`no user message to continue session ${sessionID}`);
  const { providerID, modelID, variant } = user.model;
  const body = {
    agent: user.agent,
    model: { providerID, modelID },
    ...(variant ? { variant } : {}),
    parts: [{ type: "text", text: CONTINUE }],
  } satisfies NonNullable<SessionPromptAsyncData["body"]>;
  await unwrap(client.session.promptAsync({ path: { id: sessionID }, body }), `${id} continue session ${sessionID}`);
}
