import { createHash } from "node:crypto";
const BILLING_SALT = "59cf53e54c78";
/**
 * Extract text from the first user message's first text block.
 * Matches Claude Code's K19() function exactly: find the first message
 * with role "user", then return the text of its first text content block.
 */
export function extractFirstUserMessageText(messages) {
    const userMsg = messages.find((m) => m.role === "user");
    if (!userMsg)
        return "";
    const content = userMsg.content;
    if (typeof content === "string")
        return content;
    if (Array.isArray(content)) {
        const textBlock = content.find((b) => b.type === "text");
        if (textBlock && textBlock.type === "text" && textBlock.text) {
            return textBlock.text;
        }
    }
    return "";
}
/**
 * Compute cch: first 5 hex characters of SHA-256(messageText).
 */
export function computeCch(messageText) {
    return createHash("sha256").update(messageText).digest("hex").slice(0, 5);
}
/**
 * Compute the 3-char version suffix.
 * Samples characters at indices 4, 7, 20 from the message text (padding
 * with "0" when the message is shorter), then hashes with the billing salt
 * and version string.
 */
export function computeVersionSuffix(messageText, version) {
    const sampled = [4, 7, 20]
        .map((i) => (i < messageText.length ? messageText[i] : "0"))
        .join("");
    const input = `${BILLING_SALT}${sampled}${version}`;
    return createHash("sha256").update(input).digest("hex").slice(0, 3);
}
/**
 * Build the complete billing header string for insertion into system[0].
 * Format: x-anthropic-billing-header: cc_version=V.S; cc_entrypoint=E; cch=H;
 */
export function buildBillingHeaderValue(messages, version, entrypoint) {
    const text = extractFirstUserMessageText(messages);
    const suffix = computeVersionSuffix(text, version);
    const cch = computeCch(text);
    return (`x-anthropic-billing-header: ` +
        `cc_version=${version}.${suffix}; ` +
        `cc_entrypoint=${entrypoint}; ` +
        `cch=${cch};`);
}
//# sourceMappingURL=signing.js.map