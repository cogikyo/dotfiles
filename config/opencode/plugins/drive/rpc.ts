import { Rpc } from "@opencode/plugin/rpc";
import { Schema } from "effect";

export const Decision = Schema.Literals(["allow", "deny", "ask"]);
export type Decision = typeof Decision.Type;

export const Entry = Schema.Struct({
  time: Schema.String,
  root: Schema.String,
  session: Schema.String,
  agent: Schema.optional(Schema.String),
  action: Schema.String,
  resources: Schema.Array(Schema.String),
  decision: Decision,
  reason: Schema.String,
  model: Schema.String,
});
export type Entry = typeof Entry.Type;

const State = Schema.toStandardSchemaV1(Schema.Struct({ root: Schema.String, armed: Schema.Boolean }));
const session = { sessionID: Schema.String };

export const api = Rpc.define({
  id: "drive",
  methods: {
    state: { input: Schema.toStandardSchemaV1(Schema.Struct(session)), output: State },
    arm: { input: Schema.toStandardSchemaV1(Schema.Struct({ ...session, armed: Schema.Boolean })), output: State },
    journal: {
      input: Schema.toStandardSchemaV1(Schema.Struct({ ...session, limit: Schema.optional(Schema.Int) })),
      output: Schema.toStandardSchemaV1(Schema.Array(Entry)),
    },
  },
  events: { changed: { schema: State } },
});
