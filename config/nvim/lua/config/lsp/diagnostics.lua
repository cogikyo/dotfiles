-- Diagnostic configuration
local M = {}

function M.setup()
	vim.diagnostic.config({
		severity_sort = true,
		float = { border = "rounded", source = true },
		underline = true,
		signs = {
			text = {
				[vim.diagnostic.severity.ERROR] = "󰅚 ",
				[vim.diagnostic.severity.WARN] = "󰀪 ",
				[vim.diagnostic.severity.INFO] = "󰋽 ",
				[vim.diagnostic.severity.HINT] = "󰌶 ",
			},
		},
		virtual_text = { source = false, spacing = 2 },
	})

	vim.lsp.handlers["textDocument/hover"] = vim.lsp.with(vim.lsp.handlers.hover, { border = "rounded" })
end

return M
