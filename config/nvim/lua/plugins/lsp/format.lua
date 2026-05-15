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

local function lsp_diagnostics(bufnr)
	return vim.tbl_map(function(diagnostic)
		return diagnostic.user_data and diagnostic.user_data.lsp
			or {
				code = diagnostic.code,
				message = diagnostic.message,
				range = {
					start = { line = diagnostic.lnum, character = diagnostic.col },
					["end"] = {
						line = diagnostic.end_lnum or diagnostic.lnum,
						character = diagnostic.end_col or diagnostic.col,
					},
				},
				severity = diagnostic.severity,
				source = diagnostic.source,
			}
	end, vim.diagnostic.get(bufnr))
end

local function apply_ts_source_action(bufnr, only, changedtick, done)
	local client = vim.lsp.get_clients({ bufnr = bufnr, name = "ts_ls" })[1]
	if not client or vim.api.nvim_buf_get_changedtick(bufnr) ~= changedtick then
		done()
		return
	end

	client:request("textDocument/codeAction", {
		textDocument = vim.lsp.util.make_text_document_params(bufnr),
		range = {
			start = { line = 0, character = 0 },
			["end"] = { line = vim.api.nvim_buf_line_count(bufnr), character = 0 },
		},
		context = { only = { only }, diagnostics = lsp_diagnostics(bufnr) },
	}, function(_, actions)
		if not vim.api.nvim_buf_is_valid(bufnr) or vim.api.nvim_buf_get_changedtick(bufnr) ~= changedtick then
			done()
			return
		end

		for _, action in ipairs(actions or {}) do
			if action.kind == only then
				if action.edit then
					vim.lsp.util.apply_workspace_edit(action.edit, client.offset_encoding)
				end
				if action.command then
					client:exec_cmd(action.command, { bufnr = bufnr })
				end
			end
		end

		done()
	end, bufnr)
end

local function js_autofix_after_save(bufnr)
	if vim.b[bufnr].js_autofix_after_save or vim.bo[bufnr].buftype ~= "" then
		return
	end
	if not is_js_filetype(vim.bo[bufnr].filetype) or not oxc_root(bufnr) then
		return
	end

	vim.b[bufnr].js_autofix_after_save = true
	local saved_tick = vim.api.nvim_buf_get_changedtick(bufnr)
	vim.defer_fn(function()
		if not vim.api.nvim_buf_is_valid(bufnr) or vim.api.nvim_buf_get_changedtick(bufnr) ~= saved_tick then
			vim.b[bufnr].js_autofix_after_save = false
			return
		end

		apply_ts_source_action(bufnr, "source.addMissingImports.ts", saved_tick, function()
			local imports_tick = vim.api.nvim_buf_get_changedtick(bufnr)
			apply_ts_source_action(bufnr, "source.removeUnusedImports.ts", imports_tick, function()
				require("conform").format({ bufnr = bufnr, async = true, timeout_ms = 2000 }, function(err)
					if not err and vim.api.nvim_buf_is_valid(bufnr) then
						vim.api.nvim_buf_call(bufnr, function()
							vim.cmd("silent noautocmd update")
						end)
					end
					if vim.api.nvim_buf_is_valid(bufnr) then
						vim.b[bufnr].js_autofix_after_save = false
					end
				end)
			end)
		end)
	end, 250)
end

local function format_on_save(bufnr)
	if is_js_filetype(vim.bo[bufnr].filetype) and oxc_root(bufnr) then
		return nil
	end
	return { timeout_ms = 2000 }
end

local function js_formatters(bufnr)
	if oxc_root(bufnr) then
		return { "oxlint", "oxfmt" }
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
	config = function(_, opts)
		require("conform").setup(opts)
		vim.api.nvim_create_autocmd("BufWritePost", {
			group = vim.api.nvim_create_augroup("js-autofix-after-save", { clear = true }),
			callback = function(args)
				js_autofix_after_save(args.buf)
			end,
		})
	end,
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
