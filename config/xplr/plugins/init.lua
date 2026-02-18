local home = os.getenv("HOME")
local xpm_path = home .. "/.local/share/xplr/dtomvan/xpm.xplr"
local xpm_url = "https://github.com/dtomvan/xpm.xplr"

package.path = package.path .. ";" .. xpm_path .. "/?.lua;" .. xpm_path .. "/?/init.lua"

os.execute(string.format("[ -e '%s' ] || git clone '%s' '%s'", xpm_path, xpm_url, xpm_path))

require("xpm").setup({
	plugins = {
		"dtomvan/xpm.xplr",
		"sayanarijit/wl-clipboard.xplr",
		"sayanarijit/fzf.xplr",
		"sayanarijit/zoxide.xplr",
		"sayanarijit/trash-cli.xplr",
		"Junker/nuke.xplr",
		"dtomvan/ouch.xplr",
		"emsquid/preview.xplr",
	},
	auto_install = true,
	auto_cleanup = true,
})

-- preview.xplr requires itself as "preview.lib.*" internally,
-- so it needs a "preview/" directory on the lua path.
-- Symlink the xpm-installed plugin into plugins/preview to fix this.
local preview_link = home .. "/.config/xplr/plugins/preview"
local preview_src = home .. "/.local/share/xplr/emsquid/preview.xplr"
os.execute(string.format("[ -L '%s' ] || ln -sfn '%s' '%s'", preview_link, preview_src, preview_link))
