export type UsageWindow = {
  label: string;
  usedPercent: number;
  resetAt?: string;
};

export type ProviderUsage = {
  id: string;
  label: string;
  windows: UsageWindow[];
  note?: string;
};

export type ProviderAdapter = {
  id: string;
  label: string;
  poll?: {
    minFetchIntervalMS?: number;
    errorBackoffMS?: number;
    rateLimitBackoffMS?: number;
    staleAfterMS?: number;
  };
  load(): Promise<ProviderUsage>;
};
