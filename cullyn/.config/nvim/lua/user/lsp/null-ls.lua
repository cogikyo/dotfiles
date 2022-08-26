local null_ls_status_ok, null_ls = pcall(require, "null-ls")
if not null_ls_status_ok then
	return
end

local code_actions = null_ls.builtins.code_actions
local diagnostics = null_ls.builtins.diagnostics
local formatting = null_ls.builtins.formatting
local hover = null_ls.builtins.hover

null_ls.setup({
	debug = false,
	sources = {
		formatting.prettier.with({
			extra_args = {
				"--single-quote",
				"--jsx-single-quote",
				"--print-width 80",
			},
		}),
		formatting.black.with({
			extra_args = { "--fast" },
		}),
		formatting.stylua,
		formatting.shellharden,

		diagnostics.flake8,
		diagnostics.shellcheck,
		diagnostics.eslint,
		diagnostics.jsonlint,
		diagnostics.luacheck.with({
			extra_args = { "--globals" },
		}),

		code_actions.shellcheck,
		-- code_actions.refactoring,
		code_actions.eslint,

		hover.dictionary,
	},
	on_attach = function(client)
		if client.resolved_capabilities.document_formatting then
			vim.cmd([[
            augroup LspFormatting
            autocmd! * <buffer>
            autocmd BufWritePre <buffer> lua vim.lsp.buf.formatting_sync()
            augroup END
            ]])
		end
	end,
})
