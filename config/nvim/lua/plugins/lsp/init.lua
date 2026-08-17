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

		local oxc_root_markers = { ".oxlintrc.json", ".oxlintrc.jsonc", "oxlint.json", "oxlint.jsonc" }
		local eslint_markers = {
			".eslintrc",
			".eslintrc.js",
			".eslintrc.cjs",
			".eslintrc.json",
			"eslint.config.js",
			"eslint.config.mjs",
			"eslint.config.cjs",
			"eslint.config.ts",
		}
		local eslint_root = vim.lsp.config.eslint.root_dir
		vim.lsp.config("eslint", {
			root_dir = function(bufnr, on_dir)
				local name = vim.api.nvim_buf_get_name(bufnr)
				if name ~= "" and vim.fs.root(name, oxc_root_markers) then
					return
				end
				if type(eslint_root) == "function" then
					return eslint_root(bufnr, on_dir)
				end
				local root = name ~= "" and vim.fs.root(name, eslint_markers)
				if root then
					on_dir(root)
				end
			end,
		})

		vim.lsp.enable(servers)
	end,
}
