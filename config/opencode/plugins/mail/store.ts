import { Database } from "bun:sqlite";
import { homedir } from "node:os";
import path from "node:path";
import { type Ledger, type Letter, mailable, read, source } from "./mail.ts";

const file = path.join(process.env.XDG_DATA_HOME || path.join(homedir(), ".local", "share"), "opencode", "opencode.db");

// Query host tables so unanswered asks do not depend on the TUI's loaded history window.
const received = `
  SELECT json_extract(data, '$.metadata') AS meta, 0 AS parked FROM session_message
  WHERE session_id = ?1 AND type = 'synthetic' AND json_extract(data, '$.metadata.source') = ?2
    AND json_extract(data, '$.metadata.to') = ?1
    AND EXISTS (SELECT 1 FROM session_v2 WHERE id = json_extract(data, '$.metadata.from'))
  UNION ALL
  SELECT json_extract(payload, '$.metadata'), 1 FROM session_inbox
  WHERE session_id = ?1 AND type = 'synthetic' AND json_extract(payload, '$.metadata.source') = ?2`;

const sent = `
  WITH calls AS (
    SELECT json_extract(part.value, '$.state.metadata') AS meta FROM session_message message, json_each(message.data, '$.content') part
    WHERE message.session_id = ?1 AND message.type = 'assistant' AND instr(message.data, '"name":"mail"') > 0
      AND json_extract(part.value, '$.type') = 'tool' AND json_extract(part.value, '$.name') = ?2
      AND json_extract(part.value, '$.state.status') = 'completed'
  )
  SELECT meta, EXISTS (SELECT 1 FROM session_inbox WHERE id = json_extract(meta, '$.id')) AS parked FROM calls
  WHERE json_extract(meta, '$.from') = ?1
    AND EXISTS (SELECT 1 FROM session_v2 WHERE id = json_extract(meta, '$.to'))
    AND (parked OR EXISTS (SELECT 1 FROM session_message WHERE id = json_extract(meta, '$.id')))`;

const parked = `
  SELECT DISTINCT session_id AS id FROM session_inbox
  WHERE type = 'synthetic' AND json_extract(payload, '$.metadata.source') = ?1`;

const roots = "SELECT id, parent_id AS parentID, title FROM session_v2 WHERE parent_id IS NULL";

const peers = "SELECT id, parent_id AS parentID, title FROM session_v2 WHERE id IN (SELECT value FROM json_each(?1))";

type Session = { id: string; parentID: string | null; title: string | null };

type Row = { meta: string | null; parked: number };

function query<T>(run: (db: Database) => T) {
  const db = new Database(file, { readonly: true });
  try {
    db.run("PRAGMA busy_timeout = 2000");
    return run(db);
  } finally {
    db.close();
  }
}

const letters = (rows: Row[]): Letter[] =>
  rows.flatMap((row) => {
    const mail = row.meta ? read(JSON.parse(row.meta)) : undefined;
    return mail ? [{ mail, parked: row.parked === 1 }] : [];
  });

export const ledger = (sessionID: string): Ledger =>
  query((db) => {
    const inbound = letters(db.query<Row, [string, string]>(received).all(sessionID, source));
    const outbound = letters(db.query<Row, [string, string]>(sent).all(sessionID, source));
    const ids = [...new Set([...inbound.map(({ mail }) => mail.from), ...outbound.map(({ mail }) => mail.to)])];
    const found = db.query<Session, [string]>(peers).all(JSON.stringify(ids));
    const reachable = new Set(found.filter(mailable).map((session) => session.id));
    return { received: inbound, sent: outbound, unreachable: ids.filter((id) => !reachable.has(id)) };
  });

export const inboxes = () =>
  query((db) =>
    db
      .query<{ id: string }, [string]>(parked)
      .all(source)
      .map((row) => row.id),
  );

export const sessions = () => query((db) => db.query<Session, []>(roots).all());
