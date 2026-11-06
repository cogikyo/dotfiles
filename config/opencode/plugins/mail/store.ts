import { Database } from "bun:sqlite";
import { homedir } from "node:os";
import path from "node:path";
import { type Ledger, type Letter, mailable, read, source } from "./mail.ts";

const data = path.join(process.env.XDG_DATA_HOME || path.join(homedir(), ".local", "share"), "opencode");
const releases = new Set(["latest", "dev", "beta", "next", "prod"]);

/** Mirrors packages/cli/src/database-path.ts so mail reads the host's selected database. */
export function located(channel: string) {
  const unsplit = ["1", "true"].includes(process.env.OPENCODE_DISABLE_CHANNEL_DB ?? "");
  const name =
    process.env.OPENCODE_DB ??
    (releases.has(channel) || unsplit ? "opencode.db" : `opencode-${channel.replace(/[^a-zA-Z0-9._-]/gu, "-")}.db`);
  return name === ":memory:" ? name : path.resolve(data, name);
}

export function opened<T>(file: string, run: (db: Database) => T) {
  const db = new Database(file, { readonly: true });
  try {
    db.run("PRAGMA busy_timeout = 2000");
    return run(db);
  } finally {
    db.close();
  }
}

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

/** Mirrors packages/core/src/session/history.ts withParts(); split parts replace legacy inline content. */
export const parts = (message: string) => `
  SELECT value AS data FROM json_each(${message}.data, '$.content')
  WHERE NOT EXISTS (SELECT 1 FROM session_part WHERE message_id = ${message}.id)
  UNION ALL
  SELECT data FROM (SELECT data FROM session_part WHERE message_id = ${message}.id ORDER BY ordinal)`;

// The host treats dismissal or decline as shutdown and keeps the claim; it must release these claims upstream.
const dismissed = `
  EXISTS (SELECT 1 FROM session_message tail
    WHERE tail.session_id = session.id
      AND tail.seq = (SELECT max(seq) FROM session_message WHERE session_id = session.id)
      AND tail.type = 'assistant' AND json_extract(tail.data, '$.error.type') = 'aborted'
      AND session.time_busy <= json_extract(tail.data, '$.time.created')
      AND EXISTS (SELECT 1 FROM (${parts("tail")}) part
        WHERE json_extract(part.data, '$.state.error.type') = 'aborted'
          AND json_extract(part.data, '$.state.error.message')
            IN ('The user dismissed this question', 'The user declined this tool call')))`;

const overdue = `
  SELECT inbox.id, inbox.session_id AS sessionID, session.parent_id AS parentID, session.title,
    json_extract(inbox.payload, '$.text') AS text, json_extract(inbox.payload, '$.description') AS description,
    json_extract(inbox.payload, '$.metadata') AS meta, session.time_suspended IS NOT NULL AS stranded
  FROM session_inbox inbox JOIN session_v2 session ON session.id = inbox.session_id
  WHERE inbox.type = 'synthetic' AND json_extract(inbox.payload, '$.metadata.source') = ?1
    AND inbox.time_created <= ?2 AND (session.time_suspended IS NULL OR ${dismissed})
    AND NOT (session.idle_outcome IS 'failed' AND session.time_idle >= inbox.time_created)
  ORDER BY inbox.enqueued_seq`;

const roots = "SELECT id, parent_id AS parentID, title FROM session_v2 WHERE parent_id IS NULL";

const peers = "SELECT id, parent_id AS parentID, title FROM session_v2 WHERE id IN (SELECT value FROM json_each(?1))";

type Session = { id: string; parentID: string | null; title: string | null };

type Row = { meta: string | null; parked: number };

type Overdue = {
  id: string;
  sessionID: string;
  parentID: string | null;
  title: string | null;
  text: string;
  description: string | null;
  meta: string | null;
  stranded: number;
};

const letters = (rows: Row[]): Letter[] =>
  rows.flatMap((row) => {
    const mail = row.meta ? read(JSON.parse(row.meta)) : undefined;
    return mail ? [{ mail, parked: row.parked === 1 }] : [];
  });

export const ledger = (sessionID: string, file: string): Ledger =>
  opened(file, (db) => {
    const inbound = letters(db.query<Row, [string, string]>(received).all(sessionID, source));
    const outbound = letters(db.query<Row, [string, string]>(sent).all(sessionID, source));
    const ids = [...new Set([...inbound.map(({ mail }) => mail.from), ...outbound.map(({ mail }) => mail.to)])];
    const found = db.query<Session, [string]>(peers).all(JSON.stringify(ids));
    const reachable = new Set(found.filter(mailable).map((session) => session.id));
    return { received: inbound, sent: outbound, unreachable: ids.filter((id) => !reachable.has(id)) };
  });

export const inboxes = (file: string) =>
  opened(file, (db) =>
    db
      .query<{ id: string }, [string]>(parked)
      .all(source)
      .map((row) => row.id),
  );

export const stale = (before: number, file: string) =>
  opened(file, (db) => {
    const pending = db
      .query<Overdue, [string, number]>(overdue)
      .all(source, before)
      .flatMap(({ sessionID, id, text, description, meta, stranded, ...session }) => {
        const mail = meta ? read(JSON.parse(meta)) : undefined;
        return mail && mailable(session)
          ? [{ sessionID, id, text, description: description ?? undefined, mail, stranded: stranded === 1 }]
          : [];
      });
    return [...Map.groupBy(pending, (item) => item.sessionID).values()].map(([oldest]) => oldest);
  });

export const sessions = (file: string) => opened(file, (db) => db.query<Session, []>(roots).all());

const claim = `
  SELECT session.time_suspended IS NOT NULL AND NOT ${dismissed} AS held FROM session_v2 session WHERE session.id = ?1`;

export const held = (sessionID: string, file: string) =>
  opened(file, (db) => db.query<{ held: number }, [string]>(claim).get(sessionID)?.held === 1);
