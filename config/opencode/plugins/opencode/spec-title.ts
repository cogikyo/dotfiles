import type { Plugin, PluginModule } from "@opencode-ai/plugin";
import { tool } from "@opencode-ai/plugin";
import { realpath, stat } from "node:fs/promises";
import path from "node:path";

const id = "opencode-spec-title";
const TITLE_PATTERN = /^[A-Z0-9]+(?:-[A-Z0-9]+)*(?: [A-Z0-9]+(?:-[A-Z0-9]+)*){3}(?![\s\S])/;

const server: Plugin = async ({ client, directory, worktree }) => {
  const projectPath = worktree || directory;

  return {
    tool: {
      spec_title: tool({
        description: "Name the current root session after an existing project .spec Markdown file. Title must be exactly four ALL-CAPS or hyphenated words separated by single ASCII spaces and at most 28 characters.",
        args: {
          path: tool.schema.string().describe("Existing project-relative or absolute .spec/*.md path"),
          title: tool.schema.string().describe("Exactly four ALL-CAPS words separated by single ASCII spaces, <= 28 characters"),
        },
        async execute(args, context) {
          const title = args.title;
          if (title.length > 28 || !TITLE_PATTERN.test(title)) {
            throw new Error("spec_title title must be exactly four ALL-CAPS words, <= 28 chars");
          }

          const root = context.worktree || projectPath;
          await validateSpecPath(root, args.path);

          const session = await unwrap<Record<string, unknown>>(
            client.session.get({ path: { id: context.sessionID } } as never),
            "read current session",
          );
          if (typeof session.parentID === "string" && session.parentID !== "") {
            throw new Error("spec_title is only available in root sessions");
          }

          await unwrap(
            client.session.update({ path: { id: context.sessionID }, body: { title } } as never),
            "rename current session",
          );
          return `session titled ${title}`;
        },
      }),
    },
  };
};

async function validateSpecPath(projectPath: string, inputPath: string) {
  const requested = inputPath.trim();
  if (!requested || path.extname(requested).toLowerCase() !== ".md") {
    throw new Error("spec_title path must be an existing .spec Markdown file");
  }

  const lexicalRoot = path.resolve(projectPath);
  const candidate = path.isAbsolute(requested) ? path.resolve(requested) : path.resolve(lexicalRoot, requested);
  if (!isProjectPath(lexicalRoot, candidate)) {
    throw new Error("spec_title path must resolve inside this project");
  }

  let root: string;
  let target: string;
  try {
    root = await realpath(lexicalRoot);
    target = await realpath(candidate);
  } catch {
    throw new Error("spec_title path must exist");
  }

  if (!isProjectPath(root, target)) {
    throw new Error("spec_title path must resolve inside this project");
  }
  if (path.extname(target).toLowerCase() !== ".md") {
    throw new Error("spec_title path must be an existing .spec Markdown file");
  }
  const relative = path.relative(root, target);
  if (!relative.split(path.sep).includes(".spec")) {
    throw new Error("spec_title path must contain a .spec segment");
  }

  const info = await stat(target);
  if (!info.isFile()) throw new Error("spec_title path must be a regular file");
}

function isProjectPath(root: string, target: string) {
  const relative = path.relative(root, target);
  return relative !== "" && relative !== ".." && !relative.startsWith(`..${path.sep}`) && !path.isAbsolute(relative);
}

async function unwrap<T = unknown>(promise: Promise<unknown>, label: string): Promise<T> {
  const response = await promise;
  const envelope = typeof response === "object" && response !== null ? response as Record<string, unknown> : undefined;
  if (envelope && "error" in envelope && envelope.error !== undefined) throw new Error(`${label} failed`);
  if (envelope && "data" in envelope) return envelope.data as T;
  return response as T;
}

export default { id, server } satisfies PluginModule;
