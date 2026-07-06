import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { DEFAULT_THRESHOLDS, type PressureThresholds } from "./pressure.ts";

export type ContinuitySettings = {
  pressure: PressureThresholds;
};

export function readSettings(): ContinuitySettings {
  const filePath = path.join(path.dirname(fileURLToPath(import.meta.url)), "settings.json");
  const root = plainObject(JSON.parse(readFileSync(filePath, "utf8")), `continuity settings in ${filePath}`);
  const pressure = plainObject(root.pressure, `continuity pressure settings in ${filePath}`);

  return { pressure: parsePressure(pressure, filePath) };
}

const thresholdKeys = [
  "checkpointPercent",
  "compactPercent",
  "renewPercent",
  "checkpointTokens",
  "compactTokens",
  "renewTokens",
  "renewalRemaining",
] as const satisfies ReadonlyArray<keyof PressureThresholds>;

function parsePressure(input: Record<string, unknown>, filePath: string): PressureThresholds {
  const unknown = Object.keys(input).filter((key) => !thresholdKeys.includes(key as keyof PressureThresholds));
  if (unknown.length > 0) throw new Error(`unknown continuity pressure setting(s) in ${filePath}: ${unknown.join(", ")}`);

  const thresholds = { ...DEFAULT_THRESHOLDS };
  for (const key of thresholdKeys) {
    if (input[key] !== undefined) thresholds[key] = positiveNumber(input[key], `${key} in ${filePath}`);
  }

  if (!(thresholds.checkpointPercent < thresholds.compactPercent && thresholds.compactPercent < thresholds.renewPercent)) {
    throw new Error(`continuity percent thresholds must increase checkpoint < compact < renew in ${filePath}`);
  }
  if (!(thresholds.checkpointTokens < thresholds.compactTokens && thresholds.compactTokens < thresholds.renewTokens)) {
    throw new Error(`continuity token thresholds must increase checkpoint < compact < renew in ${filePath}`);
  }

  return thresholds;
}

function plainObject(value: unknown, label: string): Record<string, unknown> {
  if (typeof value === "object" && value !== null && !Array.isArray(value)) return value as Record<string, unknown>;
  throw new Error(`${label} must be a JSON object`);
}

function positiveNumber(value: unknown, label: string) {
  if (typeof value === "number" && Number.isFinite(value) && value > 0) return value;
  throw new Error(`${label} must be a positive number`);
}
