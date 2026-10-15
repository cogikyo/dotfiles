import { convertHTMLToMarkdown, extractTextFromHTML } from "./html.ts";

declare const self: Worker;

self.addEventListener("message", (event: MessageEvent<{ html: string; format: "markdown" | "text" }>) => {
  const { html, format } = event.data;
  self.postMessage(format === "markdown" ? convertHTMLToMarkdown(html) : extractTextFromHTML(html));
});
