import { claudeAccounts } from "../anthropic/accounts.ts";

export type UsageProviderSpec = {
  id: string;
  label: string;
  staleAfterMS: number;
};

/** Shared provider identities and cache freshness limits. */
export const usageProviders = {
  openai: {
    id: "openai",
    label: "OpenAI",
    staleAfterMS: 2 * 60_000,
  },
  anthropic: {
    id: claudeAccounts.trend.id,
    label: claudeAccounts.trend.label,
    staleAfterMS: 10 * 60_000,
  },
  anthropicPersonal: {
    id: claudeAccounts.cogikyo.id,
    label: claudeAccounts.cogikyo.label,
    staleAfterMS: 10 * 60_000,
  },
  xai: {
    id: "xai",
    label: "xAI",
    staleAfterMS: 10 * 60_000,
  },
  cursor: {
    id: "cursor",
    label: "Cursor",
    staleAfterMS: 10 * 60_000,
  },
  opencodeGo: {
    id: "opencode-go",
    label: "OpenCode",
    staleAfterMS: 2 * 60_000,
  },
} as const satisfies Record<string, UsageProviderSpec>;

/** Providers in status-output order; sidebar order comes from `usageAdapters`. */
export const usageProviderList = Object.values(usageProviders);

export function usageProvider(providerID: string) {
  return usageProviderList.find((provider) => provider.id === providerID);
}
