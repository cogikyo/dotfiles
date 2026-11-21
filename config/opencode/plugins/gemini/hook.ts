import { connect } from "node:net";
import { readOnly } from "../guard/policy.ts";
import { ownerEnv, socketEnv, tokenEnv, writeMethod } from "./agy.ts";
import { check, mark, plan, refusal, rewrite } from "./calls.ts";
import type { Owner } from "./gate.ts";

const deadline = 8_000;

const input = JSON.parse(await Bun.stdin.text());
const owner: Partial<Owner> = JSON.parse(process.env[ownerEnv] || "{}");
console.log(JSON.stringify(await decide(input.toolCall ?? {})));

async function decide(call: { id?: unknown; name?: unknown; args?: unknown }) {
  const reason = refusal(call.name);
  if (reason) return { decision: "deny", reason };
  const { session, agent, directory } = owner;
  if (!readOnly(agent) || !session || !directory) return { decision: "force_ask" };
  if (call.name === "edit_file") return { decision: "deny", reason: rewrite(session) };
  if (call.name === "write_file") return { decision: "deny", reason: await relay(String(call.id), call.args) };
  if (call.name !== "run_command") return { decision: "force_ask" };
  const id = String(call.id);
  const steps = plan({ toolCallId: id, name: call.name, rawInput: call.args }, directory);
  if ("refuse" in steps) return { decision: "deny", reason: steps.refuse };
  const checked = await check(steps, { session, agent, directory });
  if ("refuse" in checked) return { decision: "deny", reason: checked.refuse };
  if (!checked.cage) return { decision: "force_ask" };
  await mark(id, checked.cage);
  return { decision: "force_ask", overwrite: checked.cage };
}

function relay(id: string, args: unknown) {
  const file = process.env[socketEnv];
  const token = process.env[tokenEnv];
  if (!file || !token) return Promise.resolve(unavailable("this turn has no OpenCode bridge; restart OpenCode"));
  return new Promise<string>((resolve) => {
    const socket = connect(file);
    const finish = (text: string) => {
      clearTimeout(timer);
      socket.destroy();
      resolve(text);
    };
    const timer = setTimeout(
      () => finish(unavailable(`OpenCode did not answer within ${deadline / 1000} seconds`)),
      deadline,
    );
    let buffer = "";
    socket.setEncoding("utf8");
    socket.on("connect", () => {
      const request = { jsonrpc: "2.0", id: 0, method: writeMethod, params: { id, args } };
      socket.write(`${token}\n${JSON.stringify(request)}\n`);
    });
    socket.on("error", (error) => finish(unavailable(`the OpenCode bridge failed (${error.message})`)));
    socket.on("close", () => finish(unavailable("the OpenCode bridge closed without answering")));
    socket.on("data", (chunk: string) => {
      buffer += chunk;
      const end = buffer.indexOf("\n");
      if (end === -1) return;
      const reply = parse(buffer.slice(0, end));
      finish(
        typeof reply?.result?.text === "string" ? reply.result.text : unavailable("the OpenCode bridge sent no result"),
      );
    });
  });
}

function parse(line: string): { result?: { text?: unknown } } | undefined {
  try {
    return JSON.parse(line);
  } catch {
    return undefined;
  }
}

function unavailable(why: string) {
  return `write_file did not run: ${why}. Nothing was written; create the file with \`run_command\` instead, such as a quoted heredoc into $OPENCODE_SCRATCH`;
}
