import { Schema } from "effect";
import { opened, parts } from "../mail/store.ts";

const Part = Schema.Union([
  Schema.Struct({ type: Schema.Literal("text"), text: Schema.String }),
  Schema.Struct({ type: Schema.Literal("tool"), status: Schema.String, input: Schema.Unknown }),
]);
type Part = typeof Part.Type;

const Parts = Schema.fromJsonString(Schema.Array(Schema.Unknown));

export type Entry =
  | { kind: "user"; id: string; seq: number; at: number; text: string; files: number }
  | { kind: "answer"; id: string; seq: number; at: number; text: string }
  | { kind: "assistant"; id: string; seq: number; at: number; finish?: string; parts: readonly Part[] }
  | { kind: "idle"; id: string; seq: number; at: number; outcome?: string };

type Row = {
  seq: number;
  id: string;
  type: string;
  at: number;
  text: string | null;
  files: number | null;
  finish: string | null;
  outcome: string | null;
  parts: string | null;
};

const rows = `
  SELECT m.seq, m.id, m.type, m.time_created AS at,
    json_extract(m.data, '$.text') AS text,
    json_array_length(m.data, '$.files') AS files,
    json_extract(m.data, '$.finish') AS finish,
    json_extract(m.data, '$.outcome') AS outcome,
    CASE m.type WHEN 'assistant' THEN (
      SELECT json_group_array(json_object(
        'type', json_extract(part.data, '$.type'),
        'text', json_extract(part.data, '$.text'),
        'status', json_extract(part.data, '$.state.status'),
        'input', json(part.data -> '$.state.input')))
      FROM (${parts("m")}) part
      WHERE json_extract(part.data, '$.type') = 'text'
        OR json_extract(part.data, '$.name') = 'workflow'
    ) END AS parts
  FROM session_message m
  WHERE m.session_id = ?1 AND (m.type IN ('user', 'assistant', 'idle') OR (m.type = 'synthetic'
    AND json_extract(m.data, '$.metadata.source') = 'question'
    AND json_extract(m.data, '$.metadata.state') = 'answered'))
  ORDER BY m.seq`;

function entry(row: Row): Entry {
  const base = { id: row.id, seq: row.seq, at: row.at };
  if (row.type === "user") return { ...base, kind: "user", text: row.text ?? "", files: row.files ?? 0 };
  if (row.type === "synthetic") return { ...base, kind: "answer", text: row.text ?? "" };
  if (row.type === "idle") return { ...base, kind: "idle", outcome: row.outcome ?? undefined };
  const content = Schema.decodeUnknownSync(Parts)(row.parts ?? "[]").filter(Schema.is(Part));
  return { ...base, kind: "assistant", finish: row.finish ?? undefined, parts: content };
}

export function history(sessionID: string, file: string): readonly Entry[] {
  return opened(file, (db) => db.query<Row, [string]>(rows).all(sessionID)).map(entry);
}
