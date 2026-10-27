import { constants, Database, type SQLQueryBindings } from "bun:sqlite";
import type { Query } from "./server.ts";
import { retag, tag, title } from "./tags.ts";

const excerpt = 800;
const brief = 160;
const tables = ["session_v2", "session_message", "project"];
const store = `s_${crypto.randomUUID().replaceAll("-", "")}`;
const filters: Record<string, string> = { session_v2: "id", session_message: "session_id" };
const huge = new Set(["session_message"]);
const scans = new Set(["Rewind", "Last", "Count"]);
const seeks = new Set(["SeekGE", "SeekGT", "SeekLE", "SeekLT"]);
const bounds = new Set(["IdxGE", "IdxGT", "IdxLE", "IdxLT"]);
const harness = ["synthetic", "system", "skill", "compaction"];
const marker = "[hidden by refusals]";
const HUGE =
  "session_message is huge, so filter it by the indexed session_id, such as m.session_id = ? or m.session_id IN (SELECT id FROM session_v2 WHERE …)";

type Row = Record<string, unknown>;
type Op = { opcode: string; p1: number; p2: number; p3: number; p4: unknown };

class Slice {
  constructor(
    readonly text: string,
    readonly size: number,
    readonly from = 0,
  ) {}
}

const queries: Record<Query["query"], (db: Database, args: Query) => Row[]> = {
  list: (db, args) =>
    db
      .query<Row, SQLQueryBindings[]>(
        `SELECT id, parent_id AS parent, agent, title, directory, time_created AS created, time_updated AS updated
         FROM session_v2
         WHERE (parent_id = ?1 OR ?1 IS NULL AND (?2 = 1 OR parent_id IS NULL))
           AND (?3 IS NULL OR directory = ?3)
           AND (?4 IS NULL OR agent = ?4)
           AND (?5 IS NULL OR time_updated >= ?5)
           AND (?6 IS NULL OR time_updated <= ?6)
         ORDER BY time_updated DESC
         LIMIT ?7 OFFSET ?8`,
      )
      .all(
        args.parent ?? null,
        args.nested ? 1 : 0,
        args.directory ?? null,
        args.agent ?? null,
        time(args.since, "since"),
        time(args.until, "until"),
        args.limit,
        args.offset ?? 0,
      ),
  text: excerpts,
  reasoning: excerpts,
  tools: (db, args) =>
    db
      .query<Row, SQLQueryBindings[]>(
        `SELECT m.id || '#' || c.key AS id,
                coalesce(json_extract(c.value, '$.time.created'), m.time_created) AS created,
                json_extract(c.value, '$.name') AS tool,
                json_extract(c.value, '$.state.status') AS status,
                json_extract(c.value, '$.state.input') AS input
         FROM session_message AS m, json_each(m.data, '$.content') AS c
         WHERE m.session_id = ?1
           AND m.type = 'assistant'
           AND json_extract(c.value, '$.type') = 'tool'
           AND (?2 IS NULL OR json_extract(c.value, '$.name') = ?2)
         ORDER BY m.seq, c.key
         LIMIT ?3 OFFSET ?4`,
      )
      .all(required(args.session, "session"), args.tool ?? null, args.limit, args.offset ?? 0)
      .map((row) =>
        Object.assign(row, { input: typeof row.input === "string" ? new Slice(row.input, brief) : row.input }),
      ),
  part: (db, args) => {
    const address = required(args.part, "part");
    const [id, index] = address.split("#");
    if (index !== undefined && !/^\d+$/u.test(index))
      throw new Error("part must be a message ID, optionally followed by #<content index>");
    const row = db
      .query<Row & { item: string | null }, SQLQueryBindings[]>(
        `SELECT session_id AS session, type AS message, time_created AS created, json_extract(data, ?2) AS item
         FROM session_message
         WHERE id = ?1 AND (?3 IS NULL OR session_id = ?3)`,
      )
      .get(id ?? "", index === undefined ? "$" : `$.content[${index}]`, args.session ?? null);
    if (!row?.item) throw new Error(`no part ${address}${args.session ? ` in session ${args.session}` : ""}`);
    const { item, ...head } = row;
    return [{ id: address, ...head, ...detail(JSON.parse(item), index === undefined, args.from ?? 0) }];
  },
  sql: (db, args) => select(db, required(args.sql, "sql"), [...(args.params ?? [])], args.limit),
};

