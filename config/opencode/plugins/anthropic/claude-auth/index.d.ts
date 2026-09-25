import type { Plugin } from "@opencode-ai/plugin";
export { addExcludedBeta, getExcludedBetas, getModelBetas, getNextBetaToExclude, isLongContextError, LONG_CONTEXT_BETAS, } from "./betas.ts";
export { resetExcludedBetas } from "./betas.ts";
export { fetchWithRetry, type FetchFn } from "./http.ts";
export { stripToolPrefix, SYSTEM_IDENTITY, transformBody, transformResponseStream, } from "./transforms.ts";
export { getCachedCredentials, syncAuthJson, refreshAccountsList, type ClaudeCredentials, } from "./credentials.ts";
export { buildBillingHeaderValue, computeCch, computeVersionSuffix, extractFirstUserMessageText, } from "./signing.ts";
export declare function buildRequestHeaders(input: RequestInfo | URL, init: RequestInit, accessToken: string, modelId?: string, excludedBetas?: Set<string>): Headers;
declare const plugin: Plugin;
export declare const ClaudeAuthPlugin: Plugin;
export default plugin;
//# sourceMappingURL=index.d.ts.map