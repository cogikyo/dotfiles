import { Plugin } from "@opencode/plugin/effect";
import { SessionMessage } from "@opencode/schema/session-message";
import { Tool } from "@opencode/schema/tool";
import { Effect, Schema } from "effect";
import { mailable } from "../mail/mail.ts";
import * as store from "../mail/store.ts";

const close = "You can now continue with the user's answers in mind.";

export const nudge =
  "At least one answer was typed rather than picked. A typed answer may ask something or voice doubt; if one does, answer it in text and end your turn without calling `question`, and act on it only once the user confirms. Otherwise, continue with the user's answers in mind.";

const WITHDRAWN =
  "Withdrawn so waiting mail could reach you: read it and act on it, then re-ask this question if you still need the user.";

const withdrawAfter = 50 * 60_000;
const pollEvery = 60_000;

const Asked = Schema.Struct({
  questions: Schema.Array(Schema.Struct({ options: Schema.Array(Schema.Struct({ label: Schema.String })) })),
});

const Answered = Schema.Struct({ answers: Schema.Array(Schema.Array(Schema.String)) });

export function typed(input: unknown, metadata: unknown) {
  if (!Schema.is(Asked)(input) || !Schema.is(Answered)(metadata)) return false;
  return metadata.answers.some((answers, index) => {
    const labels = new Set(input.questions[index]?.options.map((option) => option.label));
    return answers.some((answer) => !labels.has(answer));
  });
}

export function reword(content: Tool.Result["content"]): Tool.Result["content"] {
  if (typeof content !== "string") return [...(content ?? []), { type: "text", text: nudge }];
  return content.includes(close) ? content.replace(close, nudge) : `${content} ${nudge}`;
}

// Opus 5.5 sends pre-tool prose as reasoning marked only by "narration" in its base64 signature.
// Anthropic says not to parse signatures; if the format changes, popups just lose the note.
const narrated = (signature: unknown) => {
  if (typeof signature !== "string") return false;
  try {
    return atob(signature.slice(0, 80)).includes("narration");
  } catch {
    return false;
  }
};

export function note(messages: readonly SessionMessage.Info[], messageID: SessionMessage.ID, callID: string) {
  const message = messages.findLast(({ id }) => id === messageID);
  if (message?.type !== "assistant") return undefined;
  const end = message.content.findIndex((part) => part.type === "tool" && part.id === callID);
  if (end < 0) return undefined;
  for (const part of message.content.slice(0, end).toReversed()) {
    if (part.type === "tool") return undefined;
    if (part.type === "reasoning" && narrated(part.native?.["signature"])) return part.text.trim() || undefined;
  }
  return undefined;
}

/** Show the note in the popup, but keep the original question in the model's result. */
export const annotated = (
  execute: Tool.Info["execute"],
  input: { readonly questions: readonly [{ readonly question: string }, ...unknown[]] },
  context: Tool.Context,
  text: string,
) => {
  const [first, ...rest] = input.questions;
  const shown = `${text}\n\n${first.question}`;
  return execute({ ...input, questions: [{ ...first, question: shown }, ...rest] }, context).pipe(
    Effect.map((result) =>
      typeof result.content === "string"
        ? { ...result, content: result.content.replace(`"${shown}"`, () => `"${first.question}"`) }
        : result,
    ),
  );
};

const due = (file: string, sessionID: string) =>
  Effect.try(() => {
    if (!store.inboxes(file).includes(sessionID)) return false;
    const session = store.sessions(file).find(({ id }) => id === sessionID);
    return session !== undefined && mailable(session);
  }).pipe(
    Effect.catchCause((cause) => Effect.logError("question: withdraw check failed", cause).pipe(Effect.as(false))),
  );

const withdraw = (file: string, sessionID: string) =>
  Effect.gen(function* () {
    yield* Effect.sleep(withdrawAfter);
    while (!(yield* due(file, sessionID))) yield* Effect.sleep(pollEvery);
    return yield* new Tool.Error({ message: WITHDRAWN });
  });

export default Plugin.define({
  id: "question",
  effect: (ctx) =>
    Effect.gen(function* () {
      const file = store.located(ctx.app.channel);

      const noted = (context: Tool.Context) =>
        ctx.session.context({ sessionID: context.sessionID }).pipe(
          Effect.map((messages) => note(messages, context.messageID, context.id)),
          Effect.catchCause((cause) =>
            Effect.logError("question: history read failed; asking without the note", cause).pipe(Effect.as(undefined)),
          ),
        );

      yield* ctx.tool.transform((editor) => {
        // Keep this on the Effect API: the promise API drops form.created's location, hiding the popup.
        editor.update("question", (tool) => {
          const execute = tool.execute;
          tool.execute = (input, context) =>
            Effect.gen(function* () {
              const text = yield* noted(context);
              return yield* Effect.raceFirst(
                text === undefined ? execute(input, context) : annotated(execute, input, context, text),
                withdraw(file, context.sessionID),
              );
            });
        });
      });

      yield* ctx.tool.hook("execute.after", (event) =>
        Effect.sync(() => {
          if (event.tool !== "question" || event.status !== "completed") return;
          if (!typed(event.input, event.result.metadata)) return;
          event.result = { ...event.result, content: reword(event.result.content) };
        }),
      );
    }),
});
