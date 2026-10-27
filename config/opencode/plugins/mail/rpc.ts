import { Rpc } from "@opencode/plugin/rpc";
import { Schema } from "effect";
import { Ledger } from "./mail.ts";

export const api = Rpc.define({
  id: "mail",
  methods: {
    ledger: {
      input: Schema.toStandardSchemaV1(Schema.Struct({ sessionID: Schema.String })),
      output: Schema.toStandardSchemaV1(Ledger),
    },
    parked: {
      input: Schema.toStandardSchemaV1(Schema.Struct({})),
      output: Schema.toStandardSchemaV1(Schema.Array(Schema.String)),
    },
  },
  events: {},
});
