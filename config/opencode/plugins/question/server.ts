import { Plugin } from "@opencode/plugin/effect";
import { Tool } from "@opencode/schema/tool";
import { Cause, DateTime, Effect, Exit, Option, Predicate, Schema, Scope } from "effect";
import * as store from "../mail/store.ts";

const WITHDRAWN =
  "Popup withdrawn for waiting mail; the user did not answer it and nothing ran. Read the mail first: if it relays the user's decision here, act on it without asking again; otherwise do what the mail needs, then call this tool again if you still need the user.";

const QUEUED =
  "Queued. Now write your full reply as this turn's final text and end the turn; the form opens under it, and the answers arrive as the next message.";

const TWICE = "A form is already queued for this turn; ask every question in one call.";

const DISMISSED =
  "The user dismissed your questions without answering and will reply in chat; take their next message as the answer.";

const usage = [
  "- The call returns at once and queues the form. After it, write your full reply as this turn's final text and end the turn; the form opens under your reply, and the answers arrive as the next message",
  "- Ask every question in one call",
].join("\n");

const Answered = Schema.Struct({ answers: Schema.Array(Schema.Array(Schema.String)) });

type Execute = Tool.Info["execute"];
type Queued = {
  readonly ask: Execute;
  readonly input: { readonly questions: readonly unknown[] };
  readonly context: Tool.Context;
  readonly at: number;
};
type State = "answered" | "dismissed" | "withdrawn" | "failed";

const verbs: Record<State, string> = {
  answered: "Answered",
  dismissed: "Dismissed",
  withdrawn: "Withdrew",
  failed: "Dropped",
};

const due = (file: string, sessionID: string) =>
  Effect.try(() => store.due(sessionID, file)).pipe(
    Effect.catchCause((cause) => Effect.logError("question: withdraw check failed", cause).pipe(Effect.as(false))),
  );

const withdraw = (file: string, sessionID: string) =>
  Effect.gen(function* () {
    while (!(yield* due(file, sessionID))) yield* Effect.sleep(store.poll);
    return yield* new Tool.Error({ message: WITHDRAWN });
  });

const current = (entry: Queued, time: { readonly busy?: DateTime.Utc }) =>
  time.busy !== undefined && DateTime.toEpochMillis(time.busy) <= entry.at;

const dismissed = (cause: Cause.Cause<unknown>) =>
  cause.reasons.some(
    (reason) => Cause.isDieReason(reason) && Predicate.isTagged(reason.defect, "QuestionTool.CancelledError"),
  );

const asked = new Set<string>();
const queued = new Map<string, Queued>();

export default Plugin.define({
  id: "question",
  effect: (ctx) =>
    Effect.gen(function* () {
      const file = store.located(ctx.app.channel);
      const scope = yield* Scope.Scope;

      const queue = (ask: Execute, input: Queued["input"], context: Tool.Context) =>
        Effect.gen(function* () {
          const { time } = yield* ctx.session.get({ sessionID: context.sessionID }).pipe(Effect.orDie);
          const known = queued.get(context.sessionID);
          if (known && current(known, time)) return yield* new Tool.Error({ message: TWICE });
          queued.set(context.sessionID, { ask, input, context, at: Date.now() });
          return { content: QUEUED };
        });

      const open = ({ ask, input, context }: Queued) =>
        Effect.gen(function* () {
          const { sessionID, messageID, id: callID } = context;
          const count = input.questions.length;
          const note = (state: State, text: string, answers?: readonly (readonly string[])[]) => ({
            sessionID,
            text,
            description: `${verbs[state]} ${count} ${count === 1 ? "question" : "questions"}`,
            metadata: { source: "question", messageID, callID, state, ...(answers && { answers }) },
          });
          const exit = yield* Effect.exit(Effect.raceFirst(ask(input, context), withdraw(file, sessionID)));
          if (Exit.isSuccess(exit)) {
            const { content, metadata } = exit.value;
            const answers = Schema.is(Answered)(metadata) ? metadata.answers : [];
            yield* ctx.session.synthetic(
              note("answered", typeof content === "string" ? content : JSON.stringify(content), answers),
            );
            return;
          }
          if (Cause.hasInterruptsOnly(exit.cause)) return yield* Effect.interrupt;
          if (dismissed(exit.cause)) {
            yield* ctx.session.wait({ sessionID }).pipe(
              Effect.andThen(ctx.session.synthetic({ ...note("dismissed", DISMISSED), resume: false })),
              Effect.tapCause((cause) => Effect.logError("question: dismissal note failed", cause)),
              Effect.ignore,
              Effect.forkIn(scope),
            );
            return;
          }
          const error = Cause.findErrorOption(exit.cause);
          if (Option.isNone(error)) return yield* Effect.failCause(exit.cause);
          const { message } = error.value;
          yield* ctx.session.synthetic(note(message === WITHDRAWN ? "withdrawn" : "failed", message));
        });

      yield* ctx.tool.hook("execute.before", (event) =>
        Effect.sync(() => {
          if (event.tool === "question") asked.add(event.id);
        }),
      );

      yield* ctx.tool.transform((editor) => {
        // Keep this on the Effect API: the promise API drops form.created's location, hiding the popup.
        editor.update("question", (tool) => {
          const ask = tool.execute;
          tool.description = `${tool.description}\n${usage}`;
          delete tool.output;
          tool.execute = (input, context) =>
            asked.delete(context.id)
              ? queue(ask, input, context)
              : Effect.raceFirst(ask(input, context), withdraw(file, context.sessionID)).pipe(
                  Effect.map(({ content, metadata }) => ({ content, metadata })),
                );
        });
      });

      yield* ctx.session.hook("experimental.idle", ({ sessionID }) =>
        Effect.gen(function* () {
          const entry = queued.get(sessionID);
          if (!entry) return;
          queued.delete(sessionID);
          const { time } = yield* ctx.session.get({ sessionID }).pipe(Effect.orDie);
          if (current(entry, time)) yield* open(entry);
        }).pipe(
          Effect.catchCause((cause) =>
            Cause.hasInterruptsOnly(cause) ? Effect.interrupt : Effect.logError("question: queued form failed", cause),
          ),
        ),
      );
    }),
});
