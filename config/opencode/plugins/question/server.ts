import { Plugin } from "@opencode/plugin/effect";
import { SessionMessage } from "@opencode/schema/session-message";
import { Tool } from "@opencode/schema/tool";
import { Effect, Schema } from "effect";

const close = "You can now continue with the user's answers in mind.";

export const nudge =
  "At least one answer was typed rather than picked. A typed answer may ask something or voice doubt; if one does, answer it in text and end your turn without calling `question`, and act on it only once the user confirms. Otherwise, continue with the user's answers in mind.";

export const silent =
  "No text since the user's last message or answered question: write your answer or context in text before calling `question`. The popup covers the transcript, so the user needs that text to answer.";

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

export default Plugin.define({
  id: "question",
  effect: (ctx) =>
    Effect.gen(function* () {
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
