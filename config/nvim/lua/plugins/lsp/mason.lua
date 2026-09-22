local servers = require("config.lsp.servers")

-- tsc ships with the project's TypeScript 7 install; mason has no package for it.
local project_provided = { tsc = true }

local mason_servers = vim.tbl_filter(function(name)
	return not project_provided[name]
end, servers)

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
			ensure_installed = mason_servers,
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
