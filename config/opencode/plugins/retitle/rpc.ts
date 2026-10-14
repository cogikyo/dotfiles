import { Rpc } from "@opencode/plugin/rpc";
import { Schema } from "effect";

export function pinned(title: string | undefined) {
  return !!title && /\p{L}/u.test(title) && !/\p{Ll}/u.test(title);
}

const input = { sessionID: Schema.String };
const Result = Schema.toStandardSchemaV1(Schema.Struct({ title: Schema.String }));

export const api = Rpc.define({
  id: "retitle",
  methods: {
    now: { input: Schema.toStandardSchemaV1(Schema.Struct(input)), output: Result },
    caps: { input: Schema.toStandardSchemaV1(Schema.Struct({ ...input, on: Schema.Boolean })), output: Result },
  },
  events: {},
});
