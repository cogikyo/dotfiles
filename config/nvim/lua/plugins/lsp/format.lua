local oxc_root_markers = { ".oxlintrc.json", ".oxlintrc.jsonc", "oxlint.json", "oxlint.jsonc" }

local function oxc_root(bufnr)
	local filename = vim.api.nvim_buf_get_name(bufnr)
	return filename ~= "" and vim.fs.root(filename, oxc_root_markers) or nil
end

local function oxc_cwd(_, ctx)
	return vim.fs.root(ctx.dirname, oxc_root_markers)
end

local function is_js_filetype(filetype)
	return filetype == "javascript"
		or filetype == "javascriptreact"
		or filetype == "typescript"
		or filetype == "typescriptreact"
end

local function format_on_save(bufnr)
	-- JS/TS opts into save formatting through oxlint config; do not fall back to Prettier.
	if is_js_filetype(vim.bo[bufnr].filetype) and not oxc_root(bufnr) then
		return nil
	end
	return { timeout_ms = 2000 }
end

local function js_formatters(bufnr)
	if oxc_root(bufnr) then
		return { "oxlint", "oxfmt" }
	end
	return {}
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
				args = { "--fix", "$FILENAME" },
				cwd = oxc_cwd,
				exit_codes = { 0, 1 },
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
		format_on_save = format_on_save,
	},
}
