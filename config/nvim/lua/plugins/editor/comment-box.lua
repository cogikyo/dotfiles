return {
	"LudoPinelli/comment-box.nvim",
	keys = {
		{ "gcb", "<Cmd>CBccbox<CR>", mode = { "n", "v" }, desc = "Comment box" },
		{ "gcq", "<Cmd>CBllbox13<CR>", mode = { "n", "v" }, desc = "Comment quote" },
		{ "gcl", "<Cmd>CBccline10<CR>", mode = { "n", "v" }, desc = "Comment line" },
	},
	opts = {
		box_width = 80,
		line_width = 80,
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
