return {
	"stevearc/conform.nvim",
	event = { "BufWritePre" },
	cmd = { "ConformInfo" },
	opts = {
		formatters = {
			stylua = {
				prepend_args = { "--column-width", "120", "--collapse-simple-statement", "FunctionOnly" },
			},
		},
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
			yaml = {},
			go = { "goimports", "gofumpt" },
			templ = { "templ" },
		},
		format_on_save = { timeout_ms = 500 },
	},
}
