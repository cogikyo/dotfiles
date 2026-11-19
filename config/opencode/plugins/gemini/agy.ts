export const binary = "agy";

const billing = /^(GOOGLE_|GEMINI_|GCLOUD_|CLOUDSDK_|AGY_)/;

export function environment(): Record<string, string> {
  const env: Record<string, string> = {};
  for (const [name, value] of Object.entries(process.env)) {
    if (value !== undefined && !billing.test(name)) env[name] = value;
  }
  return env;
}
