// ╭───────────────────────────────────────────────────────────────────────────────────────────────╮
// │ Sidebar glyphs                                                                                │
// ╰───────────────────────────────────────────────────────────────────────────────────────────────╯

/** Sidebar glyphs; some include a trailing space before the label. */
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
    build: "󱢇 ",
    verify: "󰕥 ",
    review: "󰈈 ",
    scout: "󰆋 ",
  },
  scope: {
    general: "♞",
    owner: "󰡚",
    patch: "󰶯",
    scribe: "󰴓",
    git: "󰘬",
    debug: "󰃤",
    architect: "󰒪",
    simplify: "󰆐",
    critic: "󰊛",
    design: "󰏘",
    security: "󰌾",
    profile: "󰓅",
    context: "󰍍",
    library: "󰌱",
    dirty: "󰷈",
    session: "󰋚",
    web: "󰖟",
    source: "󰅩",
    test: "󰂓",
    browser: "󰳽",
  },
  spinner: {
    braille: ["⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"],
  },
  progress: ["󰪞", "󰪟", "󰪠", "󰪡", "󰪢", "󰪣", "󰪤", "󰪥", "", ""],
  barFilled: "◉",
  barEmpty: "○",
  error: "󰅜",
} as const;

// ├─ Selectors ───────────────────────────────────────────────────────────────────────────────────┤

/** Selects a progress glyph in 10% steps, clamping values outside the range. */
export function progressIcon(percent: number) {
  const index = Math.max(0, Math.min(9, Math.trunc(percent / 10)));
  return icons.progress[index];
}

/** Returns a reasoning-effort glyph, or the unknown glyph for an unrecognized effort. */
export function effortIcon(value: string) {
  if (value === "low") return icons.effort.low;
  if (value === "medium") return icons.effort.medium;
  if (value === "high") return icons.effort.high;
  if (value === "xhigh") return icons.effort.xhigh;
  if (value === "max") return icons.effort.max;
  if (value === "auto") return icons.effort.auto;
  return icons.effort.unknown;
}
