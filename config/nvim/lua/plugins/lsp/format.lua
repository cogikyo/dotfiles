local oxc_root_markers = { ".oxlintrc.json", ".oxlintrc.jsonc", "oxlint.json", "oxlint.jsonc" }

local function oxc_root(bufnr)
	local filename = vim.api.nvim_buf_get_name(bufnr)
	return filename ~= "" and vim.fs.root(filename, oxc_root_markers) or nil
end

local function oxc_cwd(_, ctx)
	return vim.fs.root(ctx.dirname, oxc_root_markers)
end

local function js_formatters(bufnr)
	if oxc_root(bufnr) then
		return { "oxfmt" }
	end
	return { "prettierd" }
end

local function web_formatters(bufnr)
	if oxc_root(bufnr) then
		return { "oxfmt" }
	end
	return { "prettierd" }
end

return {
	"stevearc/conform.nvim",
	event = { "BufWritePre" },
	cmd = { "ConformInfo" },
	opts = {
		formatters = {
			oxfmt = {
				cwd = oxc_cwd,
				require_cwd = true,
			},
			oxlint = {
				cwd = oxc_cwd,
				require_cwd = true,
			},
			stylua = {
				prepend_args = { "--column-width", "120", "--collapse-simple-statement", "FunctionOnly" },
			},
		},
		formatters_by_ft = {
			bash = { "shellharden" },
			sh = { "shellharden" },
			zsh = { "beautysh" },
			python = { "ruff_format" },
			css = web_formatters,
			json = web_formatters,
			jsonc = web_formatters,
			lua = { "stylua" },
			toml = { "taplo" },
			javascript = js_formatters,
			javascriptreact = js_formatters,
			typescript = js_formatters,
			typescriptreact = js_formatters,
			html = web_formatters,
			markdown = web_formatters,
			yaml = {},
			go = { "goimports", "gofumpt" },
			templ = { "templ" },
		},
		format_on_save = { timeout_ms = 500 },
	},
}
