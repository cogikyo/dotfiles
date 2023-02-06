---@diagnostic disable
version = "0.20.2"
local xplr = xplr -- The globally exposed configuration to be overridden.
---@diagnostic enable

-- only non-default settings inclucded
local general = {
	disable_debug_error_mode = false,
	enable_mouse = true,
	show_hidden = false,
	enable_recover_mode = true,
	prompt = {
		format = "  ",
	},
}

-- 🧮 ZHU LI, DO THE THING!
for key, val in pairs(general) do
	xplr.config.general[key] = val
end

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
-- ============================================================================

require("xpm").setup({
	plugins = { -- https://xplr.dev/en/awesome-plugins
		"dtomvan/xpm.xplr",
		-- ====================================================================
		-- 🛰️ Exentions: ⮯ {{{
		"sayanarijit/command-mode.xplr",
		-- }}} ⮭

		-- 📎 Integrations: ⮯ {{{
		"sayanarijit/fzf.xplr",
		"sayanarijit/zoxide.xplr",
		"Junker/nuke.xplr",
		-- }}} ⮭

		-- 👀 Theme ⮯ {{{
		"prncss-xyz/icons.xplr",
		{
			"dtomvan/extra-icons.xplr",
			after = function()
				xplr.config.general.table.row.cols[2] = {
					format = "custom.icons_dtomvan_col_1",
				}
			end,
		},
		-- }}} ⮭
		-- ====================================================================
	},
	auto_install = true,
	auto_cleanup = true,
})

-- ============================================================================
-- ⚙️  Nonstandard Configuration: 🢢 {{{

require("zoxide").setup({
	bin = "zoxide",
	mode = "default",
	key = "z",
})

require("fzf").setup({
	mode = "default",
	key = "t",
	bin = "fzf",
	args = "--preview 'pistol {}'",
	recursive = true, -- If true, search all files under $PWD
	enter_dir = true, -- Enter if the result is directory
})

require("nuke").setup({
	pager = "$PAGER", -- default: less -R
	open = {
		run_executables = false,
		custom = {
			{ extension = "jpg", command = "sxiv {}" },
			{ extension = "png", command = "sxiv {}" },
			{ extension = "gif", command = "gifview -a {}" },
			{ extension = "md", command = "nvim {}" },
			{ extension = "json", command = "nvim {}" },
			{ extension = "svg", command = "nvim {}" },
			{ extension = "", command = "nvim {}" },
			{ extension = "gz", command = "tar tf {} | $PAGER" },
			{ mime = "video/mp4", command = "mpv {}" },
			{ mime_regex = "^video/.*", command = "mpv {}" },
			{ mime_regex = ".*", command = "xdg-open {}" },
		},
	},
	view = {
		show_line_numbers = false, -- default: false
	},
})

local nukekey = xplr.config.modes.builtin.default.key_bindings.on_key

nukekey.v = {
	help = "nuke",
	messages = { "PopMode", { SwitchModeCustom = "nuke" } },
}
nukekey["enter"] = xplr.config.modes.custom.nuke.key_bindings.on_key.o
nukekey["right"] = xplr.config.modes.custom.nuke.key_bindings.on_key.o
nukekey["o"] = xplr.config.modes.custom.nuke.key_bindings.on_key.v

-- }}}
-- ============================================================================
