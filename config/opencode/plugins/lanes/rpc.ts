import { Rpc } from "@opencode/plugin/rpc";
import { Schema } from "effect";

export const Outcome = Schema.Struct({ lane: Schema.String, skipped: Schema.optional(Schema.String) });
export type Outcome = typeof Outcome.Type;

export const api = Rpc.define({
  id: "lanes",
  methods: {
    close: {
      input: Schema.toStandardSchemaV1(Schema.Struct({ sessionID: Schema.String, lanes: Schema.Array(Schema.String) })),
      output: Schema.toStandardSchemaV1(Schema.Array(Outcome)),
    },
  },
  events: {},
});
