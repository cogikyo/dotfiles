import type { PluginInput } from "@opencode-ai/plugin";

export type Client = PluginInput["client"];

export async function unwrap<T>(promise: Promise<{ data: T | undefined; error?: unknown }>, label: string): Promise<T> {
  const response = await promise;
  if (response.error !== undefined) {
    throw new Error(`delegate ${label} failed: ${errorMessage(response.error)}`);
  }
  return response.data!;
}

export function string(value: unknown) {
  return typeof value === "string" && value ? value : undefined;
}

export function finite(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

export function errorMessage(error: unknown): string {
  if (error instanceof Error) return error.message;
  if (typeof error === "string") return error;
  try {
    return JSON.stringify(error);
  } catch {
    return String(error);
  }
}
