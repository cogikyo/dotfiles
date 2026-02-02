-- Core LSP configuration
local servers = require("config.lsp.servers")

return {
	"neovim/nvim-lspconfig",
	event = { "BufReadPre", "BufNewFile" },
	dependencies = {
		"williamboman/mason.nvim",
		"williamboman/mason-lspconfig.nvim",
	},
	config = function()
		require("config.lsp.diagnostics").setup()
		require("config.lsp.keymaps").setup()

		local capabilities = require("blink.cmp").get_lsp_capabilities()

		for name, cfg in pairs(servers) do
			cfg.capabilities = vim.tbl_deep_extend("force", {}, capabilities, cfg.capabilities or {})
			vim.lsp.config(name, cfg)
			vim.lsp.enable(name)
		end

		vim.lsp.enable("harper_ls", false)
	end,
}
