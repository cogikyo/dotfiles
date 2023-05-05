-- ============================================================================
-- 🧰 Setup {{{
local ok, alpha = pcall(require, "alpha")
if not ok then
	vim.api.nvim_echo({
		{
			"Error: alpha plugin not found... skipping relevant setup()",
			"Error",
		},
	}, true, {})
	return
end
-- }}}
-- ============================================================================

local dashboard = require("alpha.themes.dashboard")
local header = {
	type = "text",
	val = {
		[[                                __                 ]],
		[[   ___     ___    ___   __  __ /\_\    ___ ___     ]],
		[[  / _ `\  / __`\ / __`\/\ \/\ \\/\ \  / __` __`\   ]],
		[[ /\ \/\ \/\  __//\ \_\ \ \ \_/ |\ \ \/\ \/\ \/\ \  ]],
		[[ \ \_\ \_\ \____\ \____/\ \___/  \ \_\ \_\ \_\ \_\ ]],
		[[  \/_/\/_/\/____/\/___/  \/__/    \/_/\/_/\/_/\/_/ ]],
		[[                                                   ]],
		[[  _.--'"`'--._    _.--'"`'--._    _.--'"`'--._     ]],
		[[ :`.'|`|"':-.  '-:`.'|`|"':-.  '-:`.'|`|"':-.  '-  ]],
		[[ '.  | |  | |'.  '.  | |  | |'.  '.  | |  | |'.    ]],
		[[ . '.| |  | |  '.  '.| |  | |  '.  '.| |  | |  '.  ]],
		[[  `. `.:_ | :_.' '.  `.:_ | :_.' '.  `.:_ | :_.'   ]],
		[[    `-..,..-'       `-..,..-'       `-..,..-'      ]],
	},
	opts = {
		hl = "@function.builtin",
		shrink_margin = false,
		position = "center",
	},
}

local buttons = {
	type = "group",
	val = {
		{
			type = "text",
			val = "Quick Actions",
			opts = {
				hl = "Type",
				position = "center",
			},
		},
		{ type = "padding", val = 2 },
		dashboard.button("o", "  Recent Files", ":Telescope oldfiles <CR>"),
		dashboard.button("t", "󰈞  Find file", ":Telescope find_files <CR>"),
		dashboard.button("s", "󰭎  Live grep", ":Telescope live_grep <CR>"),
		{ type = "padding", val = 2 },
		dashboard.button("q", "󰗼  Quit", ":qa<CR>"),
	},
}

alpha.setup({
	layout = {
		{ type = "padding", val = 2 },
		header,
		{ type = "padding", val = 3 },
		buttons,
	},
	opts = {
		margin = 5,
	},
})
