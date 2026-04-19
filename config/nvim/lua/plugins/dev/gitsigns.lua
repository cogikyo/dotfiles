return {
	"lewis6991/gitsigns.nvim",
	event = { "BufReadPre", "BufNewFile" },
	config = function()
		local ok, gitsigns = pcall(require, "gitsigns")
		if not ok then
			return
		end

		local api = vim.api
		local fn = vim.fn

		-- ╭─────────────────────────────────────────────────────────────────────╮
		-- │ navigation: hunks within and across changed files                   │
		-- ╰─────────────────────────────────────────────────────────────────────╯

		local function git_root()
			local root = fn.systemlist("git rev-parse --show-toplevel")[1]
			if vim.v.shell_error ~= 0 then
				return nil
			end
			return root
		end

		local function git_lines(root, args)
			return fn.systemlist("git -C " .. fn.shellescape(root) .. " " .. args)
		end

		local function hunk_edge(direction)
			return direction == "next" and "first" or "last"
		end

		local function diff_hunk_key(direction)
			return direction == "next" and "]h" or "[h"
		end

		local function nav_hunk(direction)
			if vim.wo.diff then
				vim.cmd.normal({ diff_hunk_key(direction), bang = true })
				return
			end

			gitsigns.nav_hunk(direction)
		end

		local function get_changed_files()
			local root = git_root()
			if not root then
				return {}, {}
			end

			local modified = git_lines(root, "diff --name-only")
			local untracked = git_lines(root, "ls-files --others --exclude-standard")
			local untracked_set = {}
			local files = {}
			local seen = {}

			for _, file in ipairs(untracked) do
				untracked_set[root .. "/" .. file] = true
			end

			for _, list in ipairs({ modified, untracked }) do
				for _, file in ipairs(list) do
					local full_path = root .. "/" .. file
					if not seen[full_path] then
						seen[full_path] = true
						table.insert(files, full_path)
					end
				end
			end

			table.sort(files)
			return files, untracked_set
		end

		local function jump_to_hunk_on_attach(is_untracked, direction)
			if is_untracked then
				vim.schedule(function()
					api.nvim_win_set_cursor(0, { 1, 0 })
				end)
				return
			end

			local augroup = api.nvim_create_augroup("GitsignsNavJump", { clear = true })
			local jumped = false

			local function jump()
				gitsigns.nav_hunk(hunk_edge(direction))
			end

			api.nvim_create_autocmd("User", {
				group = augroup,
				pattern = "GitSignsUpdate",
				once = true,
				callback = function()
					jumped = true
					jump()
				end,
			})

			vim.defer_fn(function()
				if jumped then
					return
				end

				pcall(api.nvim_del_augroup_by_id, augroup)
				jump()
			end, 500)
		end

		local function switch_to_changed_file(direction)
			local files, untracked_set = get_changed_files()
			if #files == 0 then
				return
			end

			local current_file = api.nvim_buf_get_name(0)
			local current_index

			for index, file in ipairs(files) do
				if file == current_file then
					current_index = index
					break
				end
			end

			local next_index
			if not current_index then
				next_index = 1
			elseif direction == "next" then
				next_index = current_index % #files + 1
			else
				next_index = (current_index - 2) % #files + 1
			end

			local target = files[next_index]
			jump_to_hunk_on_attach(untracked_set[target] or false, direction)
			vim.cmd("edit " .. fn.fnameescape(target))
		end

		local function nav_hunk_all_files(direction)
			local start_win = api.nvim_get_current_win()
			local start_buf = api.nvim_get_current_buf()
			local start_line = api.nvim_win_get_cursor(start_win)[1]

			gitsigns.nav_hunk(direction, { wrap = false, navigation_message = false }, function(err)
				if err then
					return
				end

				vim.schedule(function()
					if not api.nvim_win_is_valid(start_win) then
						return
					end
					if api.nvim_get_current_win() ~= start_win then
						return
					end
					if api.nvim_get_current_buf() ~= start_buf then
						return
					end
					if api.nvim_win_get_cursor(start_win)[1] ~= start_line then
						return
					end

					switch_to_changed_file(direction)
				end)
			end)
		end

		local function selected_lines()
			return { fn.line("."), fn.line("v") }
		end

		-- ╭─────────────────────────────────────────────────────────────────────╮
		-- │ keymaps: hunk operations, blame, diff                               │
		-- ╰─────────────────────────────────────────────────────────────────────╯

		local function on_attach(bufnr)
			local function map(mode, lhs, rhs, desc)
				vim.keymap.set(mode, lhs, rhs, { buffer = bufnr, desc = desc })
			end

			local function next_hunk()
				nav_hunk("next")
			end

			local function prev_hunk()
				nav_hunk("prev")
			end

			local function next_hunk_all_files()
				nav_hunk_all_files("next")
			end

			local function prev_hunk_all_files()
				nav_hunk_all_files("prev")
			end

			local function stage_selection()
				gitsigns.stage_hunk(selected_lines())
			end

			local function reset_selection()
				gitsigns.reset_hunk(selected_lines())
			end

			local function blame_line_full()
				gitsigns.blame_line({ full = true })
			end

			local function diff_head_previous()
				gitsigns.diffthis("~")
			end

			map("n", "]h", next_hunk, "Next hunk (file)")
			map("n", "[h", prev_hunk, "Previous hunk (file)")
			map("n", "<A-n>", next_hunk_all_files, "Next hunk (all files)")
			map("n", "<A-N>", prev_hunk_all_files, "Previous hunk (all files)")
			map("n", "<leader>hs", gitsigns.stage_hunk, "Stage hunk")
			map("n", "<leader>hr", gitsigns.reset_hunk, "Reset hunk")
			map("v", "<leader>hs", stage_selection, "Stage hunk")
			map("v", "<leader>hr", reset_selection, "Reset hunk")
			map("n", "<leader>hS", gitsigns.stage_buffer, "Stage buffer")
			map("n", "<leader>hu", gitsigns.undo_stage_hunk, "Undo stage hunk")
			map("n", "<leader>hR", gitsigns.reset_buffer, "Reset buffer")
			map("n", "<leader>hp", gitsigns.preview_hunk, "Preview hunk")
			map("n", "<leader>hb", blame_line_full, "Blame line")
			map("n", "<leader>hB", gitsigns.toggle_current_line_blame, "Toggle line blame")
			map("n", "<leader>hw", gitsigns.toggle_word_diff, "Toggle word diff")
			map("n", "<leader>hl", gitsigns.toggle_linehl, "Toggle line highlight")
			map("n", "<leader>hd", gitsigns.toggle_deleted, "Toggle deleted")
			map("n", "<leader>hD", gitsigns.diffthis, "Diff this")
			map("n", "<leader>hH", diff_head_previous, "Diff against HEAD~")
			map({ "o", "x" }, "ih", ":<C-U>Gitsigns select_hunk<CR>", "Select hunk")
		end

		-- ╭─────────────────────────────────────────────────────────────────────╮
		-- │ setup: sign characters, highlights                                  │
		-- ╰─────────────────────────────────────────────────────────────────────╯

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
