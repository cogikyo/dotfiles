import { usageProviders } from "./providers.ts";
import type { ProviderAdapter } from "./types.ts";

const { id, label, staleAfterMS } = usageProviders.cursor;

export const cursorUsage: ProviderAdapter = {
  id,
  label,
  placeholders: ["C", "O", "G"],
  poll: {
    minFetchIntervalMS: 60_000,
    errorBackoffMS: 60_000,
    warnBackoffMS: 0,
    rateLimitBackoffMS: 10 * 60_000,
    staleAfterMS,
  },
  load: async () => ({ id, label, windows: [], note: "no auth" }),
};
