import * as current from "../policy.ts";
import { baseline, flat, kind, rank, rev, verdict, write } from "./base.ts";
import { cwd } from "./cases.ts";

type Call = { a: string; c: string; w?: string };
type Stats = { calls: number; denied: number; denials: string[]; asks: string[]; allows: string[]; errors: number };

const sources = process.argv.slice(2);
if (sources.length === 0) throw new Error("pass history JSON files of [{ a: agent, c: command, w?: workdir }]");
const base = await baseline();
const history: Call[] = (await Promise.all(sources.map((file) => Bun.file(file).json()))).flat();
const stats: Record<string, Stats> = {};

for (const { a: agent, c: command, w: dir = cwd } of history) {
  const s = (stats[kind(agent)] ??= { calls: 0, denied: 0, denials: [], asks: [], allows: [], errors: 0 });
  s.calls++;
  let old;
  try {
    old = verdict(base.inspect(command, agent, dir), agent);
  } catch {
    s.errors++;
    continue;
  }
  let now;
  try {
    now = verdict(current.inspect(command, agent, dir), agent);
  } catch (error) {
    now = `deny THROW ${String(error)}`;
  }
  const line = `${agent}\t${flat(now.slice(0, 110))}\t${flat(command)}`;
  if (rank(old) === 2) s.denied++;
  if (rank(old) < 2 && rank(now) === 2) s.denials.push(line);
  if (rank(old) === 0 && rank(now) === 1) s.asks.push(line);
  if (rank(now) < rank(old)) s.allows.push(`${agent}\t${flat(old.slice(0, 110))}\t${flat(command)}`);
}

const lines: string[] = [];
for (const [name, s] of Object.entries(stats)) {
  lines.push(
    `== ${name}: calls ${s.calls}; ${rev} denied ${s.denied}; new denials ${s.denials.length}; new push asks ${s.asks.length}; loosened ${s.allows.length}; ${rev} throws ${s.errors}`,
  );
  for (const line of s.denials) lines.push(`  DENY  ${line}`);
  for (const line of s.asks) lines.push(`  ASK   ${line}`);
  for (const line of s.allows) lines.push(`  LOOSE ${line}`);
}
console.log(lines.join("\n"));
console.log(await write("replay.out", lines));
