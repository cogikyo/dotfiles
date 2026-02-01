return {
	"iamcco/markdown-preview.nvim",
	ft = { "markdown" },
	keys = {
		{ "<leader>mt", "<cmd>MarkdownPreviewToggle<CR>", desc = "Markdown preview" },
	},
	build = function()
		vim.fn["mkdp#util#install"]()
	end,
	config = function()
		vim.g.mkdp_filetypes = { "markdown", "html" }
		vim.g.mkdp_auto_start = 0
		vim.g.mkdp_auto_close = 0
		vim.g.mkdp_refresh_slow = 1
	end,
}
