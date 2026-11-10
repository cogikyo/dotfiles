import { Plugin } from "@opencode/plugin/effect";
import { SessionMessage } from "@opencode/schema/session-message";
import { Tool } from "@opencode/schema/tool";
import { Effect, Schema } from "effect";
import { mailable } from "../mail/mail.ts";
import * as store from "../mail/store.ts";

const close = "You can now continue with the user's answers in mind.";

export const nudge =
  "At least one answer was typed rather than picked. A typed answer may ask something or voice doubt; if one does, answer it in text and end your turn without calling `question`, and act on it only once the user confirms. Otherwise, continue with the user's answers in mind.";

export const silent =
  "No visible text since the user's last message or answered question. Your thinking and narration fold into hidden thoughts the user never sees, and the popup covers the transcript: write the answer or context as a text reply, then call `question`.";

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

export function quiet(messages: readonly SessionMessage.Info[], current: SessionMessage.ID) {
  let spoke = false;
  for (const message of messages) {
    if (message.type === "user") spoke = false;
    if (message.type !== "assistant") continue;
    for (const part of message.content) {
      if (part.type === "text" && (message.id === current || part.text.trim() !== "")) spoke = true;
      if (part.type === "tool" && part.name === "question" && part.state.status === "completed") spoke = false;
    }
  }
  return !spoke;
}

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

      yield* ctx.tool.transform((editor) => {
        editor.update("question", (tool) => {
          const execute = tool.execute;
          tool.execute = (input, context) =>
            Effect.raceFirst(execute(input, context), withdraw(file, context.sessionID));
        });
      });

      yield* ctx.tool.hook("execute.before", (event) => {
        if (event.tool !== "question") return Effect.void;
        return ctx.session.context({ sessionID: event.sessionID }).pipe(
          Effect.map((messages) => quiet(messages, event.messageID)),
          Effect.catchCause((cause) =>
            Effect.logError("question: history read failed; allowing the call", cause).pipe(Effect.as(false)),
          ),
          Effect.flatMap((blocked) => (blocked ? Effect.fail(new Tool.Error({ message: silent })) : Effect.void)),
        );
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
