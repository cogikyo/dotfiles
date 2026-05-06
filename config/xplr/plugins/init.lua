local home = os.getenv("HOME")
local xpm_path = home .. "/.local/share/xplr/dtomvan/xpm.xplr"
local xpm_url = "https://github.com/dtomvan/xpm.xplr"

package.path = package.path .. ";" .. xpm_path .. "/?.lua;" .. xpm_path .. "/?/init.lua"

os.execute(string.format("[ -e '%s' ] || git clone '%s' '%s'", xpm_path, xpm_url, xpm_path))

local function plugin(name)
	return { name, setup = function() end }
end

require("xpm").setup({
	plugins = {
		plugin("dtomvan/xpm.xplr"),
		plugin("sayanarijit/wl-clipboard.xplr"),
		plugin("sayanarijit/fzf.xplr"),
		plugin("sayanarijit/zoxide.xplr"),
		plugin("sayanarijit/trash-cli.xplr"),
		plugin("Junker/nuke.xplr"),
	},
	auto_install = true,
	auto_cleanup = false,
})
