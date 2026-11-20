import { homedir } from "node:os";
import { join } from "node:path";

export const binary = "agy";
export const ownerEnv = "OPENCODE_GEMINI_OWNER";
export const profile = join(process.env.XDG_STATE_HOME || join(homedir(), ".local", "state"), "opencode", "gemini");
export const cages = join(profile, "cages");

const billing = /^(GOOGLE_|GEMINI_|GCLOUD_|CLOUDSDK_|AGY_)/;

export function environment(): Record<string, string> {
  const env: Record<string, string> = {};
  for (const [name, value] of Object.entries(process.env)) {
    if (value !== undefined && !billing.test(name)) env[name] = value;
  }
  return env;
}
