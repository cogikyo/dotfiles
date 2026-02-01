-- ============================================================================
-- 🧰 Setup: {{{
---@diagnostic disable
version = "1.0.0"
local xplr = xplr -- The globally exposed configuration to be overridden.
---@diagnostic enable

-- https://xplr.dev/en/style (shorthand ⮯)
local function style(color, add_mods)
	return {
		fg = color or nil,
		add_modifiers = add_mods or nil,
	}
end

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
		style = { fg = color },
	}
end

local azure = { Rgb = { 158, 219, 255 } }
local orange = { Rgb = { 245, 169, 127 } }
local purple = { Rgb = { 194, 170, 252 } }

-- }}}
-- ============================================================================

-- 🌐 General {{{

local general = {
	disable_debug_error_mode = false,
	enable_mouse = true,
	show_hidden = false,
	read_only = false,
	enable_recover_mode = false,
	hide_remaps_in_help_menu = false,
	enforce_bounded_index_navigation = false,
	prompt = format("  ", orange),

	logs = {
		success = format("SUCCESS", "Green"),
		warning = format("WARNING", "Yellow"),
		error = format("ERROR", "Red"),
	},

	table = {
		header = {
			cols = {
				{ format = "   󱣱 " },
				{
					format = "╭╼╾⼮path ──────────────────────────────────────────────────────────────────────────────╮",
				},
				{ format = "   " },
				{ format = "󰖡 size ─╮" },
				{
					format = "  modified ────────────╮",
				},
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

	selection = {
		item = {
			format = "builtin.fmt_general_selection_item",
			style = style("Cyan", { "Dim" }),
		},
	},

	search = {
		algorithm = "Fuzzy",
		unordered = false,
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
			RelativePathDoesNotStartWith = format(" 󰊠 ", "White"),
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
		search_identifiers = {
			Fuzzy = {
				format = " ",
				style = style(orange, { "Bold" }),
			},
			Regex = {
				format = " ",
				style = style(orange, { "Bold" }),
			},
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
		table = panel_format(
			-- "──────────────────────────╼╾ ⽊ Directory ╼╾",
			nil,
			"Blue",
			{ "Bold" }
		),
		help_menu = panel_format("─╼╾ 何 Help ╼╾", "Magenta", { "Dim" }),
		input_and_logs = panel_format(
			"──────────────────────────────────────────────────────────────╼╾  Input ╼╾  Logs ╼╾",
			"Blue",
			{ "Dim" }
		),
		selection = panel_format("─╼╾ ⼶ Selection ╼╾", "Cyan", { "Dim" }),
		sort_and_filter = panel_format(
			"──────────────────────────────────────────────────────────────╼╾  Filter ╼╾ Sort  ╼╾",
			"Blue",
			{ "Dim" }
		),
	},

	initial_sorting = {
		{ sorter = "ByCanonicalIsDir", reverse = true },
		{ sorter = "ByIRelativePath", reverse = false },
	},

	initial_mode = "preview_mode",
	initial_layout = "preview",
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

for key, val in pairs(general) do
	xplr.config.general[key] = val
end

-- }}}

-- ✳️  Nodes Types {{{

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
	file = meta(" ", "White"),
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
			["*"] = meta("󰶭 ", "Green"),
		},
		text = {
			["*"] = meta(" ", "White"),
		},
	},
	extension = {
		md = meta(" ", azure, { "Dim" }),
		toml = meta(" "),
		conf = meta(" "),
		json = meta(" "),
		list = meta(" "),
		lst = meta(" "),
		dirs = meta(" "),
		gz = meta(" ", "White"),
		zip = meta(" ", "White"),
		desktop = meta("󱕷 "),
		rules = meta(" ", "Red", { "Dim" }),
		lua = meta(" "),
		rs = meta("󱘗 "),
		py = meta(" "),
		scss = meta("󰟬 "),
		css = meta(" "),
		html = meta(" "),
	},
	special = {
		downloads = meta(" "),
		dotfiles = meta("🍙"),
		docs = meta(" "),
		books = meta(" "),
		cmd = meta(" "),
		templates = meta(" "),
		notes = meta("📚"),
		media = meta("󰈯 "),
		share = meta(" "),
		music = meta(" "),
		gifs = meta("󰤺 "),
		screenshots = meta(" "),
		images = meta("󰋯 "),
		videos = meta(" "),
		recordings = meta("󰕧 "),
		etc = meta("󱁽 "),
		bin = meta("⼡"),
		usr = meta("⼈"),
		home = meta("⾕", orange),
		cullyn = meta("⾕"),
		config = meta(" "),
		LICENSE = meta(" ", "DarkGray"),
	},
}

for key, val in pairs(node_types) do
	xplr.config.node_types[key] = val
end
-- }}}

-- 🪟 Layout {{{
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
							{ Min = 10 },
							{ Max = 10 },
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

xplr.config.layouts.builtin.no_selection = {
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
							{ Min = 5 },
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
		},
	},
}

-- }}}

