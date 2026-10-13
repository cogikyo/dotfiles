import { Plugin } from "@opencode/plugin";
import { Schema } from "effect";
import { stat } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";

const Input = Schema.Struct({
  directory: Schema.Trim.check(Schema.isNonEmpty()).annotate({
    description: "Absolute path, `~`, or `~/...` of an existing directory.",
  }),
  title: Schema.Trim.check(Schema.isNonEmpty()).annotate({ description: "Session title." }),
  prompt: Schema.optional(
    Schema.Trim.check(Schema.isNonEmpty()).annotate({
      description:
        "First message, which starts a turn. Make it self-contained; the new session has none of this conversation. Omit to leave the session idle.",
    }),
  ),
  agent: Schema.optional(
    Schema.Trim.check(Schema.isNonEmpty()).annotate({
      description: "Primary agent ID. Omit to use the configured default primary agent.",
    }),
  ),
});

const Output = Schema.Struct({
  sessionID: Schema.String,
  title: Schema.String,
  directory: Schema.String,
  started: Schema.Boolean.annotate({ description: "Whether the prompt started a turn." }),
});

const description = [
  "Create a new top-level session in a directory, and optionally send its first message to start a turn.",
  "Call it only when the user asks in this conversation for a new session; that request is the approval, because this tool cannot ask for permission.",
  "The new session is independent of this one: it is not a lane, drive mode does not carry over, and it runs unattended until the user opens it.",
  "Only primary sessions can call it.",
].join(" ");

async function resolve(directory: string) {
  const home = homedir();
  const expanded =
    directory === "~" ? home : directory.startsWith("~/") ? path.join(home, directory.slice(2)) : directory;
  if (!path.isAbsolute(expanded)) throw new Error(`session_create: ${directory} is not an absolute path or ~ path`);
  const resolved = path.resolve(expanded);
  const info = await stat(resolved).catch(() => undefined);
  if (!info) throw new Error(`session_create: ${resolved} does not exist`);
  if (!info.isDirectory()) throw new Error(`session_create: ${resolved} is not a directory`);
  return resolved;
}

export default Plugin.define({
  id: "launch",
  async setup(ctx) {
    await ctx.tool.transform((editor) =>
      editor.add({
        name: "session_create",
        description,
        input: Input,
        output: Output,
        options: { namespace: "opencode", codemode: true, pinned: true },
        execute: async (input, context) => {
          const caller = await ctx.session.get({ sessionID: context.sessionID });
          if (caller.parentID)
            throw new Error(
              "session_create: only primary sessions can create sessions; child sessions never spawn work",
            );
          const directory = await resolve(input.directory);
          if (input.agent) {
            const { data: agent } = await ctx.agent.get({ agentID: input.agent, location: { directory } });
            if (agent.mode === "subagent" || agent.hidden)
              throw new Error(`session_create: ${input.agent} is not a selectable primary agent`);
          }
          const session = await ctx.session.create({
            title: input.title,
            agent: input.agent,
            location: { directory },
            metadata: { origin: context.sessionID },
          });
          if (input.prompt)
            await ctx.session.prompt({ sessionID: session.id, text: input.prompt }).catch((error: unknown) => {
              const reason = error instanceof Error ? error.message : String(error);
              throw new Error(`session_create: created session ${session.id}, but its prompt failed: ${reason}`);
            });
          const started = input.prompt !== undefined;
          return {
            output: { sessionID: session.id, title: input.title, directory, started },
            content: `Created session ${session.id} "${input.title}" in ${directory}${started ? " and started a turn" : ""}.`,
            metadata: { sessionID: session.id },
          };
        },
      }),
    );
  },
});
