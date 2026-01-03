return {
	{
		"stevearc/conform.nvim",
		event = { "BufWritePre" },
		cmd = { "ConformInfo" },
		opts = {
			formatters_by_ft = {
				bash = { "shellharden" },
				sh = { "shellharden" },
				zsh = { "beautysh" },
				python = { "ruff_format" },
				css = { "prettierd" },
				json = { "prettierd" },
				jsonc = { "prettierd" },
				lua = { "stylua" },
				toml = { "taplo" },
				javascript = { "prettierd" },
				javascriptreact = { "prettierd" },
				typescript = { "prettierd" },
				typescriptreact = { "prettierd" },
				html = { "prettierd" },
				markdown = { "prettierd" },
				yaml = { "prettierd" },
				go = { "goimports", "gofumpt" },
				templ = { "templ" },
			},
			format_on_save = { timeout_ms = 500 },
		},
	},
	{
		"mfussenegger/nvim-lint",
		event = { "BufReadPre", "BufNewFile" },
		config = function()
			local lint = require("lint")
			lint.linters_by_ft = {
				zsh = { "zsh" },
				dockerfile = { "hadolint" },
			}
			vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost" }, {
				callback = function()
					lint.try_lint()
				end,
			})
		end,
	},
}
