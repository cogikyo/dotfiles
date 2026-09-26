import { type Client, session } from "./opencode.ts";

const roots = new Set<string>();
const parents = new Map<string, string | undefined>();

export function arm(sessionID: string) {
  roots.add(sessionID);
}

export function disarm(sessionID: string) {
  roots.delete(sessionID);
}

export function isRoot(sessionID: string) {
  return roots.has(sessionID);
}

export async function armed(client: Client, sessionID: string) {
  if (roots.size === 0) return false;
  return roots.has(await root(client, sessionID));
}

async function root(client: Client, sessionID: string): Promise<string> {
  if (!parents.has(sessionID)) {
    const info = await session(client, sessionID, { label: `drive read session ${sessionID}` });
    parents.set(sessionID, info.parentID);
  }
  const parent = parents.get(sessionID);
  return parent ? root(client, parent) : sessionID;
}
