import { Rpc } from "@opencode/plugin/rpc";
import { Schema } from "effect";

export const api = Rpc.define({
  id: "question",
  methods: {
    typed: {
      input: Schema.toStandardSchemaV1(Schema.Struct({ sessionID: Schema.String })),
      output: Schema.toStandardSchemaV1(Schema.Struct({})),
    },
  },
  events: {},
});
