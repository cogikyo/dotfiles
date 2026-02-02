-- LSP keymaps (LspAttach autocmd)
local M = {}

local function diag_jump(count)
	return function()
		vim.diagnostic.jump({ count = count })
	end
end

function M.setup()
	vim.api.nvim_create_autocmd("LspAttach", {
		group = vim.api.nvim_create_augroup("lsp-attach", { clear = true }),
		callback = function(event)
			local map = function(keys, func, d, mode)
				mode = mode or "n"
				vim.keymap.set(mode, keys, func, { buffer = event.buf, desc = "LSP: " .. d })
			end

			local ts = require("telescope.builtin")

			map("gd", vim.lsp.buf.definition, "Definition")
			map("gD", vim.lsp.buf.declaration, "Declaration")
			map("gi", vim.lsp.buf.implementation, "Implementation")
			map("<F12>", ts.lsp_references, "References")
			map("gt", ts.lsp_type_definitions, "Type Definition")
			map("gO", ts.lsp_document_symbols, "Document Symbols")
			map("gW", ts.lsp_dynamic_workspace_symbols, "Workspace Symbols")

			map("K", vim.lsp.buf.hover, "Hover")
			map("<C-k>", vim.lsp.buf.signature_help, "Signature Help")
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

			map("<leader>ht", function()
				vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled({ bufnr = event.buf }))
			end, "Toggle Inlay Hints")
			map("<leader>gg", "<cmd>LspRestart<CR>", "Restart LSP")

			local client = vim.lsp.get_client_by_id(event.data.client_id)
			if not client then
				return
			end

			-- TypeScript-specific actions
			if client.name == "ts_ls" then
				local tsmap = function(keys, func, d)
					vim.keymap.set("n", keys, func, { buffer = event.buf, desc = "TS: " .. d })
				end
				local action = function(name)
					return function()
						vim.lsp.buf.code_action({ apply = true, context = { only = { name }, diagnostics = {} } })
					end
				end

				tsmap("<leader>oi", action("source.organizeImports.ts"), "Organize Imports")
				tsmap("<leader>ru", action("source.removeUnused.ts"), "Remove Unused")
				tsmap("<leader>am", action("source.addMissingImports.ts"), "Add Missing Imports")
				tsmap("<leader>fa", action("source.fixAll.ts"), "Fix All")
			end

			-- Auto-enable inlay hints for Go/TS
			if client.name == "gopls" or client.name == "ts_ls" then
				vim.lsp.inlay_hint.enable(true, { bufnr = event.buf })
			end

			-- Codelens refresh
			if client:supports_method(vim.lsp.protocol.Methods.textDocument_codeLens, event.buf) then
				vim.api.nvim_create_autocmd({ "BufEnter", "InsertLeave" }, {
					buffer = event.buf,
					callback = vim.lsp.codelens.refresh,
				})
			end
		end,
	})
end

return M
