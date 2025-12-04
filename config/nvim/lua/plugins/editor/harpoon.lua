return {
	"ThePrimeagen/harpoon",
	config = function()
		local harpoon = require("harpoon")

		harpoon.setup({
			menu = {
				borderchars = { "─", "│", "─", "│", "╭", "╮", "╯", "╰" },
			},
		})

		local function apply_highlights()
			vim.api.nvim_set_hl(0, "HarpoonBorder", { link = "FloatBorder" })
			vim.api.nvim_set_hl(0, "HarpoonWindow", { link = "NormalFloat" })
		end

		apply_highlights()

		vim.api.nvim_create_autocmd("ColorScheme", {
			group = vim.api.nvim_create_augroup("HarpoonHighlights", { clear = true }),
			callback = apply_highlights,
		})

		vim.api.nvim_create_autocmd("FileType", {
			pattern = "harpoon",
			group = vim.api.nvim_create_augroup("HarpoonColumns", { clear = true }),
			callback = function()
				vim.opt_local.statuscolumn = ""
				vim.opt_local.signcolumn = "no"
			end,
		})
	end,
}
