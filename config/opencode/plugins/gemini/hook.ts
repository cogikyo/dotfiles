import { ownerEnv } from "./agy.ts";
import { check, mark, plan, refusal } from "./calls.ts";
import type { Owner } from "./gate.ts";

const deadline = 8_000;

const input = JSON.parse(await Bun.stdin.text());
const owner: Partial<Owner> = JSON.parse(process.env[ownerEnv] || "{}");
const timer = setTimeout(() => {
  console.log(JSON.stringify({ decision: "force_ask" }));
  process.exit(0);
}, deadline);
const decision = await decide(input.toolCall ?? {});
clearTimeout(timer);
console.log(JSON.stringify(decision));

async function decide(call: { id?: unknown; name?: unknown; args?: unknown }) {
  const reason = refusal(call.name);
  if (reason) return { decision: "deny", reason };
  const { session, agent, directory } = owner;
  if (!session || !directory)
    return { decision: "deny", reason: "this request has no OpenCode session to judge tool calls against" };
  const id = String(call.id);
  const lane = { session, agent, directory };
  const steps = plan({ toolCallId: id, name: String(call.name), rawInput: call.args }, lane);
  if ("refuse" in steps) return { decision: "deny", reason: steps.refuse };
  const checked = await check(steps, lane);
  if ("refuse" in checked) return { decision: "deny", reason: checked.refuse };
  if (!checked.overwrite) return { decision: "force_ask" };
  mark(id, checked.overwrite);
  return { decision: "force_ask", overwrite: checked.overwrite };
}
