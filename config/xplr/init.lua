-- ============================================================================
-- 🧰 Setup: 🢢 {{{
---@diagnostic disable
version = "0.20.2"
local xplr = xplr -- The globally exposed configuration to be overridden.
---@diagnostic enable

-- https://xplr.dev/en/style (shorthand ⮯)
local function style(color, add_mods)
	return {
		fg = color or nil,
		add_modifiers = add_mods or nil,
	}
end

-- more shorthand for common {format, style} pattern
local function format(fmt, color, mods)
	return {
		format = fmt,
		style = {
			fg = color,
			add_modifiers = mods or {},
		},
	}
end

local function panel_format(fmt, color, mods)
	return {
		title = {
			format = fmt,
			style = {
				fg = color,
				add_modifiers = mods or {},
			},
		},
		border_style = {
			fg = color,
			add_modifiers = mods or {},
		},
        style = { fg = color }
	}
end

local azure = { Rgb = { 158, 219, 255 } }
local orange = { Rgb = { 245, 169, 127 } }
local purple = { Rgb = { 194, 170, 252 } }

-- }}} ⮭
-- ============================================================================

-- 🌐 General: ⮯ {{{

local general = {
	disable_debug_error_mode = false,
	enable_mouse = true,
	show_hidden = false,
	read_only = false,
	enable_recover_mode = false,
	hide_remaps_in_help_menu = false,
	enforce_bounded_index_navigation = false,
	prompt = format("  ", "Blue"),

	logs = {
		success = format("SUCCESS", "Green"),
		warning = format("WARNING", "Yellow"),
		error = format("ERROR", "Red"),
	},

	table = {
		header = {
			cols = {
				{ format = "  " },
				{ format = "╭─⼮path" },
				{ format = "⼈⾕⾥" },
				{ format = "猪size ─╮" },
				{ format = " modified" },
			},
			style = style("Green"),
			height = 1,
		},
		row = {
			cols = {
				{ format = "builtin.fmt_general_table_row_cols_0" },
				{ format = "builtin.fmt_general_table_row_cols_1" },
				{ format = "builtin.fmt_general_table_row_cols_2" },
				{ format = "builtin.fmt_general_table_row_cols_3" },
				{ format = "builtin.fmt_general_table_row_cols_4" },
			},
			style = {},
			height = 0,
		},
		style = {},
		tree = {
			{ format = "├", style = {} },
			{ format = "├", style = {} },
			{ format = "╰", style = {} },
		},
		col_spacing = 1,
		col_widths = {
			{ Percentage = 4 },
			{ Percentage = 57 },
			{ Percentage = 4 },
			{ Percentage = 11 },
			{ Percentage = 21 },
		},
	},

	default_ui = {
		prefix = " ",
		suffix = "",
		style = {},
	},

	focus_ui = {
		prefix = "⌬ ",
		suffix = "",
		style = style(orange, { "Bold" }),
	},

	selection_ui = {
		prefix = "⟪─── ",
		suffix = " ─╼╾ ⼶",
		style = style("Cyan", { "Dim" }),
	},

	focus_selection_ui = {
		prefix = "⌬ ⟪─── ",
		suffix = " ─╼╾ ⼶",
		style = style(orange, { "Bold" }),
	},

	sort_and_filter_ui = {
		separator = format(" ⇨ ", "DarkGray"),
		sort_direction_identifiers = {
			forward = format("⮯", "Magenta"),
			reverse = format("⮭", "Cyan"),
		},
		sorter_identifiers = {
			ByExtension = { format = "ext", style = {} },
			ByICanonicalAbsolutePath = { format = "[ci]abs", style = {} },
			ByIRelativePath = format("⻚", "Magenta"),
			ByISymlinkAbsolutePath = { format = "[si]abs", style = {} },
			ByIsBroken = { format = "⨯", style = {} },
			ByIsDir = { format = "dir", style = {} },
			ByIsFile = { format = "file", style = {} },
			ByIsReadonly = { format = "ro", style = {} },
			ByIsSymlink = { format = "sym", style = {} },
			ByMimeEssence = { format = "mime", style = {} },
			ByRelativePath = { format = "rel", style = {} },
			BySize = { format = "size", style = {} },
			ByCreated = { format = "created", style = {} },
			ByLastModified = { format = "modified", style = {} },
			ByCanonicalAbsolutePath = { format = "[c]abs", style = {} },
			ByCanonicalExtension = { format = "[c]ext", style = {} },
			ByCanonicalIsDir = format("⽊", "Cyan"),
			ByCanonicalIsFile = { format = "[c]file", style = {} },
			ByCanonicalIsReadonly = { format = "[c]ro", style = {} },
			ByCanonicalMimeEssence = { format = "[c]mime", style = {} },
			ByCanonicalSize = { format = "[c]size", style = {} },
			ByCanonicalCreated = { format = "[c]created", style = {} },
			ByCanonicalLastModified = { format = "[c]modified", style = {} },
			BySymlinkAbsolutePath = { format = "[s]abs", style = {} },
			BySymlinkExtension = { format = "[s]ext", style = {} },
			BySymlinkIsDir = { format = "[s]dir", style = {} },
			BySymlinkIsFile = { format = "[s]file", style = {} },
			BySymlinkIsReadonly = { format = "[s]ro", style = {} },
			BySymlinkMimeEssence = { format = "[s]mime", style = {} },
			BySymlinkSize = { format = "[s]size", style = {} },
			BySymlinkCreated = { format = "[s]created", style = {} },
			BySymlinkLastModified = { format = "[s]modified", style = {} },
		},
		filter_identifiers = {
			RelativePathDoesContain = { format = "rel=~", style = {} },
			RelativePathDoesEndWith = { format = "rel=$", style = {} },
			RelativePathDoesNotContain = { format = "rel!~", style = {} },
			RelativePathDoesNotEndWith = { format = "rel!$", style = {} },
			RelativePathDoesNotStartWith = format("  ", "White"),
			RelativePathDoesStartWith = { format = "rel=^", style = {} },
			RelativePathIs = { format = "rel==", style = {} },
			RelativePathIsNot = { format = "rel!=", style = {} },
			RelativePathDoesMatchRegex = { format = "rel=/", style = {} },
			RelativePathDoesNotMatchRegex = { format = "rel!/", style = {} },
			IRelativePathDoesContain = { format = "[i]rel=~", style = {} },
			IRelativePathDoesEndWith = { format = "[i]rel=$", style = {} },
			IRelativePathDoesNotContain = { format = "[i]rel!~", style = {} },
			IRelativePathDoesNotEndWith = { format = "[i]rel!$", style = {} },
			IRelativePathDoesNotStartWith = { format = "[i]rel!^", style = {} },
			IRelativePathDoesStartWith = { format = "[i]rel=^", style = {} },
			IRelativePathIs = { format = "[i]rel==", style = {} },
			IRelativePathIsNot = { format = "[i]rel!=", style = {} },
			IRelativePathDoesMatchRegex = { format = "[i]rel=/", style = {} },
			IRelativePathDoesNotMatchRegex = { format = "[i]rel!/", style = {} },
			AbsolutePathDoesContain = { format = "abs=~", style = {} },
			AbsolutePathDoesEndWith = { format = "abs=$", style = {} },
			AbsolutePathDoesNotContain = { format = "abs!~", style = {} },
			AbsolutePathDoesNotEndWith = { format = "abs!$", style = {} },
			AbsolutePathDoesNotStartWith = { format = "abs!^", style = {} },
			AbsolutePathDoesStartWith = { format = "abs=^", style = {} },
			AbsolutePathIs = { format = "abs==", style = {} },
			AbsolutePathIsNot = { format = "abs!=", style = {} },
			AbsolutePathDoesMatchRegex = { format = "abs=/", style = {} },
			AbsolutePathDoesNotMatchRegex = { format = "abs!/", style = {} },
			IAbsolutePathDoesContain = { format = "[i]abs=~", style = {} },
			IAbsolutePathDoesEndWith = { format = "[i]abs=$", style = {} },
			IAbsolutePathDoesNotContain = { format = "[i]abs!~", style = {} },
			IAbsolutePathDoesNotEndWith = { format = "[i]abs!$", style = {} },
			IAbsolutePathDoesNotStartWith = { format = "[i]abs!^", style = {} },
			IAbsolutePathDoesStartWith = { format = "[i]abs=^", style = {} },
			IAbsolutePathIs = { format = "[i]abs==", style = {} },
			IAbsolutePathIsNot = { format = "[i]abs!=", style = {} },
			IAbsolutePathDoesMatchRegex = { format = "[i]abs=/", style = {} },
			IAbsolutePathDoesNotMatchRegex = { format = "[i]abs!/", style = {} },
		},
		search_identifier = {
			format = " ",
			style = style(orange, { "Bold" }),
		},
	},
	panel_ui = {
		default = {
			title = {
				format = nil,
				style = style("Blue", { "Bold" }),
			},
			style = {},
			borders = {
				"Top",
				"Right",
				"Bottom",
				"Left",
			},
			border_type = "Rounded",
			border_style = { fg = "DarkGray" },
		},
		table = panel_format(nil, "Blue", { "Bold" }),
		help_menu = panel_format(
			"─╼╾ 何 Help ╼╾",
			"Magenta",
			{ "Bold" }
		),
		input_and_logs = panel_format(
			"─╼╾  Input ╼╾  Logs ╼╾",
			"Blue",
			{ "Dim" }
		),
		selection = panel_format(
			"─╼╾ ⼶ Selection ╼╾",
			"Cyan",
			{ "Dim" }
		),
		sort_and_filter = panel_format(
			"─╼╾  Filter ╼╾ Sort  ╼╾",
			"Blue",
			{ "Dim" }
		),
	},

	initial_sorting = {
		{ sorter = "ByCanonicalIsDir", reverse = true },
		{ sorter = "ByIRelativePath", reverse = false },
	},

	initial_mode = "default",
	initial_layout = "default",
	start_fifo = nil,

	global_key_bindings = {
		on_key = {
			["esc"] = {
				messages = {
					"PopMode",
				},
			},
			["ctrl-c"] = {
				messages = {
					"Terminate",
				},
			},
		},
	},
}

-- 🧮 ZHU LI, DO THE THING!
for key, val in pairs(general) do
	xplr.config.general[key] = val
end
-- }}} ⮭

