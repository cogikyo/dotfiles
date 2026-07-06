import type { DelegateConfig } from "./config.ts";

export function checkProviderPolicy(providerID: string, config: DelegateConfig) {
  if (!Object.hasOwn(config.providers, providerID)) {
    throw new Error(`delegate provider policy missing for ${providerID}; add it to delegate.json.providers`);
  }
}
