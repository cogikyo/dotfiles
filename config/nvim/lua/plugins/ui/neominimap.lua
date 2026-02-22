return {
	"Isrothy/neominimap.nvim",
	version = "v3.*",
	lazy = false,
	keys = {
		{ "<leader>nm", "<cmd>Neominimap Toggle<cr>", desc = "Toggle minimap" },
	},
	init = function()
		vim.g.neominimap = {
			auto_enable = true,
			layout = "float",
			float = {
				minimap_width = 10,
				margin = { right = 0, top = 0, bottom = 0 },
			},
			x_multiplier = 4,
			y_multiplier = 1,
			delay = 200,
			click = {
				enabled = true,
				auto_switch_focus = false,
			},
			search = {
				enabled = true,
				mode = "line",
			},
			diagnostic = {
				enabled = true,
				mode = "line",
			},
			git = {
				enabled = true,
				mode = "sign",
			},
			treesitter = {
				enabled = true,
			},
		}
	end,
}