-- 🎛️ Modes {{{

local modes = xplr.config.modes.builtin
local on_key = modes.default.key_bindings.on_key

modes.create_directory.prompt = "  (create dir)  "
modes.create_file.prompt = "  (create file)  "
modes.number.prompt = "  "
modes.rename.prompt = "  (rename)  "

modes.switch_layout = {
	name = "switch layout",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["s"] = {
				help = "selection",
				messages = {
					{ SwitchLayoutBuiltin = "default" },
					"PopMode",
				},
			},
			["n"] = {
				help = "no selection",
				messages = {
					{ SwitchLayoutBuiltin = "no_selection" },
					"PopMode",
				},
			},
		},
	},
}

on_key["m"] = {
	help = "bookmark",
	messages = {
		{
			BashExecSilently0 = [===[
        PTH="${XPLR_FOCUS_PATH:?}"
        PTH_ESC=$(printf %q "$PTH")
        if echo "${PTH:?}" >> "${XPLR_SESSION_PATH:?}/bookmarks"; then
          "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC added to bookmarks"
        else
          "$XPLR" -m 'LogError: %q' "Failed to bookmark $PTH_ESC"
        fi
      ]===],
		},
	},
}

on_key["`"] = {
	help = "go to bookmark",
	messages = {
		{
			BashExec0 = [===[
        PTH=$(cat "${XPLR_SESSION_PATH:?}/bookmarks" | fzf --no-sort)
        PTH_ESC=$(printf %q "$PTH")
        if [ "$PTH" ]; then
          "$XPLR" -m 'FocusPath: %q' "$PTH"
        fi
      ]===],
		},
	},
}

on_key["R"] = {
	help = "batch rename",
	messages = { { BashExec = [===[ renamer ]===] } },
}

-- }}}

-- ============================================================================
-- 📦 Plugins Setup:
local home = os.getenv("HOME")

-- Add plugins path for preview.xplr (manual installs)
package.path = home .. "/.config/xplr/plugins/?/init.lua;" .. home .. "/.config/xplr/plugins/?.lua;" .. package.path

local xpm_path = home .. "/.local/share/xplr/dtomvan/xpm.xplr"
local xpm_url = "https://github.com/dtomvan/xpm.xplr"

package.path = package.path .. ";" .. xpm_path .. "/?.lua;" .. xpm_path .. "/?/init.lua"

os.execute(string.format("[ -e '%s' ] || git clone '%s' '%s'", xpm_path, xpm_url, xpm_path))
-- }}}

-- 🔌 Selected Plugins {{{
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
-- }}}

-- ⚙️  Nonstandard Plugin Configurations {{{

require("ouch").setup({
	mode = "default",
	key = "o",
})

require("zoxide").setup({ key = "z" })

local ok, trash = pcall(require, "trash-cli")
if ok then
	trash.setup()
end

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
			{ extension = "gz", command = "tar tf {} | nvimpager" },
			{ mime_regex = ".*", command = "xdg-open {}" },
		},
	},
})

local nuke_on_key = xplr.config.modes.custom.nuke.key_bindings.on_key

on_key["v"] = nuke_on_key.v
on_key["right"] = nuke_on_key.o

-- 👁️ Preview Plugin
require("preview").setup({
	as_default = false,
	keybind = "P",
	left_pane_constraint = { Percentage = 55 },
	right_pane_constraint = { Percentage = 45 },
	style = false,
	text = {
		enable = true,
		highlight = {
			enable = false,
			method = "truecolor",
			style = nil,
		},
	},
	image = {
		enable = false, -- disabled: causes xplr panic
		method = "kitty",
	},
	directory = {
		enable = true,
	},
})

-- Override plugin's layout (Table above, Preview below)
xplr.config.layouts.custom.preview = {
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
							{ Percentage = 50 },
							{ Percentage = 50 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						{
							CustomContent = {
								title = "  Preview",
								body = {
									DynamicParagraph = {
										render = "custom.preview.render",
									},
								},
							},
						},
						"InputAndLogs",
					},
				},
			},
		},
	},
}

-- Layout with Selection panel instead of Preview (for when items are selected)
xplr.config.layouts.custom.selection = {
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
							{ Percentage = 50 },
							{ Percentage = 50 },
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

-- Helper to copy all keybindings from default mode
local function inherit_default_keys()
	local keys = {}
	for k, v in pairs(on_key) do
		keys[k] = v
	end
	return keys
end

-- Custom preview mode (default startup mode - shows preview panel)
local preview_keys = inherit_default_keys()
preview_keys["P"] = {
	help = "close preview",
	messages = {
		{ SwitchLayoutBuiltin = "default" },
		{ SwitchModeBuiltin = "default" },
	},
}
preview_keys["space"] = {
	help = "toggle selection",
	messages = {
		"ToggleSelection",
		{ SwitchLayoutCustom = "selection" },
		{ SwitchModeCustom = "selection_mode" },
	},
}

xplr.config.modes.custom.preview_mode = {
	name = "preview",
	key_bindings = {
		on_key = preview_keys,
	},
}

-- Selection mode (shows selection panel instead of preview)
local selection_keys = inherit_default_keys()
selection_keys["P"] = {
	help = "back to preview",
	messages = {
		{ SwitchLayoutCustom = "preview" },
		{ SwitchModeCustom = "preview_mode" },
	},
}
selection_keys["space"] = {
	help = "toggle selection",
	messages = {
		"ToggleSelection",
		-- Check if selection is empty via bash, switch back to preview
		{
			BashExecSilently0 = [===[
				if [ -z "$XPLR_SELECTION" ]; then
					"$XPLR" -m 'SwitchLayoutCustom: preview' -m 'SwitchModeCustom: preview_mode'
				fi
			]===],
		},
	},
}
selection_keys["u"] = {
	help = "clear selection",
	messages = {
		"ClearSelection",
		{ SwitchLayoutCustom = "preview" },
		{ SwitchModeCustom = "preview_mode" },
	},
}

xplr.config.modes.custom.selection_mode = {
	name = "selection",
	key_bindings = {
		on_key = selection_keys,
	},
}

-- Toggle preview with P from default mode
on_key["P"] = {
	help = "toggle preview",
	messages = {
		{ SwitchLayoutCustom = "preview" },
		{ SwitchModeCustom = "preview_mode" },
	},
}

-- }}}
-- ============================================================================
