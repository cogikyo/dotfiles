import { config, getModelOverride } from "./model-config.js";
// Beta flags to try removing in order when "long context" errors occur
export const LONG_CONTEXT_BETAS = config.longContextBetas;
function getRequiredBetas() {
    return (process.env.ANTHROPIC_BETA_FLAGS ?? config.baseBetas.join(","))
        .split(",")
        .map((s) => s.trim())
        .filter(Boolean);
}
// Session-level cache of excluded beta flags per model (resets on process restart)
const excludedBetas = new Map();
// Track the last-seen beta flags env var and model to detect changes
let lastBetaFlagsEnv = process.env.ANTHROPIC_BETA_FLAGS;
let lastModelId;
export function getExcludedBetas(modelId) {
    // Reset exclusions if user changed ANTHROPIC_BETA_FLAGS
    const currentBetaFlags = process.env.ANTHROPIC_BETA_FLAGS;
    if (currentBetaFlags !== lastBetaFlagsEnv) {
        excludedBetas.clear();
        lastBetaFlagsEnv = currentBetaFlags;
    }
    // Reset exclusions if user switched models (new model may support different betas)
    if (lastModelId !== undefined && lastModelId !== modelId) {
        excludedBetas.clear();
    }
    lastModelId = modelId;
    return excludedBetas.get(modelId) ?? new Set();
}
export function addExcludedBeta(modelId, beta) {
    const existing = excludedBetas.get(modelId) ?? new Set();
    existing.add(beta);
    excludedBetas.set(modelId, existing);
}
export function resetExcludedBetas() {
    excludedBetas.clear();
    lastModelId = undefined;
}
export function isLongContextError(responseBody) {
    return (responseBody.includes("Extra usage is required for long context requests") ||
        responseBody.includes("long context beta is not yet available") ||
        responseBody.includes("You're out of extra usage"));
}
export function getNextBetaToExclude(modelId) {
    const excluded = getExcludedBetas(modelId);
    for (const beta of LONG_CONTEXT_BETAS) {
        if (!excluded.has(beta)) {
            return beta;
        }
    }
    return null; // All long-context betas already excluded
}
export function getModelBetas(modelId, excluded) {
    let betas = [...getRequiredBetas()];
    // The legacy context-1m-2025-08-07 beta is never sent — the API supports
    // 1M context natively without it.
    // Apply per-model overrides (e.g. haiku excludes effort-2025-11-24)
    const override = getModelOverride(modelId);
    if (override) {
        const { exclude, add } = override;
        if (exclude) {
            // Remove every occurrence — regenerated configs can contain duplicates
            betas = betas.filter((beta) => !exclude.includes(beta));
        }
        if (add) {
            for (const beta of add) {
                if (!betas.includes(beta))
                    betas.push(beta);
            }
        }
    }
    // Filter out excluded betas (from previous failed requests due to long context errors)
    if (excluded && excluded.size > 0) {
        return betas.filter((beta) => !excluded.has(beta));
    }
    return betas;
}
//# sourceMappingURL=betas.js.map