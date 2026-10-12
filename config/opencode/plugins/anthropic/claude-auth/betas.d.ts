export declare const LONG_CONTEXT_BETAS: string[];
export declare function getExcludedBetas(modelId: string): Set<string>;
export declare function addExcludedBeta(modelId: string, beta: string): void;
export declare function resetExcludedBetas(): void;
export declare function isLongContextError(responseBody: string): boolean;
export declare function getNextBetaToExclude(modelId: string): string | null;
export declare function getModelBetas(modelId: string, excluded?: Set<string>): string[];
//# sourceMappingURL=betas.d.ts.map