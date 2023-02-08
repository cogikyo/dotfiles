---@diagnostic disable
version = "0.20.2"
local xplr = xplr -- The globally exposed configuration to be overridden.
---@diagnostic enable

-- 🌐 General: ⮯ {{{

local function styled(color, mods)
    return { fg = color or nil, add_modifiers = mods or nil }
end

local function format(fmt, style)
    return {
        format = fmt or nil,
        style = style or nil
    }
end

-- https://github.com/sayanarijit/xplr/blob/main/src/init.lua
local general = {
	enable_mouse = true,
	show_hidden = false,
	prompt = format("  ", styled("Blue")),
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
        style = styled("Blue", { "Dim" }),
	},
	focus_selection_ui = {
		prefix = " [",
		suffix = "]",
		style = styled("Blue", { "Bold" }),
	},
	sort_and_filter_ui = {
		separator = format("  ", styled("DarkGray")),
		sort_direction_identifiers = {
			forward = format("⮯", styled("Blue")),
			reverse = format("⮭", styled("Red")),
		},
		sorter_identifiers = {
			ByExtension = format(".", styled("Green")),
			ByICanonicalAbsolutePath = format("⨓", styled("Yellow")),
			ByIRelativePath = format("∫", styled("White")),
			ByISymlinkAbsolutePath = format("", styled("Megenta")),
			ByIsBroken = format("", styled("")),
			ByIsDir = format("", styled("")),
			ByIsFile = format("", styled("")),
			ByIsReadonly = format("", styled("")),
			ByIsSymlink = format("", styled("")),
			ByMimeEssence = format("", styled("")),
			ByRelativePath = format("", styled("")),
			BySize = format("", styled("")),
			ByCreated = format("", styled("")),
			ByLastModified = format("", styled("")),
			ByCanonicalAbsolutePath = format("", styled("")),
			ByCanonicalExtension = format("", styled("")),
			ByCanonicalIsDir = format("", styled("")),
			ByCanonicalIsFile = format("", styled("")),
			ByCanonicalIsReadonly = format("", styled("")),
			ByCanonicalMimeEssence = format("", styled("")),
			ByCanonicalSize = format("", styled("")),
			ByCanonicalCreated = format("", styled("")),
			ByCanonicalLastModified = format("", styled("")),
			BySymlinkAbsolutePath = format("", styled("")),
			BySymlinkExtension = format("", styled("")),
			BySymlinkIsDir = format("", styled("")),
			BySymlinkIsFile = format("", styled("")),
			BySymlinkIsReadonly = format("", styled("")),
			BySymlinkMimeEssence = format("", styled("")),
			BySymlinkSize = format("", styled("")),
			BySymlinkCreated = format("", styled("")),
			BySymlinkLastModified = format("", styled("")),
		},
	},
}

xplr.config.general.table.header.cols = {
	{ format = "  ", style = {} },
	{ format = "╭⨒ path", style = {} },
	{ format = "usgrot", style = {} },
	{ format = "猪size ─╮", style = {} },
	{ format = " modified", style = {} },
}

xplr.config.general.table.col_widths = {
	{ Percentage = 5 },
	{ Percentage = 55 },
	{ Percentage = 5 },
	{ Percentage = 15 },
	{ Percentage = 20 },
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
