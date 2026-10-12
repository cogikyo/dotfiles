import { Plugin } from "@opencode/plugin";
import { Schema } from "effect";
import { createReadStream } from "node:fs";
import { realpath } from "node:fs/promises";
import { homedir } from "node:os";
import path from "node:path";
import { createInterface } from "node:readline";
import { findBoxes } from "./boxes.ts";
import { Layout } from "./layout.ts";
import { connect, edges } from "./routes.ts";
import { tree } from "./tree.ts";

const lineNumber = (description: string) =>
  Schema.optional(Schema.Int.check(Schema.isGreaterThanOrEqualTo(1)).annotate({ description }));

const Input = Schema.Struct({
  text: Schema.optional(Schema.String.annotate({ description: "Layout rows, without Markdown fence lines." })),
  file: Schema.optional(
    Schema.String.annotate({
      description: "File under the session directory that hosts the layout; use instead of text.",
    }),
  ),
  start: lineNumber("First hosted line in file, 1-based."),
  end: lineNumber("Last hosted line in file, inclusive."),
  prefix: Schema.optional(
    Schema.String.annotate({
      description: "Indentation and comment prefix on every nonblank row, such as `// ` or `  # `; default none.",
    }),
  ),
  width: Schema.optional(
    Schema.Int.check(Schema.isGreaterThanOrEqualTo(1)).annotate({
      description: "Maximum hosted row width in display cells, prefix included; prefix tabs advance to 8-cell stops.",
    }),
  ),
});

const description = [
  "Validate a documentation diagram or annotated directory tree against the diagram skill's display-cell geometry and connectivity rules.",
  "Pass the layout as `text`, or point at the hosted region with `file`, `start`, and `end`.",
  "Returns violations as line:cell positions in the bare layout, then box widths and the traced edges, or the tree's paths.",
].join(" ");

async function inside(directory: string, target: string) {
  const root = await realpath(directory);
  if (root === "/" || root === (await realpath(homedir())))
    throw new Error(`file mode needs a project directory; the session directory is ${root}`);
  const file = await realpath(path.resolve(root, target));
  const relative = path.relative(root, file);
  if (relative === ".." || relative.startsWith(`..${path.sep}`) || path.isAbsolute(relative))
    throw new Error(`${file} is outside the session directory ${root}; pass the layout as text instead`);
  return file;
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

export default Plugin.define({
  id: "diagram",
  async setup(ctx) {
    await ctx.tool.transform((editor) =>
      editor.add({
        name: "diagram",
        description,
        input: Input,
        options: { codemode: false },
        execute: async (args, context) => {
          if (args.file === undefined) {
            if (args.text === undefined) throw new Error("pass text or file");
            const lines = args.text.split(/\r?\n/u);
            if (lines.at(-1) === "") lines.pop();
            return { content: validate(lines, 1, args.prefix ?? "", args.width) };
          }
          if (args.text !== undefined) throw new Error("pass text or file, not both");
          if (args.start === undefined || args.end === undefined) throw new Error("file needs start and end lines");
          if (args.end < args.start) throw new Error("end is before start");
          const session = await ctx.session.get({ sessionID: context.sessionID });
          const file = await inside(session.location.directory, args.file);
          const lines = await region(file, args.start, args.end);
          return { content: validate(lines, args.start, args.prefix ?? "", args.width) };
        },
      }),
    );
  },
});
