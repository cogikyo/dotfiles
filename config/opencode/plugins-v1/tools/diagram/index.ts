import { tool, type ToolContext } from "@opencode-ai/plugin";
import { createReadStream } from "node:fs";
import { realpath } from "node:fs/promises";
import { createInterface } from "node:readline";
import path from "node:path";
import { findBoxes } from "./boxes.ts";
import { Layout } from "./layout.ts";
import { connect, edges } from "./routes.ts";
import { tree } from "./tree.ts";

export const diagram = tool({
  description: [
    "Validate a documentation diagram or annotated directory tree against the diagram skill's display-cell geometry and connectivity rules.",
    "Pass the layout as `text`, or point at the hosted region with `file`, `start`, and `end`.",
    "Returns violations as line:cell positions in the bare layout, then box widths and the traced edges, or the tree's paths.",
  ].join(" "),
  args: {
    text: tool.schema.string().optional().describe("Layout rows, without Markdown fence lines."),
    file: tool.schema.string().optional().describe("File that hosts the layout; use instead of text."),
    start: tool.schema.number().int().min(1).optional().describe("First hosted line in file, 1-based."),
    end: tool.schema.number().int().min(1).optional().describe("Last hosted line in file, inclusive."),
    prefix: tool.schema
      .string()
      .optional()
      .describe("Indentation and comment prefix on every nonblank row, such as `// ` or `  # `; default none."),
    width: tool.schema
      .number()
      .int()
      .min(1)
      .optional()
      .describe("Maximum hosted row width in display cells, prefix included; prefix tabs advance to 8-cell stops."),
  },
  async execute(args, ctx) {
    await ctx.ask({ permission: "diagram", patterns: ["*"], always: ["*"], metadata: {} });
    if (args.file === undefined) {
      if (args.text === undefined) throw new Error("pass text or file");
      const lines = args.text.split(/\r?\n/u);
      if (lines.at(-1) === "") lines.pop();
      return validate(lines, 1, args.prefix ?? "", args.width);
    }
    if (args.text !== undefined) throw new Error("pass text or file, not both");
    if (args.start === undefined || args.end === undefined) throw new Error("file needs start and end lines");
    if (args.end < args.start) throw new Error("end is before start");
    const file = await realpath(path.resolve(ctx.directory, args.file));
    await external(ctx, file);
    await ctx.ask({ permission: "read", patterns: [file], always: ["*"], metadata: {} });
    return validate(await region(file, args.start, args.end), args.start, args.prefix ?? "", args.width);
  },
});

async function external(ctx: ToolContext, file: string) {
  const within = async (root: string) => {
    const relative = path.relative(await realpath(root), file);
    return relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
  };
  if (await within(ctx.directory)) return;
  if (ctx.worktree && ctx.worktree !== "/" && (await within(ctx.worktree))) return;
  const parentDir = path.dirname(file);
  const glob = path.join(parentDir, "*");
  await ctx.ask({
    permission: "external_directory",
    patterns: [glob],
    always: [glob],
    metadata: { filepath: file, parentDir },
  });
}

async function region(file: string, start: number, end: number) {
  const stream = createReadStream(file, { encoding: "utf8" });
  const reader = createInterface({ input: stream, crlfDelay: Infinity });
  const lines: string[] = [];
  let count = 0;
  try {
    for await (const line of reader) {
      if (++count >= start) lines.push(line);
      if (count === end) return lines;
    }
  } finally {
    reader.close();
    stream.destroy();
  }
  throw new Error(`${file} has ${count} lines`);
}

function validate(lines: string[], first: number, prefix: string, width?: number) {
  const layout = new Layout(lines, first, prefix, width);
  const boxes = findBoxes(layout);
  connect(layout, boxes);
  const routed = boxes.list.length > 0 || layout.cells().some(({ r, c }) => layout.tip(r, c) !== 0);
  const summary = routed ? edges(layout, boxes) : tree(layout);
  const sizes = boxes.list.map((box) => `${box.name} ${box.right - box.left + 1}x${box.bottom - box.top + 1}`);
  const { violations } = layout;
  return [
    `${lines.length} rows, widest ${layout.widest.cells} cells at line ${layout.widest.row + first}`,
    violations.length
      ? `violations (${violations.length}):\n${violations.map((v) => `- ${v}`).join("\n")}`
      : "no violations",
    sizes.length ? `boxes: ${sizes.join(", ")}` : "",
    summary,
  ]
    .filter(Boolean)
    .join("\n\n");
}
