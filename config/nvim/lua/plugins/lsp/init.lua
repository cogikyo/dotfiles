-- Core LSP configuration
local servers = require("config.lsp.servers")

return {
	"neovim/nvim-lspconfig",
	lazy = false,
	dependencies = {
		"williamboman/mason.nvim",
		"williamboman/mason-lspconfig.nvim",
	},
	config = function()
		require("config.lsp.diagnostics").setup()
		require("config.lsp.keymaps").setup()

		vim.lsp.config("*", {
			capabilities = require("blink.cmp").get_lsp_capabilities(),
		})

		vim.lsp.enable(servers)
	end,
}
