export interface ModelOverride {
    exclude?: string[];
    add?: string[];
    disableEffort?: boolean;
}
export interface ModelConfig {
    ccVersion: string;
    baseBetas: string[];
    longContextBetas: string[];
    modelOverrides: Record<string, ModelOverride>;
}
export declare const config: ModelConfig;
/**
 * Find the override entry matching a model ID.
 * Keys are matched via includes() against the lowercased model ID.
 *
 * First-match-wins: if multiple keys match, only the first (by insertion
 * order) is returned. List more specific keys before broader ones
 * (e.g. "opus-4-6" before "opus") so they take priority.
 */
export declare function getModelOverride(modelId: string): ModelOverride | null;
//# sourceMappingURL=model-config.d.ts.map