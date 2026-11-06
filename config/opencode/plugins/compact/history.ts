import { Result, Schema } from "effect";
import { opened, parts } from "../mail/store.ts";
import { Message } from "./digest.ts";

const messages = `
  SELECT message.seq, json_set(CASE message.type
    WHEN 'user' THEN json_remove(message.data, '$.files')
    WHEN 'assistant' THEN json_set(message.data, '$.content', (
      SELECT json_group_array(json(part.data)) FROM (${parts("message")}) part
      WHERE json_extract(part.data, '$.type') = 'text' OR json_extract(part.data, '$.name') = 'question'))
    ELSE message.data END, '$.id', message.id, '$.type', message.type) AS data
  FROM session_message message
  WHERE message.session_id = ?1 AND message.type IN ('user', 'assistant', 'idle') ORDER BY message.seq`;

const decode = Schema.decodeUnknownResult(Schema.fromJsonString(Message));

export type Skip = { seq: number; error: string };

/** Reads past compactions; ctx.session.context starts at the latest completed compaction. */
export function history(sessionID: string, file: string, skip: (row: Skip) => void): readonly Message[] {
  const rows = opened(file, (db) => db.query<{ seq: number; data: string }, [string]>(messages).all(sessionID));
  if (rows.length === 0) throw new Error(`no stored messages for ${sessionID} in ${file}`);
  return rows.flatMap((row) => {
    const decoded = decode(row.data);
    if (Result.isSuccess(decoded)) return [decoded.success];
    skip({ seq: row.seq, error: String(decoded.failure) });
    return [];
  });
}

export function checkpoint(context: ReadonlyArray<{ type: string; id: string; status?: string }>) {
  return context.findLast((message) => message.type === "compaction" && message.status === "completed")?.id;
}
