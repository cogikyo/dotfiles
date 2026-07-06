export type TokenMessage = {
  role?: string;
  tokens?: {
    input?: number;
    output?: number;
    reasoning?: number;
    cache?: {
      read?: number;
      write?: number;
    };
  };
};

export type PressureThresholds = {
  checkpointPercent: number;
  compactPercent: number;
  renewPercent: number;
  checkpointTokens: number;
  compactTokens: number;
  renewTokens: number;
  renewalRemaining: number;
};

export type PressureAssessment = {
  tokens: number;
  limit?: number;
  reserved: number;
  usable?: number;
  percent: number;
  remaining?: number;
  level: "low" | "checkpoint" | "compact" | "renew";
  shouldCheckpoint: boolean;
  shouldCompact: boolean;
  shouldRenew: boolean;
};

export const DEFAULT_RESERVED_TOKENS = 10_000;
export const DEFAULT_CONTEXT_LIMIT = 150_000;
export const DEFAULT_THRESHOLDS: PressureThresholds = {
  checkpointPercent: 75,
  compactPercent: 90,
  renewPercent: 96,
  checkpointTokens: 90_000,
  compactTokens: 120_000,
  renewTokens: 200_000,
  renewalRemaining: 12_000,
};

export function assessPressure(input: {
  messages: ReadonlyArray<TokenMessage>;
  modelLimit?: number;
  reserved?: number;
  thresholds?: Partial<PressureThresholds>;
}): PressureAssessment {
  const thresholds = { ...DEFAULT_THRESHOLDS, ...input.thresholds };
  const reserved = sanePositive(input.reserved) ?? DEFAULT_RESERVED_TOKENS;
  const explicitLimit = sanePositive(input.modelLimit);
  const limit = explicitLimit ?? DEFAULT_CONTEXT_LIMIT;
  const usable = Math.max(1, limit - reserved);
  const tokens = Math.max(...input.messages.map(contextTokens), 0);
  const percent = Math.min(100, (tokens / usable) * 100);
  const remaining = Math.max(0, usable - tokens);
  const shouldRenew = tokens >= thresholds.renewTokens || (explicitLimit !== undefined && (percent >= thresholds.renewPercent || remaining <= thresholds.renewalRemaining));
  const shouldCompact = shouldRenew || tokens >= thresholds.compactTokens || (explicitLimit !== undefined && percent >= thresholds.compactPercent);
  const shouldCheckpoint = shouldCompact || tokens >= thresholds.checkpointTokens || (explicitLimit !== undefined && percent >= thresholds.checkpointPercent);
  const level = shouldRenew ? "renew" : shouldCompact ? "compact" : shouldCheckpoint ? "checkpoint" : "low";

  return { tokens, limit, reserved, usable, percent, remaining, level, shouldCheckpoint, shouldCompact, shouldRenew };
}

function contextTokens(message: TokenMessage) {
  if (message.role !== "assistant") return 0;
  const tokens = message.tokens;
  if (!tokens) return 0;
  return number(tokens.input) + number(tokens.cache?.read) + number(tokens.cache?.write);
}

function sanePositive(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : undefined;
}

function number(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
