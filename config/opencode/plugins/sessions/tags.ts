export const states = ["closed", "archived", "research"] as const;
export const flags = ["fix", "issues", "reviewme", "refusals"] as const;

export type State = (typeof states)[number];
export type Flag = (typeof flags)[number];
export type Tag = { readonly state?: State; readonly flags: readonly Flag[] };

const pattern = /^\[([A-Z,]+)\] /u;

// Agents can't retag REFUSALS sessions, so some may still carry the retired TODO flag.
const retired = "TODO";

function read(title?: string) {
  const match = pattern.exec(title ?? "");
  if (!match) return undefined;
  const names = match[1].split(",").filter((name) => name !== retired);
  const state = states.find((name) => name.toUpperCase() === names[0]);
  const found = names.slice(state ? 1 : 0).map((name) => flags.find((flag) => flag.toUpperCase() === name));
  if (found.some((flag) => !flag)) return undefined;
  return {
    tag: { state, flags: found.filter((flag) => flag !== undefined) },
    plain: match.input.slice(match[0].length),
  };
}

export const tag = (title?: string): Tag | undefined => read(title)?.tag;

export type Change = { readonly state?: State; readonly flags?: readonly Flag[]; readonly add?: readonly Flag[] };

export function revise(current: Tag | undefined, change: Change): Tag {
  if (!change.state && !change.flags && !change.add) throw new Error("session_tag: pass state, flags, or add");
  // Only the TUI toggle can clear REFUSALS.
  const sticky = current?.flags.filter((name) => name === "refusals") ?? [];
  const base = change.flags ?? current?.flags.filter((name) => name !== "refusals") ?? [];
  return {
    state: change.state ?? current?.state,
    flags: [...new Set([...base, ...(change.add ?? []), ...sticky])],
  };
}

export function retag(title: string, next: Tag) {
  const plain = read(title)?.plain ?? title;
  const names = [...(next.state ? [next.state] : []), ...flags.filter((flag) => next.flags.includes(flag))];
  return names.length ? `[${names.join(",").toUpperCase()}] ${plain}` : plain;
}

export const title = (session: { readonly title?: string; readonly time: { readonly created: number } }) =>
  session.title ?? `New session - ${new Date(session.time.created).toISOString()}`;
