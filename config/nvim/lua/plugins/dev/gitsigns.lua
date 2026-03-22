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

		local function get_changed_files()
			local git_root = vim.fn.systemlist("git rev-parse --show-toplevel")[1]
			if vim.v.shell_error ~= 0 then
				return {}, {}
			end
			local modified = vim.fn.systemlist("git -C " .. vim.fn.shellescape(git_root) .. " diff --name-only")
			local untracked =
				vim.fn.systemlist("git -C " .. vim.fn.shellescape(git_root) .. " ls-files --others --exclude-standard")
			local untracked_set = {}
			for _, f in ipairs(untracked) do
				untracked_set[git_root .. "/" .. f] = true
			end
			local files = {}
			local seen = {}
			for _, list in ipairs({ modified, untracked }) do
				for _, f in ipairs(list) do
					local full = git_root .. "/" .. f
					if not seen[full] then
						seen[full] = true
						table.insert(files, full)
					end
				end
			end
			table.sort(files)
			return files, untracked_set
		end

		local function jump_to_hunk_on_attach(is_untracked, direction)
			if is_untracked then
				-- Untracked files have no hunks; just go to line 1
				vim.schedule(function()
					vim.api.nvim_win_set_cursor(0, { 1, 0 })
				end)
				return
			end
			local gs = require("gitsigns")
			local augroup = vim.api.nvim_create_augroup("GitsignsNavJump", { clear = true })
			local jumped = false
			vim.api.nvim_create_autocmd("User", {
				group = augroup,
				pattern = "GitSignsUpdate",
				once = true,
				callback = function()
					jumped = true
					gs.nav_hunk(direction == "next" and "first" or "last")
				end,
			})
			vim.defer_fn(function()
				if not jumped then
					vim.api.nvim_del_augroup_by_id(augroup)
					gs.nav_hunk(direction == "next" and "first" or "last")
				end
			end, 500)
		end

		local function switch_to_next_changed_file(direction)
			local files, untracked_set = get_changed_files()
			if #files == 0 then
				return
			end

			local cur_file = vim.api.nvim_buf_get_name(0)
			local cur_idx
			for i, f in ipairs(files) do
				if f == cur_file then
					cur_idx = i
					break
				end
			end

			local next_idx
			if not cur_idx then
				next_idx = 1
			elseif direction == "next" then
				next_idx = cur_idx % #files + 1
			else
				next_idx = (cur_idx - 2) % #files + 1
			end

			local target = files[next_idx]
			jump_to_hunk_on_attach(untracked_set[target] or false, direction)
			vim.cmd("edit " .. vim.fn.fnameescape(target))
		end

		local function nav_hunk_all_files(direction)
			local gs = require("gitsigns")
			local start_buf = vim.api.nvim_get_current_buf()
			local start_line = vim.api.nvim_win_get_cursor(0)[1]

			-- Try navigating within current buffer without wrapping
			gs.nav_hunk(direction, { wrap = false, navigation_message = false })

			-- nav_hunk is async; check result after it completes
			vim.schedule(function()
				local new_buf = vim.api.nvim_get_current_buf()
				local new_line = vim.api.nvim_win_get_cursor(0)[1]
				if new_buf ~= start_buf or new_line ~= start_line then
					return -- moved to a different hunk in this file
				end
				-- No more hunks in this direction; switch files
				switch_to_next_changed_file(direction)
			end)
		end

		local function on_attach(bufnr)
			local gs = require("gitsigns")

			local function map(mode, l, r, opts)
				opts = opts or {}
				opts.buffer = bufnr
				vim.keymap.set(mode, l, r, opts)
			end

			map("n", "]h", nav_hunk("next"), { desc = "Next hunk (file)" })
			map("n", "[h", nav_hunk("prev"), { desc = "Previous hunk (file)" })
			map("n", "<A-n>", function()
				nav_hunk_all_files("next")
			end, { desc = "Next hunk (all files)" })
			map("n", "<A-N>", function()
				nav_hunk_all_files("prev")
			end, { desc = "Previous hunk (all files)" })
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
			map("n", "<leader>hc", function()
				gs.stage_hunk()
				vim.cmd("!hunk-commit")
			end, { desc = "Stage hunk + commit via Claude" })
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
