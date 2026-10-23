import path from "node:path";
import { located, temp, within } from "./paths.ts";
import { type Command, launches } from "./shell.ts";

export function readOnly(agent: string | undefined) {
  if (!agent) return false;
  return (
    agent === "review" ||
    agent.startsWith("review/") ||
    agent === "scout" ||
    agent.startsWith("scout/") ||
    agent === "verify/source" ||
    agent === "verify/web"
  );
}

const credentials = [
  /(?:^|[\s/=@<>:])\.claude(?:-[\w-]+)?\/\.credentials\.json(?![\w.-])/u,
  /(?:^|[\s/=@<>:])opencode[\w-]*\/(?:opencode\/)?auth\.json(?![\w.-])/u,
  /(?:^|[\s/=@<>:])opencode\/service(?:-[\w.-]+)?\.json(?![\w.-])/u,
];
const database = /(?:^|[\s/=@<>:])opencode[\w-]*\/(?:opencode\/)?opencode\.db(?:-wal|-shm)?(?![\w.-])/u;

function unresolved(name: string) {
  return name !== "[" && name !== "[[" && /[$`*?[]|\{[^}]*,/u.test(name);
}

function operands(args: string[]) {
  const end = args.indexOf("--");
  const flagged = end < 0 ? args : args.slice(0, end);
  const rest = end < 0 ? [] : args.slice(end + 1);
  return [...flagged.filter((arg) => arg === "-" || !arg.startsWith("-")), ...rest];
}

function removable(operand: string) {
  if (!path.isAbsolute(operand) || /[$`]/u.test(operand) || operand.endsWith("/")) return false;
  if (operand.includes("..") || operand.split("/").includes(".")) return false;
  const dir = path.dirname(operand);
  if (/[*?[{]/u.test(dir)) return false;
  try {
    const real = located(dir);
    return within(real, temp) && !(real === temp && /[*?[{]/u.test(path.basename(operand)));
  } catch {
    return false;
  }
}

function removal([, ...args]: Command) {
  const targets = operands(args);
  return targets.length > 0 && targets.every(removable);
}

export function denial(resource: string, agent: string | undefined) {
  const commands = launches(resource);
  // Cage already confines read-only deletion to scratch tmp/ and cache/.
  if (!readOnly(agent) && commands.some((command) => command[0] === "rm" && !removal(command))) {
    return `rm takes only literal absolute paths inside ${temp}/, without variables, \`..\`, a trailing slash, or a top-level glob; move anything else to trash with \`trash -- <path>\``;
  }
  if (commands.some(([name]) => name === "grok")) return "grok is disabled in the shell; use the `x` tool for X search";
  if (commands.some(([name, ...args]) => (name === "opencode" || name === "opencode-v2") && args.includes("api"))) {
    return "agents must not call the OpenCode server API; `opencode api` is disabled in the shell; to create a session the user asked for, call `tools.opencode.session_create` through `execute`";
  }
  const dynamic = commands.find(([name]) => unresolved(name));
  if (dynamic) {
    return `the command name \`${dynamic[0]}\` is expanded at run time; write the command name literally so the guard can check it`;
  }
  // cage's --hide operands name denied files, not attempted reads.
  const args = commands.filter(([name]) => name !== "cage").flatMap(([, ...rest]) => rest);
  const secret = args.flatMap((arg) => credentials.map((file) => file.exec(arg)?.[0])).find(Boolean);
  if (secret) return `\`${secret.replace(/^[\s/=@<>:]/u, "")}\` is a credential file; agents must not read it`;
  if (args.some((arg) => database.test(arg))) {
    return "the OpenCode database holds provider credentials; agents must not open it from the shell; use the `sessions` tool for session history";
  }
  return undefined;
}
