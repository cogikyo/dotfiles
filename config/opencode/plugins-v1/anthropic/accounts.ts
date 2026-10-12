import os from "node:os";
import path from "node:path";

export const claudeAccounts = {
  trend: {
    id: "anthropic",
    label: "Anthropic [Trend]",
    configDir: path.join(os.homedir(), ".claude"),
    loginCommand: "claude-auth trend",
  },
  cogikyo: {
    id: "anthropic-personal",
    label: "Anthropic [Cogikyo]",
    configDir: path.join(os.homedir(), ".claude-cogikyo"),
    loginCommand: "claude-auth cogikyo",
  },
} as const;