-- ✳️  Nodes Types: ⮯ {{{

local function meta(icon, color, mods)
	return {
		style = {
			fg = color,
			add_modifiers = mods or nil,
		},
		meta = { icon = icon },
	}
end

local node_types = {
	directory = meta(" ", "Blue"),
	file = meta(" ", "Reset"),
	symlink = meta(" ", azure),
	mime_essence = {
		audio = {
			["*"] = meta(" ", "Magenta"),
		},
		video = {
			["*"] = meta(" ", purple),
		},
		image = {
			["*"] = meta(" ", "Magenta", { "Dim" }),
		},
		application = {
			["x-pie-executable"] = meta(" ", "Green"),
			["x-shellscript"] = meta(" ", "Green"),
			["x-executable"] = meta(" ", "Green"),
			["*"] = meta(" ", "Green"),
		},
		text = {
			["*"] = meta(" ", "Reset"),
		},
	},
	extension = {
		md = meta(" ", "Yellow"),
		toml = meta(" "),
		conf = meta(" "),
		json = meta(" "),
		list = meta(" "),
		lst = meta(" "),
		dirs = meta(" "),
		gz = meta(" ", "White"),
		zip = meta(" ", "White"),
		desktop = meta(" "),
		rules = meta(" ", "Red", { "Dim" }),
		lua = meta(" ", orange, { "Dim" }),
		rs = meta(" ", orange, { "Dim" }),
		py = meta(" ", orange, { "Dim" }),
		scss = meta(" ", orange, { "Dim" }),
		css = meta(" ", orange, { "Dim" }),
	},
	special = {
		downloads = meta(" "),
		dotfiles = meta("🍙"),
		docs = meta(" "),
		books = meta(" "),
		papers = meta("📜"),
		templates = meta(" "),
		notes = meta("📚"),
		media = meta(" "),
		vagari = meta("🧬"),
		share = meta("⾦"),
		etc = meta("⽳"),
		bin = meta("⼡"),
		usr = meta("⼈"),
		home = meta("⾕", orange),
		cullyn = meta("⾕"),
		config = meta(" "),
		LICENSE = meta(" ", "DarkGray"),
	},
}

-- 🧮 ZHU LI, DO THE THING!
for key, val in pairs(node_types) do
	xplr.config.node_types[key] = val
end
-- }}} ⮭

