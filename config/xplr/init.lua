---@diagnostic disable
version = "0.20.2"
local xplr = xplr -- The globally exposed configuration to be overridden.
---@diagnostic enable

-- 🌐 General: ⮯ {{{

local orange = {}
-- https://xplr.dev/en/style (shorthand ⮯)
local function style(fg, add_mods)
	return {
		fg = fg or nil,
		add_modifiers = add_mods or nil,
	}
end

-- more shorthand for common {format, style} pattern
local function format(fmt, color)
	return {
		format = fmt or nil,
		style = { fg = color } or {},
	}
end

-- https://github.com/sayanarijit/xplr/blob/main/src/init.lua
local general = {
	disable_debug_error_mode = true,
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
				{ format = "╭──⨒ path" },
				{ format = "⼈⾕⾥" },
				{ format = "猪size ─╮" },
				{ format = " modified" },
			},
			style = { fg = "Blue" },
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
			{ Percentage = 5 },
			{ Percentage = 55 },
			{ Percentage = 5 },
			{ Percentage = 15 },
			{ Percentage = 20 },
		},
	},
	default_ui = {
		prefix = " ",
		suffix = "",
		style = {},
	},
	focus_ui = {
		prefix = "⌬ ╼( ",
		suffix = " )╾",
		style = {
			fg = { Rgb = { 245, 169, 127 } },
		},
	},
	selection_ui = {
		prefix = "⟪ ",
		suffix = " ⟫",
		style = style("Cyan", { "Bold" }),
	},
	focus_selection_ui = {
		prefix = " ╼[",
		suffix = "]╾",
		style = {
			fg = {
				Rgb = { 245, 169, 127 },
			},
			add_modifiers = { "Bold" },
		},
	},
	sort_and_filter_ui = {
		separator = format("  ", "DarkGray"),
		sort_direction_identifiers = {
			forward = format("⮯", "Blue"),
			reverse = format("⮭", "Red"),
		},
		sorter_identifiers = {
			ByExtension = { format = "ext", style = {} },
			ByICanonicalAbsolutePath = { format = "[ci]abs", style = {} },
			ByIRelativePath = { format = "[i]rel", style = {} },
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
			ByCanonicalIsDir = { format = "[c]dir", style = {} },
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
			RelativePathDoesNotStartWith = { format = "rel!^", style = {} },
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
			format = "  ",
			style = { fg = { Rgb = { 245, 169, 127 } } },
		},
	},
	-- panel_ui = {
	-- 	default = {
	-- 		title = format(nil),
	-- 	},
	-- },
}

-- 🧮 ZHU LI, DO THE THING!
for key, val in pairs(general) do
	xplr.config.general[key] = val
end
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
		"prncss-xyz/icons.xplr",
		{
			"dtomvan/extra-icons.xplr",
			after = function()
				xplr.config.general.table.row.cols[2] = {
					format = "custom.icons_dtomvan_col_1",
				}
			end,
		},
	},
	auto_install = true,
	auto_cleanup = true,
})
-- }}} ⮭

-- ⚙️  Nonstandard Configurations: ⮯ {{{

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
