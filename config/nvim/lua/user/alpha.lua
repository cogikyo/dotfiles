local alpha = require("alpha")
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
		hl = "@function.macro",
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
		dashboard.button("r", "  Recent Files", ":Telescope oldfiles <CR>"),
		dashboard.button("t", "  Find file", ":Telescope find_files <CR>"),
		dashboard.button("s", "  Live grep", ":Telescope live_grep <CR>"),
		{ type = "padding", val = 2 },
		dashboard.button("q", "  Quit", ":qa<CR>"),
	},
}

local opts = {
	layout = {
		{ type = "padding", val = 2 },
		header,
		{ type = "padding", val = 3 },
		buttons,
	},
	opts = {
		margin = 5,
	},
}

alpha.setup(opts)
