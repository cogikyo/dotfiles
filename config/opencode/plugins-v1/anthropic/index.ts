import type { Plugin } from "@opencode-ai/plugin";
import { ClaudeAuthPlugin } from "./claude-auth/index.js";
import { claudeAccounts } from "./accounts.ts";

export const Trend: Plugin = (input) => {
  const { id: providerID, configDir, loginCommand } = claudeAccounts.trend;
  return ClaudeAuthPlugin(input, { providerID, configDir, loginCommand });
};

export const Cogikyo: Plugin = (input) => {
  const { id: providerID, configDir, loginCommand } = claudeAccounts.cogikyo;
  return ClaudeAuthPlugin(input, { providerID, configDir, loginCommand });
};
