import { launches } from "./shell.ts";

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

export function denial(resource: string) {
  const commands = launches(resource);
  if (commands.some(([name]) => name === "rm")) return "rm is disabled; move files to trash with `trash -- <path>`";
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
