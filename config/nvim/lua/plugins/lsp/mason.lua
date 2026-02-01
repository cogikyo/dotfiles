-- Mason ecosystem: package management for LSP servers, formatters, linters
local server_config = require("config.servers")

return {
	{
		"williamboman/mason.nvim",
		cmd = "Mason",
		build = ":MasonUpdate",
		opts = {},
	},
	{
		"williamboman/mason-lspconfig.nvim",
		opts = {
			ensure_installed = vim.tbl_keys(server_config.servers),
		},
	},
	{
		"WhoIsSethDaniel/mason-tool-installer.nvim",
		dependencies = { "williamboman/mason.nvim" },
		opts = {
			ensure_installed = server_config.tools,
			auto_update = true,
			run_on_start = true,
		},
	},
}