function excerpts(db: Database, args: Query) {
  return db
    .query<Row, SQLQueryBindings[]>(
      `WITH items AS (
         SELECT m.id || '#' || c.key AS id, m.seq, c.key AS position, 'assistant' AS role, m.type,
                m.time_created AS created, json_extract(c.value, '$.text') AS text
         FROM session_message AS m, json_each(m.data, '$.content') AS c
         WHERE m.session_id = ?1 AND m.type = 'assistant' AND json_extract(c.value, '$.type') = ?2
         UNION ALL
         SELECT id, seq, -1, iif(type = 'compaction', 'assistant', 'user'), type,
                time_created, coalesce(json_extract(data, '$.text'), json_extract(data, '$.summary'))
         FROM session_message
         WHERE ?2 = 'text' AND session_id = ?1
           AND (type = 'user' OR ?4 = 1 AND type IN (${harness.map((type) => `'${type}'`).join(", ")}))
       )
       SELECT id, role, type, created, length(text) AS length, text
       FROM items
       WHERE length(text) > 0
         AND (?3 IS NULL OR role = ?3)
         AND (?5 IS NULL OR instr(lower(text), lower(?5)) > 0)
       ORDER BY seq, position
       LIMIT ?6 OFFSET ?7`,
    )
    .all(
      required(args.session, "session"),
      args.query,
      args.role ?? null,
      args.synthetic ? 1 : 0,
      args.match ?? null,
      args.limit,
      args.offset ?? 0,
    );
}

type Content = { type: string; text?: string; name?: string; mime?: string };
type Item = {
  type?: string;
  name?: string;
  text?: string;
  summary?: string;
  content?: unknown[];
  state?: { status?: string; input?: unknown; content?: Content[]; error?: { message?: string } };
};

function detail(item: Item, whole: boolean, from: number) {
  if (whole && item.content)
    throw new Error(
      `assistant messages hold ${item.content.length} content items; address one as <message ID>#<index>`,
    );
  const output =
    item.type === "tool"
      ? (item.state?.content ?? [])
          .map((entry) => (entry.type === "text" ? (entry.text ?? "") : `[file ${entry.name ?? entry.mime}]`))
          .join("\n")
      : (item.text ?? item.summary ?? "");
  const { status, input, error } = item.state ?? {};
  return {
    type: whole ? undefined : item.type,
    tool: item.name,
    status,
    input: input === undefined ? undefined : JSON.stringify(input),
    length: output.length,
    output: new Slice(output, excerpt, from),
    error: error ? (error.message ?? JSON.stringify(error)) : undefined,
  };
}

function select(db: Database, source: string, params: SQLQueryBindings[], limit: number) {
  const sql = source.trim().replace(/;\s*$/u, "");
  if (!/^(select|with)\b/iu.test(sql)) throw new Error("sql must be one SELECT or WITH statement");
  if (sql.includes(";")) throw new Error("sql must be one statement; bind values that contain `;` through params");
  const ops = db.query<Op, SQLQueryBindings[]>(`EXPLAIN ${sql}`).all(...params);
  guardScans(ops, inspect(db, ops));

  const rows: Row[] = [];
  for (const row of db.query<Row, SQLQueryBindings[]>(sql).iterate(...params)) {
    rows.push(row);
    if (rows.length >= limit) break;
  }
  return rows;
}

