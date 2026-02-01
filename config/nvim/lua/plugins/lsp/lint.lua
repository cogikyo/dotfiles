-- Linting via nvim-lint
return {
	"mfussenegger/nvim-lint",
	event = { "BufReadPre", "BufNewFile" },
	config = function()
		local lint = require("lint")
		lint.linters_by_ft = {
			go = { "staticcheck" },
			zsh = { "zsh" },
			dockerfile = { "hadolint" },
		}
		vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost" }, {
			group = vim.api.nvim_create_augroup("nvim-lint", { clear = true }),
			callback = function()
				if vim.bo.buftype == "" then
					lint.try_lint()
				end
			end,
		})
	end,
}
