import { anthropicUsage } from "./anthropic.ts";
import { kimiCodeUsage } from "./kimi-code.ts";
import { opencodeGoUsage } from "./opencode-go.ts";
import { openaiUsage } from "./openai.ts";
import type { ProviderAdapter } from "./types.ts";
import { xaiUsage } from "./xai.ts";

export const usageAdapters = [
  openaiUsage,
  anthropicUsage,
  xaiUsage,
  opencodeGoUsage,
  kimiCodeUsage,
] satisfies ProviderAdapter[];
