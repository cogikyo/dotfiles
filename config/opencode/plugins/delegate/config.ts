import fs from "node:fs/promises";

export const DELEGATE_CONFIG_PATH = "/home/cullyn/dotfiles/config/opencode/delegate.json";

export type DelegateConfig = {
  thresholds: Record<string, number>;
  maxWaitMinutes: number;
  staleCacheMinutes: number;
  providers: Record<string, Record<string, unknown>>;
};

export async function loadDelegateConfig(path = DELEGATE_CONFIG_PATH): Promise<DelegateConfig> {
  let raw: string;
  try {
    raw = await fs.readFile(path, "utf8");
  } catch (error) {
    throw new Error(`delegate config not readable at ${path}: ${errorMessage(error)}`);
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(raw) as unknown;
  } catch (error) {
    throw new Error(`delegate config is not valid JSON at ${path}: ${errorMessage(error)}`);
  }

  return validateDelegateConfig(parsed, path);
}

function validateDelegateConfig(value: unknown, source: string): DelegateConfig {
  const root = object(value, source);
  const thresholds = numberRecord(root.thresholds, `${source}.thresholds`, 0, 100);
  const maxWaitMinutes = positiveNumber(root.maxWaitMinutes, `${source}.maxWaitMinutes`);
  const staleCacheMinutes = positiveNumber(root.staleCacheMinutes, `${source}.staleCacheMinutes`);
  const providers = objectRecord(root.providers, `${source}.providers`);

  if (!Object.keys(thresholds).length) throw new Error(`delegate config ${source}.thresholds must not be empty`);
  if (!Object.keys(providers).length) throw new Error(`delegate config ${source}.providers must not be empty`);

  return { thresholds, maxWaitMinutes, staleCacheMinutes, providers };
}

function object(value: unknown, label: string): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new Error(`delegate config ${label} must be an object`);
  }
  return value as Record<string, unknown>;
}

function objectRecord(value: unknown, label: string): Record<string, Record<string, unknown>> {
  const root = object(value, label);
  return Object.fromEntries(Object.entries(root).map(([key, item]) => [key, object(item, `${label}.${key}`)]));
}

function numberRecord(value: unknown, label: string, min: number, max: number): Record<string, number> {
  const root = object(value, label);
  return Object.fromEntries(
    Object.entries(root).map(([key, item]) => [key, boundedNumber(item, `${label}.${key}`, min, max)]),
  );
}

function positiveNumber(value: unknown, label: string) {
  return boundedNumber(value, label, Number.MIN_VALUE, Number.POSITIVE_INFINITY);
}

function boundedNumber(value: unknown, label: string, min: number, max: number) {
  if (typeof value !== "number" || !Number.isFinite(value) || value < min || value > max) {
    throw new Error(`delegate config ${label} must be a finite number from ${min} to ${max}`);
  }
  return value;
}

function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}