-- 🪟 Layout: ⮯ {{{
-- Type: [Layout](https://xplr.dev/en/layout)
xplr.config.layouts.builtin.default = {
	Horizontal = {
		config = {
			margin = 1,
			horizontal_margin = 2,
			constraints = { { Percentage = 100 } },
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Min = 1 },
							{ Length = 15 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						"Selection",
						"InputAndLogs",
					},
				},
			},
		},
	},
}

-- The layout without help menu
--
-- Type: [Layout](https://xplr.dev/en/layout)
xplr.config.layouts.builtin.no_help = {
	Horizontal = {
		config = {
			constraints = {
				{ Percentage = 70 },
				{ Percentage = 30 },
			},
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Min = 1 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						"InputAndLogs",
					},
				},
			},
			"Selection",
		},
	},
}

-- The layout without selection panel
--
-- Type: [Layout](https://xplr.dev/en/layout)
xplr.config.layouts.builtin.no_selection = {
	Horizontal = {
		config = {
			constraints = {
				{ Percentage = 70 },
				{ Percentage = 30 },
			},
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Min = 1 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						"InputAndLogs",
					},
				},
			},
			"HelpMenu",
		},
	},
}

-- The layout without help menu and selection panel
--
-- Type: [Layout](https://xplr.dev/en/layout)
xplr.config.layouts.builtin.no_help_no_selection = {
	Vertical = {
		config = {
			constraints = {
				{ Length = 3 },
				{ Min = 1 },
				{ Length = 3 },
			},
		},
		splits = {
			"SortAndFilter",
			"Table",
			"InputAndLogs",
		},
	},
}

-- This is where you can define custom layouts
--
-- Type: mapping of the following key-value pairs:
--
-- * key: string
-- * value: [Layout](https://xplr.dev/en/layout)
--
-- Example:
--
-- ```lua
-- xplr.config.layouts.custom.example = "Nothing" -- Show a blank screen
-- xplr.config.general.initial_layout = "example" -- Load the example layout
-- ```
xplr.config.layouts.custom = {}
-- }}} ⮭

