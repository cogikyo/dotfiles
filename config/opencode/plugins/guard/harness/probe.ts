import { prompts } from "../../terminal/server.ts";
import * as current from "../policy.ts";
import { baseline, flat, rank, rev, verdict, write } from "./base.ts";
import { benign, bypass, cwd, holes, prompted, unprompted } from "./cases.ts";

const base = await baseline();
const lines: string[] = [];
const failed: string[] = [];
const looser: string[] = [];

for (const [set, list] of [
  ["bypass", bypass],
  ["benign", benign],
  ["holes", holes],
] as const) {
  lines.push(`== ${set}`);
  for (const [agent, command, dir = cwd] of list) {
    const old = verdict(base.inspect(command, agent, dir), agent);
    const now = verdict(current.inspect(command, agent, dir), agent);
    const mark = rank(old) === rank(now) ? "  " : "* ";
    const where = dir === cwd ? "" : ` (cwd ${dir})`;
    lines.push(
      `${mark}${agent.padEnd(14)} ${flat(command)}${where}`,
      `    ${rev}: ${flat(old)}`,
      `    now: ${flat(now)}`,
    );
    if (set === "bypass" && now === "allow") failed.push(command);
    if (set === "bypass" && rank(now) < rank(old)) looser.push(command);
    if (set === "benign" && /rm /u.test(command) && now !== "allow") failed.push(command);
  }
}
lines.push("== terminal prompts()");
for (const [command, expected] of [
  ...prompted.map((line) => [line, true] as const),
  ...unprompted.map((line) => [line, false] as const),
]) {
  const now = prompts(command);
  lines.push(`${now === expected ? "  " : "! "}${String(now).padEnd(5)} ${command}`);
  if (now !== expected) failed.push(`prompts(${command}) = ${now}`);
}
lines.push(
  `bypass allowed, required benign denied, or prompts() wrong: ${failed.length}`,
  ...failed.map((command) => `  ${flat(command)}`),
  `bypass looser than ${rev}: ${looser.length}`,
  ...looser.map((command) => `  ${flat(command)}`),
);
console.log(lines.join("\n"));
console.log(await write("probe.out", lines));
