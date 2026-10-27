import type { Plugin } from "@opencode/plugin/tui";
import { Schema } from "effect";
import { retag, tag, title } from "../sessions/tags.ts";

type Data = Plugin.Context["data"]["session"];
type Message = ReturnType<Data["message"]["list"]>[number];

export const source = "mail";
export const kinds = ["info", "ask"] as const;
export const envelope = { closed: "\u{f01ee}", open: "\u{f01ef}" };
export const glyphs: Record<Kind, string> = { info: envelope.closed, ask: "\u{f173a}" };

export const Mail = Schema.Struct({
  source: Schema.Literal(source),
  kind: Schema.Literals(kinds),
  id: Schema.String,
  from: Schema.String,
  sender: Schema.String,
  to: Schema.String,
  recipient: Schema.String,
  subject: Schema.String,
  body: Schema.String,
  sent: Schema.Finite,
  replyTo: Schema.optional(Schema.String),
});

export type Mail = typeof Mail.Type;
export type Kind = Mail["kind"];

export const Letter = Schema.Struct({ mail: Mail, parked: Schema.Boolean });
export type Letter = typeof Letter.Type;

export const Ledger = Schema.Struct({
  received: Schema.Array(Letter),
  sent: Schema.Array(Letter),
  unreachable: Schema.Array(Schema.String),
});
export type Ledger = typeof Ledger.Type;

export type State = "unread" | "owed" | "waiting" | "asked";
export type Open = { state: State; mail: Mail; peer: string; name: string };

export const plain = (session: Parameters<typeof title>[0]) => retag(title(session), { flags: [] });

export const flat = (text: string) => text.replace(/\s+/gu, " ").trim();

export function mailable(session: { readonly parentID?: string | null; readonly title?: string | null }) {
  const marked = tag(session.title ?? undefined);
  return !session.parentID && !marked?.state && !marked?.flags.includes("refusals");
}

export const read = (metadata: unknown): Mail | undefined => (Schema.is(Mail)(metadata) ? metadata : undefined);

const answered = (replies: readonly Letter[], ask: Mail) =>
  replies.some(({ mail }) => mail.replyTo === ask.id && mail.from === ask.to && mail.to === ask.from);

export function open({ received, sent, unreachable }: Ledger): Open[] {
  const pending = (ask: Mail, peer: string, replies: readonly Letter[]) =>
    ask.kind === "ask" && !unreachable.includes(peer) && !answered(replies, ask);
  const inbound = received.flatMap(({ mail, parked }): Open[] => {
    const item = { mail, peer: mail.from, name: mail.sender };
    if (parked) return [{ ...item, state: "unread" }];
    return pending(mail, mail.from, sent) ? [{ ...item, state: "owed" }] : [];
  });
  const outbound = sent.flatMap(({ mail, parked }): Open[] => {
    const item = { mail, peer: mail.to, name: mail.recipient };
    if (parked) return [{ ...item, state: "waiting" }];
    return pending(mail, mail.to, received) ? [{ ...item, state: "asked" }] : [];
  });
  return [...inbound, ...outbound].toSorted((a, b) => b.mail.sent - a.mail.sent);
}

export const received = (messages: readonly Message[]) =>
  messages.flatMap((message) => {
    const mail = message.type === "synthetic" ? read(message.metadata) : undefined;
    return mail ? [{ message, mail }] : [];
  });

export const sent = (messages: readonly Message[]) =>
  messages.flatMap((message) =>
    message.type !== "assistant"
      ? []
      : message.content.flatMap((part) => {
          if (part.type !== "tool" || part.name !== source || part.state.status !== "completed") return [];
          const mail = read(part.state.metadata);
          return mail ? [{ message, part, state: part.state, mail }] : [];
        }),
  );
