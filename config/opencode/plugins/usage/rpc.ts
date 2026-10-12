import { Rpc } from "@opencode/schema/rpc";
import { Schema } from "effect";

const Window = Schema.Struct({
  label: Schema.String,
  usedPercent: Schema.optional(Schema.Finite),
  resetAt: Schema.optional(Schema.String),
});

export const Usage = Schema.Struct({
  id: Schema.String,
  label: Schema.String,
  windows: Schema.mutable(Schema.Array(Window)),
  note: Schema.optional(Schema.String),
  noteKind: Schema.optional(Schema.Literals(["info", "warn", "error"])),
  placeholders: Schema.optional(Schema.mutable(Schema.Array(Schema.String))),
});

export const Snapshot = Schema.Struct({ providers: Schema.Array(Usage) });
export type Snapshot = typeof Snapshot.Type;

const snapshot = Schema.toStandardSchemaV1(Snapshot);

export const Definition = Rpc.define({
  id: "usage",
  methods: {
    list: { input: Schema.toStandardSchemaV1(Schema.Struct({})), output: snapshot },
    refresh: {
      input: Schema.toStandardSchemaV1(Schema.Struct({ providerID: Schema.optional(Schema.String) })),
      output: snapshot,
    },
  },
  events: { updated: { schema: snapshot } },
});
