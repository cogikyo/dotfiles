local status_ok, alpha = pcall(require, "alpha")
if not status_ok then
	return
end

local dashboard = require("alpha.themes.dashboard")
local theta = require("alpha.themes.theta")

dashboard.section.header.val = {
	[[                                                 ]],
	[[                                                 ]],
	[[                               __                ]],
	[[  ___     ___    ___   __  __ /\_\    ___ ___    ]],
	[[ / _ `\  / __`\ / __`\/\ \/\ \\/\ \  / __` __`\  ]],
	[[/\ \/\ \/\  __//\ \_\ \ \ \_/ |\ \ \/\ \/\ \/\ \ ]],
	[[\ \_\ \_\ \____\ \____/\ \___/  \ \_\ \_\ \_\ \_\]],
	[[ \/_/\/_/\/____/\/___/  \/__/    \/_/\/_/\/_/\/_/]],
	[[                                                 ]],
}

dashboard.section.buttons.val = {
	dashboard.button("t", "  Find file", ":Telescope find_files <CR>"),
	dashboard.button("r", "  Recently used files", ":Telescope oldfiles <CR>"),
	dashboard.button("T", "  Live grep", ":Telescope live_grep <CR>"),
	dashboard.button("c", "  Configuration", ":cd ~/dotfiles/config/nvim/<CR> :e lua/user/plugins.lua<CR>"),
	dashboard.button("q", "  Quit neovim", ":qa<CR>"),
}

dashboard.section.footer.val = vim.fn.getcwd()

dashboard.section.header.opts.hl = "Constant"

alpha.setup(dashboard.opts)
