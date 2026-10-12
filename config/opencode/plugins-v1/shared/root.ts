import path from "node:path";
import { fileURLToPath } from "node:url";

export const configRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..", "..");

export const COLLAB = "collab";

export type Loader = { agent?: string; parentID?: string };

type Scope = (session: Loader) => boolean;

const always: Scope = () => true;
const topCollab: Scope = (session) => session.agent === COLLAB && !session.parentID;

const files = [
  ["AGENTS.md", always],
  ["COLLAB.md", topCollab],
  ["ROUTING.md", topCollab],
] as const;

export const ROOT_FILES = files.map(([file]) => file);

export type RootFile = (typeof ROOT_FILES)[number];

export function rootFiles(session: Loader): RootFile[] {
  return files.filter(([, loads]) => loads(session)).map(([file]) => file);
}
