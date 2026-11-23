import path from "node:path";
import { scratch } from "../guard/paths.ts";
import { readOnly } from "../guard/policy.ts";
import type { Owner } from "./gate.ts";

const refused = [
  "ask_question",
  "define_subagent",
  "invoke_subagent",
  "send_message",
  "manage_subagents",
  "schedule",
  "swarm",
  "run_workflow",
];

export function preface(owner: Owner) {
  const lines = [
    "Tool names in your brief map to yours: `shell` is `run_command`, `read` is `read_file`, `write` is `write_file`, and `edit` or `patch` is `edit_file`.",
    "- Other tools in a brief are `mcp_opencode_<name>`, such as `mcp_opencode_grep`; a tool missing from your list is unavailable.",
    "- `execute` is unavailable, so call each tool directly instead of scripting tool calls.",
    "- `search_web` searches the web. `manage_task` can list, check, or kill background commands, but not send them input.",
    `- ${refused.map((name) => `\`${name}\``).join(", ")} are refused; ask questions in your reply instead.`,
  ];
  if (!owner.session || !readOnly(owner.agent))
    return [
      ...lines,
      "- `$OPENCODE_SCRATCH` is unset in `run_command`; put scratch files under `/tmp/opencode/`.",
    ].join("\n");
  const dir = path.join(scratch, owner.session);
  return [
    ...lines,
    `- You are read-only. \`run_command\` runs offline in a sandbox that can write only under your scratch directory ${dir}, which it names \`$OPENCODE_SCRATCH\`; its \`tmp/\` and \`cache/\` hold throwaway files.`,
    `- \`write_file\` writes only regular files directly in ${dir}, and \`edit_file\` is refused.`,
  ].join("\n");
}
