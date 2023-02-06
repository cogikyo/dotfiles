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
        'sayanarijit/zoxide.xplr',
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

require("zoxide").setup{
  bin = "zoxide",
  mode = "default",
  key = "z",
}

-- }}}
-- ============================================================================
