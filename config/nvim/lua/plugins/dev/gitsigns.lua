return {
	"lewis6991/gitsigns.nvim",
	event = { "BufReadPre", "BufNewFile" },
	config = function()
		local ok, gitsigns = pcall(require, "gitsigns")
		if not ok then
			return
		end

		gitsigns.setup({
			signs = {
				add = { text = "┣" },
				change = { text = "┃" },
				untracked = { text = "┊" },
				delete = { text = "│" },
				topdelete = { text = "┼" },
				changedelete = { text = "╋" },
			},

			signcolumn = true,
			numhl = true,
			linehl = false,
			word_diff = false,
			current_line_blame = true,

			on_attach = require("config.keymaps").gitsigns_on_attach,
		})
	end,
}
