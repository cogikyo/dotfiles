import { Database, type SQLQueryBindings } from "bun:sqlite";
import { errorMessage } from "../../shared/error.ts";
import { Query } from "./index.ts";

const store = "/home/cullyn/.local/share/opencode/opencode.db";
const excerpt = 800;
const brief = 160;
const readable = new Set(["session", "message", "part", "project", "todo", "sqlite_schema"]);

type Row = Record<string, unknown>;
type Op = { opcode: string; p2: number; p3: number };

const queries: Record<Query["query"], (db: Database, args: Query, limit: number) => Row[]> = {
  list: (db, args, limit) =>
    db
      .query<Row, SQLQueryBindings[]>(
        `SELECT id, parent_id AS parent, agent, title, directory, time_created AS created, time_updated AS updated
         FROM session
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
        limit,
        args.offset ?? 0,
      ),
  text: excerpts,
  reasoning: excerpts,
  tools: (db, args, limit) =>
    db
      .query<Row, SQLQueryBindings[]>(
        `SELECT id, time_created AS created,
                json_extract(data, '$.tool') AS tool,
                json_extract(data, '$.state.status') AS status,
                substr(json_extract(data, '$.state.input'), 1, ${brief}) AS input
         FROM part
         WHERE session_id = ?1
           AND json_extract(data, '$.type') = 'tool'
           AND (?2 IS NULL OR json_extract(data, '$.tool') = ?2)
         ORDER BY time_created, id
         LIMIT ?3 OFFSET ?4`,
      )
      .all(required(args.session, "session"), args.tool ?? null, limit, args.offset ?? 0),
  part: (db, args) => {
    const rows = db
      .query<Row, SQLQueryBindings[]>(
        `SELECT id, session_id AS session, message_id AS message, time_created AS created,
                json_extract(data, '$.type') AS type,
                json_extract(data, '$.tool') AS tool,
                json_extract(data, '$.state.status') AS status,
                substr(json_extract(data, '$.state.input'), 1, ${excerpt}) AS input,
                length(coalesce(json_extract(data, '$.state.output'), json_extract(data, '$.text'))) AS length,
                substr(coalesce(json_extract(data, '$.state.output'), json_extract(data, '$.text')), ?3 + 1, ${excerpt}) AS output,
                substr(json_extract(data, '$.state.error'), 1, ${excerpt}) AS error
         FROM part
         WHERE id = ?1 AND (?2 IS NULL OR session_id = ?2)
         LIMIT 1`,
      )
      .all(required(args.part, "part"), args.session ?? null, args.from ?? 0);
    if (rows.length === 0) throw new Error(`no part ${args.part}${args.session ? ` in session ${args.session}` : ""}`);
    return rows;
  },
  sql: (db, args, limit) => select(db, required(args.sql, "sql"), args.params ?? [], limit),
};

function excerpts(db: Database, args: Query, limit: number) {
  return db
    .query<Row, SQLQueryBindings[]>(
      `SELECT p.id, json_extract(m.data, '$.role') AS role, p.time_created AS created,
              length(json_extract(p.data, '$.text')) AS length,
              substr(json_extract(p.data, '$.text'), 1, ${excerpt}) AS text
       FROM part AS p
       JOIN message AS m ON m.id = p.message_id
       WHERE p.session_id = ?1
         AND json_extract(p.data, '$.type') = ?2
         AND length(json_extract(p.data, '$.text')) > 0
         AND (?3 IS NULL OR json_extract(m.data, '$.role') = ?3)
         AND (?4 = 1 OR coalesce(json_extract(p.data, '$.synthetic'), 0) = 0)
         AND (?5 IS NULL OR instr(lower(json_extract(p.data, '$.text')), lower(?5)) > 0)
       ORDER BY p.time_created, p.id
       LIMIT ?6 OFFSET ?7`,
    )
    .all(
      required(args.session, "session"),
      args.query,
      args.role ?? null,
      args.synthetic ? 1 : 0,
      args.match ?? null,
      limit,
      args.offset ?? 0,
    );
}

function select(db: Database, source: string, params: SQLQueryBindings[], limit: number) {
  const sql = source.trim().replace(/;\s*$/u, "");
  if (!/^(select|with)\b/iu.test(sql)) throw new Error("sql must be one SELECT or WITH statement");
  if (sql.includes(";")) throw new Error("sql must be one statement; bind values that contain `;` through params");

  const roots = new Map(
    db
      .query<{ rootpage: number; tbl_name: string }, []>(
        "SELECT rootpage, tbl_name FROM sqlite_schema WHERE rootpage > 0",
      )
      .all()
      .map((row) => [row.rootpage, row.tbl_name]),
  );
  roots.set(1, "sqlite_schema");
  for (const op of db.query<Op, SQLQueryBindings[]>(`EXPLAIN ${sql}`).all(...params)) {
    if (op.opcode === "OpenWrite") throw new Error("sql must not write");
    if (op.opcode === "VOpen") throw new Error("sql must not read virtual tables or table-valued functions");
    if (op.opcode !== "OpenRead" && op.opcode !== "ReopenIdx") continue;
    const table = op.p3 === 0 ? roots.get(op.p2) : undefined;
    if (table && readable.has(table)) continue;
    throw new Error(`sql may read only ${[...readable].join(", ")}; it reads ${table ?? `root page ${String(op.p2)}`}`);
  }

  const rows: Row[] = [];
  for (const row of db.query<Row, SQLQueryBindings[]>(sql).iterate(...params)) {
    rows.push(Object.fromEntries(Object.entries(row).map(([name, value]) => [name, clip(value)])));
    if (rows.length >= limit) break;
  }
  return rows;
}

function clip(value: unknown) {
  return typeof value === "string" && value.length > excerpt ? `${value.slice(0, excerpt)}…` : value;
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

function render(rows: Row[]) {
  if (rows.length === 0) return "no rows";
  return rows.map((row) => JSON.stringify(row, stamp)).join("\n");
}

function stamp(name: string, value: unknown) {
  return (name === "created" || name === "updated") && typeof value === "number"
    ? new Date(value).toISOString()
    : value;
}

function reply(input: unknown) {
  const args = Query.parse(input);
  const db = new Database(store, { readonly: true });
  try {
    db.run("PRAGMA query_only=ON");
    db.run("PRAGMA busy_timeout=200");
    return { output: render(queries[args.query](db, args, args.limit)) };
  } finally {
    db.close();
  }
}

if (import.meta.main) {
  try {
    process.stdout.write(JSON.stringify(reply(JSON.parse(await Bun.stdin.text()))));
  } catch (error) {
    process.stdout.write(JSON.stringify({ error: errorMessage(error) }));
  }
}
