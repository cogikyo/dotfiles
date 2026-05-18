local function existing_dir(paths)
	for _, path in ipairs(paths) do
		local dir = vim.fn.expand(path)
		if vim.uv.fs_stat(dir) then
			return dir
		end
	end
end

return {
	"cogikyo/harpoon",
	dir = existing_dir({ "~/cogikyo/harpoon", "~/vagari/harpoon" }),
	keys = {
		{ "<leader>nn", function() require("harpoon.mark").add_file() end, desc = "Pin file" },
		{ "<leader>nl", function() require("harpoon.ui").toggle_quick_menu() end, desc = "Quick menu" },
		{ "<leader>nt", function() require("harpoon.ui").nav_file(1) end, desc = "File 1" },
		{ "<leader>ne", function() require("harpoon.ui").nav_file(2) end, desc = "File 2" },
		{ "<leader>ns", function() require("harpoon.ui").nav_file(3) end, desc = "File 3" },
		{ "<leader>na", function() require("harpoon.ui").nav_file(4) end, desc = "File 4" },
		{ "<leader>nf", function() require("harpoon.ui").nav_file(5) end, desc = "File 5" },
		{ "<leader>nd", function() require("harpoon.ui").nav_file(6) end, desc = "File 6" },
		{ "<leader>ng", function() require("harpoon.ui").nav_file(7) end, desc = "File 7" },
	},
	config = function()
		local harpoon = require("harpoon")

		harpoon.setup({
			pins_path = vim.fn.stdpath("config") .. "/lua/plugins/editor/harpoon.json",
			state_path = vim.fn.stdpath("state") .. "/harpoon.json",
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