function inspect(db: Database, ops: Op[]) {
  const attached = db
    .query<{ seq: number; name: string }, []>("PRAGMA database_list")
    .all()
    .find((row) => row.name === store)?.seq;
  const roots = new Map(
    db
      .query<{ rootpage: number; tbl_name: string }, []>(
        `SELECT rootpage, tbl_name FROM ${store}.sqlite_schema WHERE rootpage > 0 AND tbl_name IN (${list(tables)})`,
      )
      .all()
      .map((row) => [row.rootpage, row.tbl_name]),
  );
  const json = jsonTables(db);
  const opened = new Map<number, string>();
  for (const op of ops) {
    if (op.opcode === "OpenWrite") throw new Error("sql must not write");
    if (op.opcode === "VOpen" && !json.has(op.p4))
      throw new Error("sql may use only json_each and json_tree as virtual tables or table-valued functions");
    if (op.opcode !== "OpenRead" && op.opcode !== "ReopenIdx") continue;
    if (op.p3 === 0) continue;
    const table = op.p3 === attached ? roots.get(op.p2) : undefined;
    if (!table)
      throw new Error(
        `sql may read only ${[...tables, "sqlite_schema"].join(", ")}; it reads root page ${String(op.p2)}`,
      );
    if (huge.has(table)) opened.set(op.p1, table);
  }
  return opened;
}

function jsonTables(db: Database) {
  return new Set(
    ["json_each", "json_tree"].flatMap((name) =>
      db
        .query<Op, []>(`EXPLAIN SELECT * FROM ${name}('[]')`)
        .all()
        .filter((op) => op.opcode === "VOpen")
        .map((op) => op.p4),
    ),
  );
}

function guardScans(ops: Op[], cursors: Map<number, string>) {
  const seeked = new Set<number>();
  const bounded = new Set<number>();
  for (const op of ops) {
    const table = cursors.get(op.p1);
    if (!table) continue;
    if (scans.has(op.opcode)) throw new Error(`sql scans ${table} (${op.opcode}); ${HUGE}`);
    if (seeks.has(op.opcode)) seeked.add(op.p1);
    if (bounds.has(op.opcode)) bounded.add(op.p1);
  }
  for (const cursor of seeked) {
    if (!bounded.has(cursor)) throw new Error(`sql reads an open-ended range of ${cursors.get(cursor)}; ${HUGE}`);
  }
}

function clip(value: unknown, redact: (text: string) => string) {
  // Redact before clipping or paging, which could split a hidden title and prevent a match.
  if (value instanceof Slice) {
    const text = redact(value.text).slice(value.from);
    return text.length > value.size ? `${text.slice(0, value.size)}…` : text;
  }
  if (typeof value !== "string") return value;
  const text = redact(value);
  return text.length > excerpt ? `${text.slice(0, excerpt)}…` : text;
}

function required(value: string | undefined, name: string) {
  if (!value) throw new Error(`${name} is required for this query`);
  return value;
}

function time(value: string | undefined, name: string) {
  if (value === undefined) return null;
  const ms = Date.parse(value);
  if (Number.isNaN(ms)) throw new Error(`${name} must be an ISO 8601 time`);
  return ms;
}

function render(rows: Row[], hidden: number, redact: (text: string) => string) {
  const body =
    rows.length === 0
      ? "no rows"
      : rows
          .map((row) =>
            JSON.stringify(
              Object.fromEntries(Object.entries(row).map(([name, value]) => [name, clip(value, redact)])),
              stamp,
            ),
          )
          .join("\n");
  return hidden ? `${body}\n(${hidden} sessions hidden by the refusals flag)` : body;
}

function encode(text: string) {
  return JSON.stringify(text).slice(1, -1);
}

function redactor(titles: string[]) {
  const forms = [...new Set(titles.flatMap((text) => [text, encode(text), encode(encode(text))]))].toSorted(
    (a, b) => b.length - a.length,
  );
  return (text: string) => forms.reduce((out, form) => out.replaceAll(form, marker), text);
}

