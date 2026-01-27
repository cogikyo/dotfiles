return {
	"LudoPinelli/comment-box.nvim",
	keys = {
		{ "<leader>cb", "<Cmd>CBccbox<CR>", mode = { "n", "v" }, desc = "Comment box" },
		{ "<leader>cB", "<Cmd>CBcatalog<CR>", desc = "Comment box catalog" },
		{ "<leader>c-", "<Cmd>CBline<CR>", desc = "Comment line" },
	},
	opts = {
		box_width = 80,
		doc_width = 80,
		borders = {
			top = "─",
			bottom = "─",
			left = "│",
			right = "│",
			top_left = "╭",
			top_right = "╮",
			bottom_left = "╰",
			bottom_right = "╯",
		},
	},
}
