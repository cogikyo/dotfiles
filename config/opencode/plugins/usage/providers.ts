/** Display name and cache freshness limit for a usage provider. */
export type UsageProviderSpec = {
  id: string;
  label: string;
  staleAfterMS: number;
};

/** Provider IDs, labels, and stale thresholds used by adapters and status output. */
export const usageProviders = {
  openai: {
    id: "openai",
    label: "OpenAI",
    staleAfterMS: 2 * 60_000,
  },
  anthropic: {
    id: "anthropic",
    label: "Anthropic",
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

/** Provider specifications in declaration order. */
export const usageProviderList = Object.values(usageProviders);

/** Looks up a provider specification by its ID. */
export function usageProvider(providerID: string) {
  return usageProviderList.find((provider) => provider.id === providerID);
}
