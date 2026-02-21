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
		local bin_path = vim.fn.stdpath("data") .. "/lazy/markdown-preview.nvim/app/bin"
		if vim.fn.isdirectory(bin_path) == 0 then
			vim.fn["mkdp#util#install"]()
		end

		vim.g.mkdp_filetypes = { "markdown", "html" }
		vim.g.mkdp_auto_start = 0
		vim.g.mkdp_auto_close = 0
		vim.g.mkdp_refresh_slow = 1
	end,
}
