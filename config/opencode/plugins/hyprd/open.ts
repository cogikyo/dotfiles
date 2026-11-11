import { Plugin } from "@opencode/plugin/tui";
import { Schema } from "effect";
import { message } from "../sessions/source.ts";
import { listen } from "./events.ts";
import { request, SOCKET } from "./notify.ts";

const OPEN_MS = 60_000;

const Navigate = Schema.Struct({
  event: Schema.Literal("navigate"),
  data: Schema.Struct({ kitty_pid: Schema.Int, kitty_pane_id: Schema.Int, session_id: Schema.String }),
});

const kitty = Number(process.env.KITTY_PID) || 0;
const pane = Number(process.env.KITTY_WINDOW_ID) || 0;

const LINE_MAX = 4096;

export type Desks = { readonly here: number; readonly of: ReadonlyMap<string, number> };

const missing = (error: unknown) => {
  const code = error instanceof Error && "code" in error ? error.code : undefined;
  return code === "ENOENT" || code === "ECONNREFUSED";
};

function batch(head: string, directories: readonly string[]) {
  const base = Buffer.byteLength(head);
  const batches: string[][] = [[]];
  const oversized: string[] = [];
  let size = base;
  for (const directory of directories) {
    const bytes = Buffer.byteLength(directory) + 1;
    if (base + bytes > LINE_MAX) {
      oversized.push(directory);
      continue;
    }
    if (size + bytes > LINE_MAX) {
      batches.push([]);
      size = base;
    }
    batches.at(-1)!.push(directory);
    size += bytes;
  }
  return { batches, oversized };
}

export async function desks(
  directories: readonly string[],
): Promise<(Desks & { readonly oversized: readonly string[] }) | undefined> {
  if (!kitty) return undefined;
  const head = `workspaces ${kitty}`;
  const { batches, oversized } = batch(head, directories);
  let replies: string[];
  try {
    replies = await Promise.all(batches.map((dirs) => request(SOCKET, `${head} ${dirs.join("\t")}`)));
  } catch (error) {
    if (missing(error)) return undefined;
    throw error;
  }
  const of = new Map<string, number>();
  let here = 0;
  for (const [at, dirs] of batches.entries()) {
    const reply = replies[at].trim();
    const fields = reply.split(" ").map(Number);
    if (fields.length !== dirs.length + 1 || !fields.every(Number.isInteger))
      throw new Error(reply.replace(/^error: /, "") || "hyprd did not answer");
    here = fields[0];
    dirs.forEach((directory, index) => of.set(directory, fields[index + 1]));
  }
  return { here, of, oversized };
}

export async function show(ui: Plugin.Context["ui"], sessionID: string, directory?: string) {
  const here = () => ui.router.navigate({ type: "session", sessionID });
  if (!kitty || !pane || !directory) return here();
  let reply: string;
  try {
    reply = (await request(SOCKET, `open ${kitty} ${pane} ${sessionID} ${directory}`, OPEN_MS)).trim();
  } catch (error) {
    if (missing(error)) return here();
    reply = `error: ${message(error)}`;
  }
  if (reply === "here") return here();
  if (reply === "focused" || reply.startsWith("opened ")) return;
  const reason = reply.replace(/^error: /, "") || "hyprd did not answer";
  ui.toast.show({ message: `Failed to open session: ${reason}`, variant: "error" });
}

export default Plugin.define({
  id: "hyprd-open",
  setup({ ui }) {
    if (!kitty || !pane) return;
    return listen((value) => {
      if (!Schema.is(Navigate)(value)) return;
      const { kitty_pid, kitty_pane_id, session_id } = value.data;
      if (kitty_pid !== kitty || kitty_pane_id !== pane) return;
      const current = ui.router.current();
      if (current.type === "session" && current.sessionID === session_id) return;
      ui.router.navigate({ type: "session", sessionID: session_id });
    });
  },
});
