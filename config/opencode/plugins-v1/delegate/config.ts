import { z } from "zod";
import { errorMessage } from "../shared/error.ts";
import { readJson } from "../shared/file.ts";

/** Provider allowlist path; policy checks only its provider IDs. */
export const DELEGATE_CONFIG_PATH = "/home/cullyn/dotfiles/config/opencode/delegate.json";

const DelegateConfig = z.strictObject({
  providers: z
    .record(z.string(), z.record(z.string(), z.unknown()))
    .refine((providers) => Object.keys(providers).length > 0, "must not be empty"),
});

/** Provider allowlist; nested provider policy values are not interpreted here. */
export type DelegateConfig = z.infer<typeof DelegateConfig>;

/** Loads the provider allowlist, failing explicitly for a missing or invalid file. */
export async function loadDelegateConfig(path = DELEGATE_CONFIG_PATH): Promise<DelegateConfig> {
  let config: DelegateConfig | undefined;
  try {
    config = await readJson(path, DelegateConfig);
  } catch (error) {
    const detail = error instanceof z.ZodError ? z.prettifyError(error) : errorMessage(error);
    throw new Error(`delegate config is invalid at ${path}: ${detail}`, { cause: error });
  }
  if (!config) throw new Error(`delegate config not readable at ${path}: file does not exist`);
  return config;
}
