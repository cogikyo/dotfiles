local M = {}

M.markers = { ".oxlintrc.json", ".oxlintrc.jsonc", "oxlint.json", "oxlint.jsonc" }

local pending = {}
local clean = {}
local kinds = {
	tsc = { "source.organizeImports", "source.fixAll" },
	ts_ls = {
		"source.addMissingImports.ts",
		"source.removeUnused.ts",
		"source.organizeImports.ts",
		"source.fixAll.ts",
	},
}

function M.root(bufnr)
	local name = vim.api.nvim_buf_get_name(bufnr)
	return name ~= "" and vim.fs.root(name, M.markers) or nil
end

local function stamp(name)
	local stat = vim.uv.fs_stat(name)
	return stat and { stat.ino, stat.size, stat.mtime, stat.ctime, stat.mode } or nil
end

local function cancel(bufnr, reason)
	clean[bufnr] = nil
	local state = pending[bufnr]
	if not state then
		return
	end
	pending[bufnr] = nil
	if not state.timer:is_closing() then
		state.timer:stop()
		state.timer:close()
	end
	if state.request then
		state.client:cancel_request(state.request)
	end
	if reason then
		vim.notify("Save cancelled: " .. reason, vim.log.levels.WARN)
	end
end

local function current(state)
	local buf = state.buf
	if pending[buf] ~= state then
		return false
	end
	if not vim.api.nvim_buf_is_loaded(buf) or vim.api.nvim_buf_get_name(buf) ~= state.name then
		cancel(buf)
		return false
	end
	if not vim.deep_equal(stamp(state.name), state.disk) then
		cancel(buf, "file changed on disk; keeping the external edit")
		vim.cmd.checktime(tostring(buf))
		return false
	end
	if vim.api.nvim_buf_get_changedtick(buf) ~= state.tick then
		cancel(buf, "buffer changed; press Ctrl-S again when ready")
		return false
	end
	return true
end

local function apply(buf, client, action)
	if action.command then
		error("this action needs a command; run it manually with a code-action keymap", 0)
	end
	local edit = action.edit
	if not edit then
		return
	end
	local uri = vim.uri_from_bufnr(buf)
	for target in pairs(edit.changes or {}) do
		if target ~= uri then
			error("action edits another file; run it manually with a code-action keymap", 0)
		end
	end
	for _, change in ipairs(edit.documentChanges or {}) do
		if change.kind or change.textDocument.uri ~= uri then
			error("action edits another file; run it manually with a code-action keymap", 0)
		end
		local version = change.textDocument.version
		if version and version ~= vim.NIL and version ~= vim.lsp.util.buf_versions[buf] then
			error("language server returned an outdated edit", 0)
		end
	end
	vim.lsp.util.apply_workspace_edit(edit, client.offset_encoding)
end

local function source_client(buf)
	for _, client in ipairs(vim.lsp.get_clients({ bufnr = buf })) do
		if kinds[client.name] then
			return client
		end
	end
end

