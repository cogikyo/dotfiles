import type { Writable } from "node:stream";
export declare function initLogger(options?: {
    stream?: Writable;
}): void;
export declare function log(event: string, data?: Record<string, unknown>): void;
export declare function closeLogger(): void;
export declare function redact(data: Record<string, unknown>): Record<string, unknown>;
//# sourceMappingURL=logger.d.ts.map