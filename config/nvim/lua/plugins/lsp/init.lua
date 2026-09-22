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

		-- TypeScript 7 dropped tsserver.js, so ts_ls would silently fall back to its bundled
		-- TypeScript 6. Stand down there and let tsc (the native --lsp server) take the project.
		local lock_markers = { "yarn.lock", "package-lock.json", "pnpm-lock.yaml", "bun.lock", "bun.lockb" }
		local ts_ls_root = vim.lsp.config.ts_ls.root_dir
		vim.lsp.config("ts_ls", {
			root_dir = function(bufnr, on_dir)
				local name = vim.api.nvim_buf_get_name(bufnr)
				local root = name ~= "" and vim.fs.root(name, lock_markers)
				if root then
					local ts = root .. "/node_modules/typescript"
					if vim.uv.fs_stat(ts) and not vim.uv.fs_stat(ts .. "/lib/tsserver.js") then
						return
					end
				end
				return ts_ls_root(bufnr, on_dir)
			end,
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
