return {
	"luckasRanarison/tailwind-tools.nvim",
	dependencies = { "nvim-treesitter/nvim-treesitter" },
	opts = {
		server = { override = false },
		document_color = { enabled = true, inline_symbol = "█" },
		conceal = { enabled = true, symbol = "…" },
	},
}
