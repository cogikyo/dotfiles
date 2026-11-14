import { Rpc } from "@opencode/plugin/rpc";
import { Schema } from "effect";

const session = { sessionID: Schema.String };
const State = Schema.Struct({ root: Schema.String, on: Schema.Boolean });

export const api = Rpc.define({
  id: "drive",
  methods: {
    state: { input: Schema.toStandardSchemaV1(Schema.Struct(session)), output: Schema.toStandardSchemaV1(State) },
    set: {
      input: Schema.toStandardSchemaV1(Schema.Struct({ ...session, on: Schema.Boolean })),
      output: Schema.toStandardSchemaV1(State),
    },
  },
  events: { changed: { schema: Schema.toStandardSchemaV1(State) } },
});