-- ============================================================================
-- 📦 XPM, PREP THE THING! 🢢 {{{
local home = os.getenv("HOME")
local xpm_path = home .. "/.local/share/xplr/dtomvan/xpm.xplr"
local xpm_url = "https://github.com/dtomvan/xpm.xplr"

package.path = package.path
	.. ";"
	.. xpm_path
	.. "/?.lua;"
	.. xpm_path
	.. "/?/init.lua"

os.execute(
	string.format(
		"[ -e '%s' ] || git clone '%s' '%s'",
		xpm_path,
		xpm_url,
		xpm_path
	)
)
-- }}}

-- 🔌 Selected Plugins: ⮯ {{{
require("xpm").setup({
	plugins = { -- https://xplr.dev/en/awesome-plugins
		"dtomvan/xpm.xplr",
		"sayanarijit/wl-clipboard.xplr",
		"sayanarijit/fzf.xplr",
		"sayanarijit/zoxide.xplr",
		"sayanarijit/trash-cli.xplr",
		"Junker/nuke.xplr",
		"dtomvan/ouch.xplr",
	},
	auto_install = true,
	auto_cleanup = true,
})
-- }}} ⮭

-- ⚙️  Nonstandard Plugin Configurations: ⮯ {{{

require("ouch").setup({
	mode = "default",
	key = "o",
})

require("zoxide").setup({ key = "z" })

require("fzf").setup({
	key = "t",
	recursive = true,
	enter_dir = true,
})

require("nuke").setup({
	pager = "$PAGER",
	open = {
		run_executables = false,
		custom = {
			{ mime_regex = ".*", command = "xdg-open {}" },
		},
	},
})

local key = xplr.config.modes.builtin.default.key_bindings.on_key
local nukey = xplr.config.modes.custom.nuke.key_bindings.on_key

key.v = nukey.v
key["right"] = nukey.o

-- }}}
-- ============================================================================
