import type { Plugin } from "@opencode/plugin/tui";

type Session = Plugin.Context["data"]["session"];
type Message = NonNullable<ReturnType<Session["message"]["get"]>>;
type Tool = Extract<Extract<Message, { type: "assistant" }>["content"][number], { type: "tool" }>;

export function tool(session: Pick<Session, "message">, sessionID: string, rowID: string): Tool | undefined {
  const [, messageID, partID] = /^session-part:([^:]+):(.+)$/.exec(rowID) ?? [];
  const message = session.message.get(sessionID, messageID ?? rowID);
  if (message?.type !== "assistant") return;
  const [first] = message.content;
  if (partID === undefined) return first?.type === "tool" ? first : undefined;
  return message.content.find((part): part is Tool => part.type === "tool" && part.id === partID);
}

export function asks(session: Pick<Session, "message">, sessionID: string, rowID: string) {
  return tool(session, sessionID, rowID)?.name === "question";
}
