import { readOnly } from "../guard/policy.ts";
import { ownerEnv } from "./agy.ts";
import { check, mark, plan, refusal } from "./calls.ts";
import type { Owner } from "./gate.ts";

const input = JSON.parse(await Bun.stdin.text());
const owner: Partial<Owner> = JSON.parse(process.env[ownerEnv] || "{}");
console.log(JSON.stringify(await decide(input.toolCall ?? {})));

async function decide(call: { id?: unknown; name?: unknown; args?: unknown }) {
  const reason = refusal(call.name);
  if (reason) return { decision: "deny", reason };
  const { session, agent, directory } = owner;
  if (call.name !== "run_command" || !readOnly(agent) || !session || !directory) return { decision: "force_ask" };
  const id = String(call.id);
  const steps = plan({ toolCallId: id, name: call.name, rawInput: call.args }, directory);
  if ("refuse" in steps) return { decision: "deny", reason: steps.refuse };
  const checked = await check(steps, { session, agent, directory });
  if ("refuse" in checked) return { decision: "deny", reason: checked.refuse };
  if (!checked.cage) return { decision: "force_ask" };
  await mark(id, checked.cage);
  return { decision: "force_ask", overwrite: checked.cage };
}
