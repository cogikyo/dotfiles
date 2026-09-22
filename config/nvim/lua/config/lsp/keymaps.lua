local M = {}

local function restart_lsp()
	local clients = vim.lsp.get_clients({ bufnr = 0 })
	if vim.tbl_isempty(clients) then
		vim.notify("No active LSP clients for this buffer", vim.log.levels.WARN)
		return
	end

	local names = {}
	for _, client in ipairs(clients) do
		names[client.name] = true
	end

	for name in pairs(names) do
		vim.lsp.enable(name, false)
	end

	vim.defer_fn(function()
		for name in pairs(names) do
			vim.lsp.enable(name, true)
		end
	end, 300)
end

local function diag_jump(count)
	return function()
		vim.diagnostic.jump({ count = count })
	end
end

-- Code action kinds are server-specific: ts_ls suffixes them with .ts, and the native tsc
-- server has no addMissingImports at all.
local ts_actions = {
	ts_ls = {
		{ "<leader>oi", "source.organizeImports.ts", "Organize Imports" },
		{ "<leader>ru", "source.removeUnused.ts", "Remove Unused" },
		{ "<leader>am", "source.addMissingImports.ts", "Add Missing Imports" },
		{ "<leader>fa", "source.fixAll.ts", "Fix All" },
	},
	tsc = {
		{ "<leader>oi", "source.organizeImports", "Organize Imports" },
		{ "<leader>ru", "source.removeUnusedImports", "Remove Unused Imports" },
		{ "<leader>si", "source.sortImports", "Sort Imports" },
		{ "<leader>fa", "source.fixAll", "Fix All" },
	},
}

local function disable_semantic_tokens(client, bufnr)
	client.server_capabilities.semanticTokensProvider = nil

	if vim.lsp.semantic_tokens then
		vim.lsp.semantic_tokens.enable(false, { bufnr = bufnr })
	end
end

function M.setup()
	vim.api.nvim_create_user_command("LspRestart", restart_lsp, { desc = "Restart LSP clients for current buffer" })

	vim.api.nvim_create_autocmd("LspAttach", {
		group = vim.api.nvim_create_augroup("lsp-attach", { clear = true }),
		callback = function(event)
			local client = vim.lsp.get_client_by_id(event.data.client_id)
			if not client then
				return
			end

			disable_semantic_tokens(client, event.buf)

			local map = function(keys, func, d, mode)
				mode = mode or "n"
				vim.keymap.set(mode, keys, func, { buffer = event.buf, desc = "LSP: " .. d })
			end

			local ts = require("telescope.builtin")

			map("gd", ts.lsp_definitions, "Definition")
			map("gD", vim.lsp.buf.declaration, "Declaration")
			map("gi", vim.lsp.buf.implementation, "Implementation")
			map("<F12>", ts.lsp_references, "References")
			map("gt", ts.lsp_type_definitions, "Type Definition")
			map("gO", ts.lsp_document_symbols, "Document Symbols")
			map("gW", ts.lsp_dynamic_workspace_symbols, "Workspace Symbols")

			map("<C-k>", function()
				vim.lsp.buf.hover({ border = "single" })
			end, "Hover")
			map("K", function()
				vim.lsp.buf.signature_help({ border = "single" })
			end, "Signature Help")
			map("<leader>k", vim.diagnostic.open_float, "Diagnostic Float")

			vim.keymap.set("n", "<F2>", function()
				return ":IncRename " .. vim.fn.expand("<cword>")
			end, { buffer = event.buf, desc = "LSP: Rename", expr = true })
			map("<leader>ca", vim.lsp.buf.code_action, "Code Action")
			map("<leader>cl", vim.lsp.codelens.run, "Code Lens")

			map("<leader>ci", ts.lsp_incoming_calls, "Incoming Calls")
			map("<leader>co", ts.lsp_outgoing_calls, "Outgoing Calls")

			map("[d", diag_jump(-1), "Previous Diagnostic")
			map("]d", diag_jump(1), "Next Diagnostic")

			map("<leader>gq", "<cmd>LspRestart<CR>", "Restart LSP")
			map("<leader>ht", function()
				vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled({ bufnr = event.buf }))
			end, "Toggle Inlay Hints")
			map("<leader>tl", function()
				local enabled = vim.lsp.codelens.is_enabled({ bufnr = event.buf })
				vim.lsp.codelens.enable(not enabled, { bufnr = event.buf })
			end, "Toggle Code Lens")

			local actions = ts_actions[client.name]
			if actions then
				for _, entry in ipairs(actions) do
					local keys, kind, d = entry[1], entry[2], entry[3]
					vim.keymap.set("n", keys, function()
						vim.lsp.buf.code_action({ apply = true, context = { only = { kind }, diagnostics = {} } })
					end, { buffer = event.buf, desc = "TS: " .. d })
				end

				vim.lsp.inlay_hint.enable(false, { bufnr = event.buf })
			end

			if client.name ~= "gopls"
				and client:supports_method(vim.lsp.protocol.Methods.textDocument_codeLens, event.buf)
			then
				vim.lsp.codelens.enable(false, { bufnr = event.buf })
			end
		end,
	})
end

return M
