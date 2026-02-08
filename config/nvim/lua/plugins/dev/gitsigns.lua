return {
	"lewis6991/gitsigns.nvim",
	event = { "BufReadPre", "BufNewFile" },
	config = function()
		local ok, gitsigns = pcall(require, "gitsigns")
		if not ok then
			return
		end

		local function nav_hunk(direction)
			return function()
				local gs = require("gitsigns")
				if vim.wo.diff then
					vim.cmd.normal({ direction == "next" and "]h" or "[h", bang = true })
				else
					gs.nav_hunk(direction)
				end
			end
		end

		local function on_attach(bufnr)
			local gs = require("gitsigns")

			local function map(mode, l, r, opts)
				opts = opts or {}
				opts.buffer = bufnr
				vim.keymap.set(mode, l, r, opts)
			end

			map("n", "]h", nav_hunk("next"), { desc = "Next hunk" })
			map("n", "[h", nav_hunk("prev"), { desc = "Previous hunk" })
			map("n", "<leader>hs", gs.stage_hunk, { desc = "Stage hunk" })
			map("n", "<leader>hr", gs.reset_hunk, { desc = "Reset hunk" })
			map("v", "<leader>hs", function()
				gs.stage_hunk({ vim.fn.line("."), vim.fn.line("v") })
			end, { desc = "Stage hunk" })
			map("v", "<leader>hr", function()
				gs.reset_hunk({ vim.fn.line("."), vim.fn.line("v") })
			end, { desc = "Reset hunk" })
			map("n", "<leader>hS", gs.stage_buffer, { desc = "Stage buffer" })
			map("n", "<leader>hu", gs.undo_stage_hunk, { desc = "Undo stage hunk" })
			map("n", "<leader>hR", gs.reset_buffer, { desc = "Reset buffer" })
			map("n", "<leader>hp", gs.preview_hunk, { desc = "Preview hunk" })
			map("n", "<leader>hb", function()
				gs.blame_line({ full = true })
			end, { desc = "Blame line" })
			map("n", "<leader>hB", gs.toggle_current_line_blame, { desc = "Toggle line blame" })
			map("n", "<leader>hw", gs.toggle_word_diff, { desc = "Toggle word diff" })
			map("n", "<leader>hl", gs.toggle_linehl, { desc = "Toggle line highlight" })
			map("n", "<leader>hd", gs.toggle_deleted, { desc = "Toggle deleted" })
			map("n", "<leader>hD", gs.diffthis, { desc = "Diff this" })
			map("n", "<leader>hH", function()
				gs.diffthis("~")
			end, { desc = "Diff against HEAD~" })
			map({ "o", "x" }, "ih", ":<C-U>Gitsigns select_hunk<CR>", { desc = "Select hunk" })
		end

		gitsigns.setup({
			signs = {
				add = { text = "┃" },
				change = { text = "┃" },
				untracked = { text = "┋" },
				delete = { text = "╏" },
				topdelete = { text = "┏" },
				changedelete = { text = "╋" },
			},
			signs_staged = {
				add = { text = "│" },
				change = { text = "│" },
				delete = { text = "╎" },
				topdelete = { text = "┌" },
				changedelete = { text = "┼" },
			},

			signcolumn = true,
			numhl = true,
			linehl = false,
			word_diff = false,
			current_line_blame = true,

			on_attach = on_attach,
		})
	end,
}
