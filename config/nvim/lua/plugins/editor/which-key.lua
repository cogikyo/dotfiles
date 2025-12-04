return {
	"folke/which-key.nvim",
	event = "VeryLazy",
	opts = function()
		local max_width = math.min(math.floor(vim.o.columns * 0.95), 200)
		return {
			delay = 350,
			icons = { mappings = false },
		win = {
			border = "rounded",
			width = max_width,
			col = 0.5,
			row = -2,
		},
			show_help = false,
			spec = require("config.keymaps").groups or {},
		}
	end,
}
