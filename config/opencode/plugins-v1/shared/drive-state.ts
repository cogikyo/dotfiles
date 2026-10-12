import { homedir } from "node:os";
import { join } from "node:path";
import { z } from "zod";
import { readJson, writeJson } from "./file.ts";

const path = join(process.env.XDG_STATE_HOME || join(homedir(), ".local", "state"), "opencode", "drive.json");
const Roots = z.array(z.string());

export async function roots() {
  return new Set((await readJson(path, Roots)) ?? []);
}

export async function toggle(sessionID: string) {
  const current = await roots();
  const on = !current.delete(sessionID);
  if (on) current.add(sessionID);
  await writeJson(path, [...current]);
  return on;
}
