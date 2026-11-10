import { Rpc } from "@opencode/plugin/rpc";
import { Schema } from "effect";

const Focus = Schema.Struct({ tui: Schema.Int, session: Schema.String, focused: Schema.Boolean });
export type Focus = typeof Focus.Type;

const focus = Schema.toStandardSchemaV1(Focus);

export const api = Rpc.define({
  id: "hyprd",
  methods: { focus: { input: focus, output: focus } },
  events: { focus: { schema: focus } },
});
