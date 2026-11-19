import { decide, type Call, type Verdict } from "./policy.ts";

const allowed =
  "this read-only session allows read_file and the shell commands pwd, ls without recursion, and git log, show, diff, or blame";

const input = JSON.parse(await Bun.stdin.text());
const { id, name, args } = input.toolCall;
const call: Call | undefined =
  name === "run_command"
    ? { toolCallId: id, kind: "execute", rawInput: args }
    : name === "read_file"
      ? { toolCallId: id, kind: "read", locations: [{ path: args?.Path }] }
      : undefined;
const verdict: Verdict = call ? decide(call, input.cwd) : { outcome: "reject", reason: `${name} is not available` };
console.log(
  JSON.stringify(
    verdict.outcome === "allow"
      ? { decision: "force_ask" }
      : { decision: "deny", reason: `${verdict.reason}; ${allowed}` },
  ),
);