function specific(text: string) {
  return text.length >= 10 && text.trim().split(/\s+/u).length >= 2;
}

function list(values: Iterable<string>) {
  return [...values].map((value) => `'${value.replaceAll("'", "''")}'`).join(", ");
}

function shield(db: Database, path: string) {
  db.run(`ATTACH DATABASE ?1 AS ${store}`, [`file:${path}?mode=ro`]);
  db.run("BEGIN");
  for (const { sql } of db
    .query<{ sql: string }, []>(
      `SELECT sql FROM ${store}.sqlite_schema WHERE type IN ('table', 'index') AND sql IS NOT NULL AND tbl_name IN (${list(tables)})`,
    )
    .all())
    db.run(sql);
  const sessions = db
    .query<{ id: string; parent: string | null; fork: string | null; title: string | null; created: number }, []>(
      `SELECT id, parent_id AS parent, fork_session_id AS fork, title, time_created AS created FROM ${store}.session_v2`,
    )
    .all();
  const hidden = new Set(
    sessions.filter((session) => tag(session.title ?? undefined)?.flags.includes("refusals")).map(({ id }) => id),
  );
  const children = Map.groupBy(
    sessions.flatMap((session) => [session.parent, session.fork].map((source) => ({ source, id: session.id }))),
    (edge) => edge.source,
  );
  for (const id of hidden) for (const child of children.get(id) ?? []) hidden.add(child.id);
  for (const table of tables) {
    const column = filters[table];
    const where = column && hidden.size ? ` WHERE ${column} NOT IN (${list(hidden)})` : "";
    db.run(`CREATE TEMP VIEW ${table} AS SELECT * FROM ${store}.${table}${where}`);
  }
  const titles = sessions.flatMap((session) => {
    if (!hidden.has(session.id) || !session.title) return [];
    const plain = retag(session.title, { flags: [] });
    const generic = !specific(plain) || plain === title({ time: { created: session.created } });
    return generic ? [session.title].filter((text) => text !== plain) : [session.title, plain];
  });
  return { hidden, redact: redactor(titles) };
}

function refuse(db: Database, hidden: Set<string>, args: Query) {
  for (const id of [args.session, args.parent])
    if (id && hidden.has(id)) throw new Error(`session ${id} is hidden from agents by the refusals flag`);
  if (args.query !== "part" || !args.part) return;
  const owner = db
    .query<{ session: string }, [string]>(`SELECT session_id AS session FROM ${store}.session_message WHERE id = ?1`)
    .get(args.part.split("#")[0] ?? "");
  if (owner && hidden.has(owner.session))
    throw new Error(`part ${args.part} belongs to a session hidden from agents by the refusals flag`);
}

function stamp(name: string, value: unknown) {
  return (name === "created" || name === "updated") && typeof value === "number"
    ? new Date(value).toISOString()
    : value;
}

export function run(path: string, args: Query) {
  const db = new Database(":memory:", constants.SQLITE_OPEN_READWRITE | constants.SQLITE_OPEN_URI);
  try {
    db.run("PRAGMA busy_timeout=200");
    const { hidden, redact } = shield(db, path);
    try {
      refuse(db, hidden, args);
      db.run("PRAGMA query_only=ON");
      return render(queries[args.query](db, args), hidden.size, redact);
    } catch (error) {
      throw new Error(redact(error instanceof Error ? error.message : String(error)), { cause: error });
    }
  } finally {
    db.close();
  }
}

if (import.meta.main) {
  try {
    const path = process.argv[2];
    if (!path) throw new Error("usage: query.ts <opencode.db> < query.json");
    process.stdout.write(JSON.stringify({ output: run(path, JSON.parse(await Bun.stdin.text())) }));
  } catch (error) {
    process.stdout.write(JSON.stringify({ error: error instanceof Error ? error.message : String(error) }));
  }
}
