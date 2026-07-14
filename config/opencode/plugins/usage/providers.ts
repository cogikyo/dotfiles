export type UsageProviderSpec = {
  id: string;
  label: string;
  staleAfterMS: number;
};

export const usageProviders = {
  openai: {
    id: "openai",
    label: "OpenAI",
    staleAfterMS: 2 * 60_000,
  },
  anthropic: {
    id: "anthropic",
    label: "Anthropic",
    staleAfterMS: 5 * 60_000,
  },
  xai: {
    id: "xai",
    label: "xAI",
    staleAfterMS: 10 * 60_000,
  },
  opencodeGo: {
    id: "opencode-go",
    label: "OpenCode",
    staleAfterMS: 2 * 60_000,
  },
} as const satisfies Record<string, UsageProviderSpec>;

export const usageProviderList = Object.values(usageProviders);

export function usageProvider(providerID: string) {
  return usageProviderList.find((provider) => provider.id === providerID);
}