local function sources(buf, request, changed)
	local client = source_client(buf)
	if not client then
		return
	end
	for _, kind in ipairs(kinds[client.name]) do
		local result = request(client, "textDocument/codeAction", {
			textDocument = vim.lsp.util.make_text_document_params(buf),
			range = { start = { line = 0, character = 0 }, ["end"] = { line = 0, character = 0 } },
			context = { only = { kind }, diagnostics = {} },
		})
		local selected
		for _, action in ipairs(result or {}) do
			if
				not action.disabled and (action.kind == kind or (action.kind or ""):sub(1, #kind + 1) == kind .. ".")
			then
				if selected then
					error("multiple source actions returned; choose one with a code-action keymap", 0)
				end
				selected = action
			end
		end
		if selected then
			if not selected.edit and not selected.command and client:supports_method("codeAction/resolve", buf) then
				selected = request(client, "codeAction/resolve", selected)
			end
			apply(buf, client, selected)
			changed()
		end
	end
end

function M.sources(buf)
	cancel(buf)
	local deadline = vim.uv.hrtime() + 800 * 1e6
	sources(buf, function(client, method, params)
		local name = vim.api.nvim_buf_get_name(buf)
		local disk = stamp(name)
		local tick = vim.api.nvim_buf_get_changedtick(buf)
		local remaining = math.floor((deadline - vim.uv.hrtime()) / 1e6)
		if remaining <= 0 then
			error("Save cancelled: source actions timed out", 0)
		end
		local reply, err = client:request_sync(method, params, remaining, buf)
		if not reply then
			error("Save cancelled: " .. tostring(err), 0)
		end
		if reply.err then
			error("Save cancelled: " .. reply.err.message, 0)
		end
		if not vim.deep_equal(disk, stamp(name)) then
			vim.schedule(function() vim.cmd.checktime(tostring(buf)) end)
			error("Save cancelled: file changed on disk during cleanup", 0)
		end
		if tick ~= vim.api.nvim_buf_get_changedtick(buf) then
			error("Save cancelled: buffer changed during cleanup", 0)
		end
		return reply.result
	end, function() end)
end

function M.committing(buf) return pending[buf] and pending[buf].committing end

function M.write()
	local buf = vim.api.nvim_get_current_buf()
	local name = vim.api.nvim_buf_get_name(buf)
	if
		name == ""
		or vim.bo[buf].buftype ~= ""
		or vim.bo[buf].readonly
		or not vim.uv.fs_stat(vim.fs.dirname(name))
		or not M.root(buf)
	then
		vim.cmd.write()
		return
	end
	if pending[buf] and current(pending[buf]) then
		return
	end
	vim.cmd.checktime(tostring(buf))
	local previous = clean[buf]
	if
		previous
		and previous.client == source_client(buf)
		and not vim.bo[buf].modified
		and previous.tick == vim.api.nvim_buf_get_changedtick(buf)
		and vim.deep_equal(previous.disk, stamp(name))
	then
		return
	end
	local state = {
		buf = buf,
		name = name,
		tick = vim.api.nvim_buf_get_changedtick(buf),
		disk = stamp(name),
	}
	pending[buf] = state
	state.timer = vim.defer_fn(function()
		if pending[buf] == state then
			cancel(buf, "cleanup timed out; buffer is still unsaved")
		end
	end, 5000)

	local thread
	local function resume(err, result)
		vim.schedule(function()
			if not current(state) then
				return
			end
			state.request = nil
			if err then
				cancel(buf, type(err) == "table" and err.message or tostring(err))
				return
			end
			local ok, failure = coroutine.resume(thread, result)
			if not ok then
				cancel(buf, tostring(failure))
			end
		end)
	end

	thread = coroutine.create(function()
		local conform = require("conform")
		sources(buf, function(client, method, params)
			state.client = client
			local ok, id = client:request(method, params, resume, buf)
			if not ok then
				error("language server is unavailable", 0)
			end
			state.request = id
			return coroutine.yield()
		end, function() state.tick = vim.api.nvim_buf_get_changedtick(buf) end)

		local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
		local names = conform.list_formatters_for_buffer(buf)
		for _, name in ipairs(names) do
			local formatter = conform.get_formatter_info(name, buf)
			if not formatter.available then
				error("formatter unavailable: " .. name, 0)
			end
		end
		-- Keep formatter output separate until the pending save passes its buffer and disk checks.
		conform.format_lines(names, lines, { bufnr = buf, async = true }, resume)
		local formatted = coroutine.yield()
		require("conform.runner").apply_format(buf, lines, formatted, nil, false, false, false)
		state.tick = vim.api.nvim_buf_get_changedtick(buf)
		state.committing = true
		vim.api.nvim_buf_call(buf, function() vim.cmd.write() end)
		cancel(buf)
		clean[buf] = { tick = vim.api.nvim_buf_get_changedtick(buf), disk = stamp(name), client = state.client }
	end)
	resume()
end

vim.api.nvim_create_autocmd({ "BufWritePre", "BufReadPost", "BufUnload", "BufWipeout" }, {
	group = vim.api.nvim_create_augroup("CancelPendingSave", { clear = true }),
	callback = function(args)
		if not M.committing(args.buf) then
			cancel(args.buf)
		end
	end,
})

return M
