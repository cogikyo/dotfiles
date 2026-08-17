local oxc_root_markers = { ".oxlintrc.json", ".oxlintrc.jsonc", "oxlint.json", "oxlint.jsonc" }
local ts_source_kinds = {
	"source.addMissingImports.ts",
	"source.removeUnused.ts",
	"source.organizeImports.ts",
	"source.fixAll.ts",
}
local ts_source_budget_ms = 800
local fast_oxlint_cache = {}

local function oxc_root(bufnr)
	local filename = vim.api.nvim_buf_get_name(bufnr)
	return filename ~= "" and vim.fs.root(filename, oxc_root_markers) or nil
end

local function oxc_cwd(_, ctx) return vim.fs.root(ctx.dirname, oxc_root_markers) end

local function is_js_filetype(filetype)
	return filetype == "javascript"
		or filetype == "javascriptreact"
		or filetype == "typescript"
		or filetype == "typescriptreact"
end

-- oxlint has --type-aware / --type-check, but no CLI off-switch once the project config enables them.
local function oxlint_save_config(root)
	local src
	for _, name in ipairs(oxc_root_markers) do
		local path = root .. "/" .. name
		if vim.uv.fs_stat(path) then
			src = path
			break
		end
	end
	if not src then
		return nil
	end

	local stat = vim.uv.fs_stat(src)
	local mtime = stat and stat.mtime.sec or 0
	local hit = fast_oxlint_cache[src]
	if hit and hit.mtime == mtime and vim.uv.fs_stat(hit.dest) then
		return hit.dest
	end

	local ok, lines = pcall(vim.fn.readfile, src)
	if not ok then
		return nil
	end

	local text = table.concat(lines, "\n")
	if not text:find('"typeAware"%s*:%s*true') and not text:find('"typeCheck"%s*:%s*true') then
		return nil
	end

	text = text:gsub('"typeAware"%s*:%s*true', '"typeAware": false')
	text = text:gsub('"typeCheck"%s*:%s*true', '"typeCheck": false')

	local dest = vim.fn.stdpath("cache") .. "/oxlint-save/" .. vim.fn.sha256(src) .. ".json"
	vim.fn.mkdir(vim.fn.fnamemodify(dest, ":h"), "p")
	if vim.fn.writefile(vim.split(text, "\n", { plain = true }), dest) ~= 0 then
		return nil
	end

	fast_oxlint_cache[src] = { mtime = mtime, dest = dest }
	return dest
end

local function apply_workspace_edit(client, bufnr, action, timeout_ms)
	if action.disabled then
		return
	end

	local resolved = action
	if not resolved.edit and not resolved.command then
		local supports = client.supports_method and client:supports_method("codeAction/resolve", { bufnr = bufnr })
		if supports then
			local reply = client:request_sync("codeAction/resolve", resolved, timeout_ms, bufnr)
			if reply and reply.result then
				resolved = reply.result
			end
		end
	end

	if resolved.edit then
		vim.lsp.util.apply_workspace_edit(resolved.edit, client.offset_encoding)
	end
	if resolved.command then
		client:request_sync("workspace/executeCommand", resolved.command, timeout_ms, bufnr)
	end
end

-- Run before oxfmt so import sort stays with the formatter.
local function apply_ts_sources(bufnr)
	local client = vim.lsp.get_clients({ bufnr = bufnr, name = "ts_ls" })[1]
	if not client then
		return
	end

	local start = vim.uv.hrtime()
	for _, kind in ipairs(ts_source_kinds) do
		local remaining = ts_source_budget_ms - (vim.uv.hrtime() - start) / 1e6
		if remaining < 50 then
			return
		end

		local reply = client:request_sync("textDocument/codeAction", {
			textDocument = vim.lsp.util.make_text_document_params(bufnr),
			range = {
				start = { line = 0, character = 0 },
				["end"] = { line = vim.api.nvim_buf_line_count(bufnr), character = 0 },
			},
			context = { only = { kind }, diagnostics = {} },
		}, remaining, bufnr)
		if reply and reply.result then
			for _, action in ipairs(reply.result) do
				apply_workspace_edit(client, bufnr, action, remaining)
			end
		end
	end
end

local function format_on_save(bufnr)
	-- JS/TS opts into save formatting through oxlint config; do not fall back to Prettier.
	if is_js_filetype(vim.bo[bufnr].filetype) then
		if not oxc_root(bufnr) then
			return nil
		end
		apply_ts_sources(bufnr)
	end
	return { timeout_ms = 2000 }
end

local function js_formatters(bufnr)
	if oxc_root(bufnr) then
		return { "oxfmt", "oxlint" }
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
				args = function(_, ctx)
					local root = vim.fs.root(ctx.dirname, oxc_root_markers)
					local cfg = root and oxlint_save_config(root)
					if cfg then
						return { "--fix", "-c", cfg, "$FILENAME" }
					end
					return { "--fix", "$FILENAME" }
				end,
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
