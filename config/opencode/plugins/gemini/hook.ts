import { connect } from "node:net";
import { readOnly } from "../guard/policy.ts";
import { ownerEnv, socketEnv, tokenEnv, writeMethod } from "./agy.ts";
import { check, mark, plan, refusal, rewrite } from "./calls.ts";
import type { Owner } from "./gate.ts";

// Stays under session.ts's 10s hook timeout so the hook can still deny cleanly.
const deadline = 8_000;

const input = JSON.parse(await Bun.stdin.text());
const owner: Partial<Owner> = JSON.parse(process.env[ownerEnv] || "{}");
console.log(JSON.stringify(await decide(input.toolCall ?? {})));

async function decide(call: { id?: unknown; name?: unknown; args?: unknown }) {
  const reason = refusal(call.name);
  if (reason) return { decision: "deny", reason };
  const { session, agent, directory } = owner;
  if (!session || !directory) return { decision: "force_ask" };
  const restricted = readOnly(agent);
  if (restricted && call.name === "edit_file") return { decision: "deny", reason: rewrite(session) };
  // The Gate already did the caged write; deny keeps agy's host write_file from running.
  if (restricted && call.name === "write_file")
    return { decision: "deny", reason: await relay(String(call.id), call.args) };
  const id = String(call.id);
  const steps = plan({ toolCallId: id, name: String(call.name), rawInput: call.args }, directory);
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
    let sent = false;
    const finish = (text: string) => {
      clearTimeout(timer);
      socket.destroy();
      resolve(text);
    };
    const fail = (why: string) => finish(sent ? unknown(why) : unavailable(why));
    const timer = setTimeout(() => fail(`OpenCode did not answer within ${deadline / 1000} seconds`), deadline);
    let buffer = "";
    socket.setEncoding("utf8");
    socket.on("connect", () => {
      const request = { jsonrpc: "2.0", id: 0, method: writeMethod, params: { id, args } };
      socket.write(`${token}\n${JSON.stringify(request)}\n`);
      sent = true;
    });
    socket.on("error", (error) => fail(`the OpenCode bridge failed (${error.message})`));
    socket.on("close", () => fail("the OpenCode bridge closed without answering"));
    socket.on("data", (chunk: string) => {
      buffer += chunk;
      const end = buffer.indexOf("\n");
      if (end === -1) return;
      const reply = parse(buffer.slice(0, end));
      if (typeof reply?.result?.text === "string") finish(reply.result.text);
      else fail("the OpenCode bridge sent no result");
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

function unknown(why: string) {
  return `write_file outcome unknown: ${why}, and the write may still land. Check the target path with \`run_command\` before you retry`;
}
