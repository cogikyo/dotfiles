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

end

return M
