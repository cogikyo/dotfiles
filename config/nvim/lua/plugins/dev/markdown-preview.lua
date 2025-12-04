return {
	"iamcco/markdown-preview.nvim",
	build = function()
		vim.fn["mkdp#util#install"]()
	end,
	config = function()
		vim.g.mkdp_filetypes = { "markdown", "html" }
		vim.g.mkdp_auto_start = 1
		vim.g.mkdp_auto_close = 0
		vim.g.mkdp_refresh_slow = 1
	end,
}
