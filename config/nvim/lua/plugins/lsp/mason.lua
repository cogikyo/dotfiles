-- Mason ecosystem: package management for LSP servers, formatters, linters
local servers = require("config.lsp.servers")

local tools = {
	-- Formatters
	"shellharden",
	"beautysh",
	"prettierd",
	"stylua",
	"goimports",
	"gofumpt",
	-- Linters
	"staticcheck",
	"hadolint",
}

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
			ensure_installed = servers,
			automatic_enable = false,
		},
	},
	{
		"WhoIsSethDaniel/mason-tool-installer.nvim",
		dependencies = { "williamboman/mason.nvim" },
		opts = {
			ensure_installed = tools,
			auto_update = true,
			run_on_start = true,
		},
	},
}
