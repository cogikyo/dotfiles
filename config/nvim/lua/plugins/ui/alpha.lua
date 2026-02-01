return {
	"goolord/alpha-nvim",
	dependencies = { "nvim-tree/nvim-web-devicons" },
	config = function()
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
				hl = "@lsp.typemod.function.defaultLibrary",
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

		require("alpha").setup({
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

		vim.api.nvim_create_autocmd("User", {
			pattern = "AlphaReady",
			callback = function(ev)
				vim.keymap.set("n", "<CR>", function()
					for _, file in ipairs(vim.v.oldfiles) do
						if vim.fn.filereadable(file) == 1 then
							vim.cmd("edit " .. vim.fn.fnameescape(file))
							return
						end
					end
				end, { buffer = ev.buf })
			end,
		})
	end,
}
