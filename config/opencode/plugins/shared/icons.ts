/** Nerd Font glyphs used by the TUI plugin sidebars. */
export const icons = {
  model: "󰯉 ",
  effort: {
    low: "󰤟",
    medium: "󰤢",
    high: "󰤥",
    xhigh: "󰤨",
    max: "",
    auto: "󰙴",
    unknown: "󰤫",
  },
  git: {
    branch: " ",
    ahead: "⮭",
    behind: "⮯",
    staged: "",
    modified: " ",
    untracked: " ",
    deleted: "󰚃 ",
    stashed: "󰸧 ",
    renamed: "󰑕 ",
    conflict: " ",
  },
  context: "㊋",
  agents: "󰯉",
  agentsCore: "",
  subagent: "",
  folder: "",
  folderLibrary: "",
  skill: "",
  skillProject: "󰏗",
  command: "󰘳",
  commandProject: "󰡛",
  partial: "󰈙",
  markdown: "󰍔",
  compacted: "",
  restore: "󰁯",
  readme: "",
  spec: "󱍅",
  lane: {
    idle: "󰏦",
    limited: "󰡴",
  },
  role: {
    build: "󰣪",
    verify: "󰕥",
    review: "󰈈",
    scout: "󰆋",
  },
  spinner: {
    braille: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"],
  },
  progress: ["󰪞", "󰪟", "󰪠", "󰪡", "󰪢", "󰪣", "󰪤", "󰪥", "", ""],
  barFilled: "◉",
  barEmpty: "○",
  error: "󰅜",
} as const;

/** Selects one of ten progress glyphs from a percentage. */
export function progressIcon(percent: number) {
  const index = Math.max(0, Math.min(9, Math.trunc(percent / 10)));
  return icons.progress[index];
}

/** Returns the glyph for a reasoning effort or the unknown-effort glyph. */
export function effortIcon(value: string) {
  if (value === "low") return icons.effort.low;
  if (value === "medium") return icons.effort.medium;
  if (value === "high") return icons.effort.high;
  if (value === "xhigh") return icons.effort.xhigh;
  if (value === "max") return icons.effort.max;
  if (value === "auto") return icons.effort.auto;
  return icons.effort.unknown;
}
