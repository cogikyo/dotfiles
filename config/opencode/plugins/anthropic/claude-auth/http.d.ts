export type FetchFn = typeof fetch;
export declare function fetchWithRetry(input: RequestInfo | URL, init?: RequestInit, retries?: number, fetchImpl?: FetchFn): Promise<Response>;
//# sourceMappingURL=http.d.ts.map