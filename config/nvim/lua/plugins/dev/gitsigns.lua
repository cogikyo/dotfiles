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
		local commit = require("config.gitsigns_commit")
		local git = commit.git
		local pending_jump = nil

		-- ╭─────────────────────────────────────────────────────────────────────╮
		-- │ navigation: hunks within and across changed files                   │
		-- ╰─────────────────────────────────────────────────────────────────────╯

		local function git_root(path)
			local dir = path ~= "" and fn.fnamemodify(path, ":p:h") or fn.getcwd()
			local out = git(dir, { "rev-parse", "--show-toplevel" })
			return out and vim.trim(out) or nil
		end

		local function git_lines(root, args)
			return vim.split(git(root, args) or "", "\n", { plain = true, trimempty = true })
		end

		local function hunk_edge(direction) return direction == "next" and "first" or "last" end

		local function diff_hunk_key(direction) return direction == "next" and "]h" or "[h" end
		local function show_hunk_linehl() gitsigns.toggle_linehl(true) end

		local function nav_hunk(direction)
			show_hunk_linehl()

			if vim.wo.diff then
				vim.cmd.normal({ diff_hunk_key(direction), bang = true })
				return
			end

			gitsigns.nav_hunk(direction)
		end

		local function get_changed_files(root)
			local modified = git_lines(root, { "diff", "--name-only" })
			local untracked = git_lines(root, { "ls-files", "--others", "--exclude-standard" })
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

		local function cancel_jump()
			if pending_jump then
				pending_jump()
			end
		end

		local function jump_to_hunk_on_attach(is_untracked, direction)
			cancel_jump()
			local win, buf = api.nvim_get_current_win(), api.nvim_get_current_buf()
			local autocmd, finish

			local function here()
				return pending_jump == finish
					and api.nvim_win_is_valid(win)
					and api.nvim_get_current_win() == win
					and api.nvim_win_get_buf(win) == buf
			end

			function finish()
				if pending_jump == finish then
					pending_jump = nil
				end
				if autocmd then
					pcall(api.nvim_del_autocmd, autocmd)
				end
			end
			pending_jump = finish

			if is_untracked then
				vim.schedule(function()
					if here() then
						api.nvim_win_set_cursor(win, { 1, 0 })
					end
					finish()
				end)
				return
			end

			local function jump()
				if here() then
					gitsigns.nav_hunk(hunk_edge(direction))
				end
				finish()
			end

			local function jump_if_ready()
				if not here() then
					finish()
				elseif next(gitsigns.get_hunks(buf) or {}) then
					jump()
				end
			end

			autocmd = api.nvim_create_autocmd("User", {
				pattern = "GitSignsUpdate",
				callback = function(args)
					if args.data and args.data.buffer == buf then
						vim.schedule(jump_if_ready)
					end
				end,
			})

			vim.schedule(jump_if_ready)
			vim.defer_fn(jump, 500)
		end

		local function switch_to_changed_file(direction, files, untracked_set)
			local current_file = fn.fnamemodify(api.nvim_buf_get_name(0), ":p")
			if not files then
				local root = git_root(current_file)
				if not root then
					return
				end
				files, untracked_set = get_changed_files(root)
			end
			if #files == 0 then
				return
			end

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
			vim.cmd("edit " .. fn.fnameescape(target))
			jump_to_hunk_on_attach(untracked_set[target] or false, direction)
		end

		local function nav_hunk_all_files(direction, files, untracked)
			show_hunk_linehl()

			local start_win = api.nvim_get_current_win()
			local start_buf = api.nvim_get_current_buf()
			local start_line = api.nvim_win_get_cursor(start_win)[1]
			local current_file = fn.fnamemodify(api.nvim_buf_get_name(start_buf), ":p")

			if untracked[current_file] then
				switch_to_changed_file(direction, files, untracked)
				return
			end

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

		local function walk(direction)
			cancel_jump()
			local root = git_root(fn.fnamemodify(api.nvim_buf_get_name(0), ":p"))
			if not root then
				api.nvim_echo({ { "Not in a git repository" } }, false, {})
				return
			end

			local files, untracked = get_changed_files(root)
			if #files == 0 then
				commit.nav(direction, root)
				return
			end

			commit.clear()
			nav_hunk_all_files(direction, files, untracked)
		end

		local function selected_lines() return { fn.line("."), fn.line("v") } end

		local function current_gitsigns_cache()
			local ok_cache, cache = pcall(require, "gitsigns.cache")
			if not ok_cache then
				return nil
			end

			return cache.cache[api.nvim_get_current_buf()]
		end

		local function current_hunk(hunks)
			if not hunks then
				return nil
			end

			local cache = current_gitsigns_cache()
			if not cache then
				return nil
			end

			return cache:get_cursor_hunk(hunks)
		end

		local function current_unstaged_hunk()
			local cache = current_gitsigns_cache()
			if not cache then
				return nil
			end

			return current_hunk(cache.hunks)
		end

		local function current_staged_hunk()
			local cache = current_gitsigns_cache()
			if not cache then
				return nil
			end

			return current_hunk(cache.hunks_staged)
		end

		local function next_unstaged_hunk_after(hunk)
			local cache = current_gitsigns_cache()
			if not cache or not cache.hunks then
				return nil
			end

			for _, next_hunk in ipairs(cache.hunks) do
				if next_hunk.added.start > hunk.added.start then
					return next_hunk
				end
			end

			return nil
		end

		local function jump_to_hunk(hunk)
			api.nvim_win_set_cursor(0, { math.max(hunk.added.start, 1), 0 })
			show_hunk_linehl()
		end

		local function fallback_keys(keys, fallback)
			if fallback and fallback.callback then
				fallback.callback()
				return
			end

			if fallback and fallback.rhs and fallback.rhs ~= "" then
				local mode = fallback.noremap == 1 and "n" or "m"
				api.nvim_feedkeys(api.nvim_replace_termcodes(fallback.rhs, true, true, true), mode, false)
				return
			end

			api.nvim_feedkeys(api.nvim_replace_termcodes(keys, true, true, true), "n", false)
		end

		local function hunk_header_parts(line)
			local removed_start, removed_count, added_start, added_count =
				line:match("^@@ %-(%d+),?(%d*) %+(%d+),?(%d*) @@")
			if not removed_start then
				return nil
			end

			return {
				removed_start = tonumber(removed_start),
				removed_count = tonumber(removed_count) or 1,
				added_start = tonumber(added_start),
				added_count = tonumber(added_count) or 1,
			}
		end

		local function hunk_matches(parts, hunk)
			return parts
				and parts.removed_start == hunk.removed.start
				and parts.removed_count == hunk.removed.count
				and parts.added_start == hunk.added.start
				and parts.added_count == hunk.added.count
		end

		local function diff_hunk(root, path, hunk, staged)
			local relpath = path:sub(#root + 2)
			local args = staged and { "diff", "--cached" } or { "diff" }
			local lines = git_lines(root, vim.list_extend(args, { "--unified=3", "--", relpath }))

			for index, line in ipairs(lines) do
				if hunk_matches(hunk_header_parts(line), hunk) then
					local hunk_lines = {}
					for hunk_index = index, #lines do
						local hunk_line = lines[hunk_index]
						if hunk_index > index and hunk_line:match("^@@ ") then
							break
						end

						table.insert(hunk_lines, hunk_line)
					end

					return table.concat(hunk_lines, "\n")
				end
			end

			return nil
		end

		local function patch_hunk(hunk)
			local ok_hunks, hunks = pcall(require, "gitsigns.hunks")
			if not ok_hunks then
				return hunk.head
			end

			local lines = { hunk.head }
			vim.list_extend(lines, hunks.patch_lines(hunk, vim.bo.fileformat))
			return table.concat(lines, "\n")
		end

		local function yank_hunk_or_diagnostics()
			local diagnostics = _G.context_yank_api and _G.context_yank_api.diagnostics
			if diagnostics and diagnostics({ silent_empty = true }) then
				return
			end

			local patch, short = commit.patch()
			local hunk, staged = nil, false
			if not patch then
				hunk = current_unstaged_hunk()
				if not hunk then
					hunk = current_staged_hunk()
					staged = true
				end
				if not hunk then
					vim.notify("No diagnostics or hunk under cursor", vim.log.levels.WARN)
					return
				end
			end

			local set_register = _G.context_yank_api and _G.context_yank_api.set_register
			if not set_register then
				vim.notify("context yank helpers are not loaded", vim.log.levels.ERROR)
				return
			end

			local path = fn.fnamemodify(api.nvim_buf_get_name(0), ":p")
			if patch then
				set_register(path, patch, "diff")
				vim.notify("Yanked " .. short .. " hunk", vim.log.levels.INFO)
				return
			end

			local root = git_root(path)
			local content = root and diff_hunk(root, path, hunk, staged) or nil
			set_register(path, content or patch_hunk(hunk), "diff")
			vim.notify("Yanked hunk", vim.log.levels.INFO)
		end

		local function stage_hunk_then_next(fallback)
			if commit.advance("next") then
				return
			end

			local hunk = current_unstaged_hunk()
			if not hunk then
				fallback_keys("<C-y>", fallback)
				return
			end
			local next_hunk = next_unstaged_hunk_after(hunk)

			gitsigns.stage_hunk(nil, nil, function(err)
				if err then
					vim.notify(err, vim.log.levels.ERROR)
					return
				end

				vim.schedule(function()
					if next_hunk then
						jump_to_hunk(next_hunk)
					else
						switch_to_changed_file("next")
					end
				end)
			end)
		end

		-- ╭─────────────────────────────────────────────────────────────────────╮
		-- │ keymaps: hunk operations, blame, diff                               │
		-- ╰─────────────────────────────────────────────────────────────────────╯

		local function on_attach(bufnr)
			local function map(mode, lhs, rhs, desc) vim.keymap.set(mode, lhs, rhs, { buffer = bufnr, desc = desc }) end
			local ctrl_y_fallback = fn.maparg("<C-y>", "n", false, true)
			local function next_hunk() nav_hunk("next") end
			local function prev_hunk() nav_hunk("prev") end
			local function stage_hunk_then_next_or_fallback() stage_hunk_then_next(ctrl_y_fallback) end
			local function stage_selection() gitsigns.stage_hunk(selected_lines()) end
			local function reset_selection() gitsigns.reset_hunk(selected_lines()) end
			local function blame_line_full() gitsigns.blame_line({ full = true }) end
			local function diff_head_previous() gitsigns.diffthis("~") end

			map("n", "]h", next_hunk, "Next hunk (file)")
			map("n", "[h", prev_hunk, "Previous hunk (file)")
			map("n", "<C-n>", function() walk("next") end, "Next hunk (all files, or cursor line's commit)")
			map("n", "<C-p>", function() walk("prev") end, "Previous hunk (all files, or cursor line's commit)")
			map("n", "<C-y>", stage_hunk_then_next_or_fallback, "Stage hunk and next")
			map("n", "<A-w>", yank_hunk_or_diagnostics, "Yank diagnostics or hunk")
			map("n", "<leader>hs", gitsigns.stage_hunk, "Stage hunk")
			map("n", "<leader>hr", gitsigns.reset_hunk, "Reset hunk")
			map("v", "<leader>hS", stage_selection, "Stage hunk")
			map("v", "<leader>hr", reset_selection, "Reset hunk")
			map("n", "<leader>hs", gitsigns.stage_buffer, "Stage buffer")
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
			attach_to_untracked = true,
			signs = {
				add = { text = "┃+" },
				change = { text = "┃◦" },
				untracked = { text = "" },
				delete = { text = "╏-" },
				topdelete = { text = "╏󰗩" },
				changedelete = { text = "╋⊘" },
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

		api.nvim_set_hl(0, "GitSignsUntracked", { link = "GitSignsAdd" })
	end,
}
