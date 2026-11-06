import { Rpc } from "@opencode/plugin/rpc";
import { Schema } from "effect";
import { Flow } from "./state.ts";

export const State = Schema.Struct({ flow: Schema.optionalKey(Flow) });
export type State = typeof State.Type;

const session = { sessionID: Schema.String };
const output = Schema.toStandardSchemaV1(State);

export const api = Rpc.define({
  id: "workflow",
  methods: {
    state: { input: Schema.toStandardSchemaV1(Schema.Struct(session)), output },
    pause: { input: Schema.toStandardSchemaV1(Schema.Struct(session)), output },
    link: {
      input: Schema.toStandardSchemaV1(Schema.Struct({ ...session, step: Schema.Int })),
      output,
    },
  },
  events: {
    changed: {
      schema: Schema.toStandardSchemaV1(
        Schema.Struct({
          root: Schema.String,
          ended: Schema.optionalKey(Schema.Record(Schema.String, Schema.Int)),
        }),
      ),
    },
  },
});
