return {
	"nvim-tree/nvim-tree.lua",
	dependencies = { "nvim-tree/nvim-web-devicons" },
	config = function()
		local ok, nvim_tree = pcall(require, "nvim-tree")
		if not ok then
			return
		end
		local NestedGit = require("config.nvim-tree-nested-git")
		local TestFiles = require("nvim-tree.api").Decorator:extend()

		local test_file_patterns = {
			"_test%.go$",
			"%.test%.[%w_%-]+$",
			"%.spec%.[%w_%-]+$",
			"%.cy%.[jt]sx?$",
			"%.e2e%.[jt]sx?$",
			"^test_.*%.py$",
			"_test%.py$",
			"_test%.rb$",
			"_spec%.rb$",
		}

		local function is_test_file(name)
			for _, pattern in ipairs(test_file_patterns) do
				if name:match(pattern) then
					return true
				end
			end

			return false
		end

		function TestFiles:new()
			self.enabled = true
			self.highlight_range = "name"
			self.icon_placement = "none"
		end

		function TestFiles:highlight_group(node)
			if node.type == "file" and is_test_file(node.name or "") then
				return "NvimTreeTestFile"
			end
		end

		vim.api.nvim_set_hl(0, "NvimTreeTestFile", { link = "Comment", default = true })

		local function on_attach(bufnr)
			local api = require("nvim-tree.api")
			local core = require("nvim-tree.core")
			local view = require("nvim-tree.view")
			local function map(lhs, rhs, d)
				vim.keymap.set(
					"n",
					lhs,
					rhs,
					{ desc = "nvim-tree: " .. d, buffer = bufnr, silent = true, nowait = true }
				)
			end

			local function cursor_node()
				return api.tree.get_node_under_cursor()
			end

			local function is_file(node)
				return node and node.type == "file"
			end

			local function is_open_dir(node)
				return node and node.type == "directory" and node.open
			end

			local function is_closed_dir(node)
				return node and node.type == "directory" and not node.open
			end

			local function nav_right()
				local node = cursor_node()
				if is_file(node) then
					api.node.open.edit()
				elseif is_open_dir(node) then
					vim.cmd("normal! j")
				else
					api.node.open.edit()
				end
			end

			local function nav_left()
				local node = cursor_node()
				if is_open_dir(node) then
					api.node.navigate.parent_close()
				else
					api.node.navigate.parent()
				end
			end

			local function open_and_close()
				local node = cursor_node()
				if is_file(node) then
					api.node.open.edit()
					api.tree.close()
				else
					api.node.open.edit()
				end
			end

			local function expand_one_level()
				local node = cursor_node()
				if is_closed_dir(node) then
					api.node.open.edit()
				elseif is_open_dir(node) then
					for _, child in ipairs(node.nodes or {}) do
						if is_closed_dir(child) then
							api.tree.find_file({ buf = child.absolute_path, focus = true })
							api.node.open.edit()
						end
					end
					api.tree.find_file({ buf = node.absolute_path, focus = true })
				end
			end

			local function reveal_current_file()
				for _, w in ipairs(vim.api.nvim_list_wins()) do
					local name = vim.api.nvim_buf_get_name(vim.api.nvim_win_get_buf(w))
					if name ~= "" and not name:match("NvimTree_") and vim.api.nvim_win_get_config(w).relative == "" then
						api.tree.find_file({ buf = name, open = true, focus = true })
						return
					end
				end
			end

			local function fuzzy_find_dir()
				local cwd = require("nvim-tree.core").get_cwd()
				require("telescope.builtin").find_files({
					prompt_title = "Jump to directory",
					cwd = cwd,
					find_command = { "fd", "--type", "d", "--hidden", "--exclude", ".git" },
					attach_mappings = function(_, _)
						local actions = require("telescope.actions")
						local action_state = require("telescope.actions.state")
						actions.select_default:replace(function(prompt_bufnr)
							local entry = action_state.get_selected_entry()
							actions.close(prompt_bufnr)
							if entry then
								api.tree.find_file({ buf = cwd .. "/" .. entry[1], open = true, focus = true })
							end
						end)
						return true
					end,
				})
			end

			local function copy_absolute_path_with_spacing()
				local node = cursor_node()
				if not node or not node.absolute_path then
					return
				end

				vim.fn.setreg("+", node.absolute_path .. "\n\n")
				vim.notify("Copied absolute path", vim.log.levels.INFO)
			end

			local function has_git_status(node)
				if not node then
					return false
				end

				if type(node.get_git_xy) == "function" then
					local ok_status, status = pcall(node.get_git_xy, node)
					if ok_status and status ~= nil and #status > 0 then
						return true
					end
				end

				return NestedGit.is_dirty(node)
			end

			local function navigate_git(direction)
				local explorer = core.get_explorer()
				if not explorer then
					return
				end

				local first_line = core.get_nodes_starting_line()
				local nodes_by_line = explorer:get_nodes_by_line(first_line)
				local cursor = explorer:get_cursor_position()
				local cursor_line = cursor and cursor[1] or first_line
				local first_git_line
				local next_git_line

				local start_line = direction == "next" and first_line or #nodes_by_line
				local end_line = direction == "next" and #nodes_by_line or first_line
				local step = direction == "next" and 1 or -1

				for line = start_line, end_line, step do
					if has_git_status(nodes_by_line[line]) then
						first_git_line = first_git_line or line

						if
							(direction == "next" and line > cursor_line) or (direction == "prev" and line < cursor_line)
						then
							next_git_line = line
							break
						end
					end
				end

				if next_git_line then
					view.set_cursor({ next_git_line, 0 })
				elseif vim.o.wrapscan and first_git_line then
					view.set_cursor({ first_git_line, 0 })
				end
			end

			-- stylua: ignore start
			-- navigation ──────────────────────────────────────────────────
			map("<Up>",    function() vim.cmd("normal! k") end, "Up")
			map("<Down>",  function() vim.cmd("normal! j") end, "Down")
			map("<Right>", nav_right,                         "Open/enter dir/open file")
			map("<Left>",  nav_left,                          "Close dir/go to parent")
			map("[",       function() navigate_git("prev") end, "Prev Git")
			map("]",       function() navigate_git("next") end, "Next Git")

			-- open / close ────────────────────────────────────────────────
			map("<CR>",    api.node.open.preview,             "Preview")
			map("o",       open_and_close,                    "Open and close tree")
			map("<C-v>",   api.node.open.vertical,            "Open: Vertical Split")
			map("<C-h>",   api.node.open.horizontal,          "Open: Horizontal Split")
			map("O",       api.node.run.system,               "Run System")
			map("q",       api.tree.close,                    "Close")
			map("<Esc>",   function() vim.cmd("wincmd p") end, "Back to editor")

			-- fold / expand ───────────────────────────────────────────────
			map("zc",      api.tree.collapse_all,             "Collapse all")
			map("zr",      expand_one_level,                  "Expand one level")
			map("zR",      api.tree.expand_all,               "Expand all")
			map("zf",      reveal_current_file,               "Reveal current file")

			-- search / jump ───────────────────────────────────────────────
			map("f",       fuzzy_find_dir,                    "Fuzzy find directory")

			-- file operations ─────────────────────────────────────────────
			map("n",       api.fs.create,                    "Create")
			map("r",       api.fs.rename,                    "Rename")
			map("<C-r>",   api.fs.rename_sub,                "Rename: Omit Filename")
			map("d",       function() pcall(api.fs.trash) end, "Trash")
			map("X",       api.fs.remove,                    "Delete")
			map("x",       api.fs.cut,                       "Cut")
			map("<C-x>",   api.fs.cut,                       "Cut")
			map("yy",      api.fs.copy.node,                 "Copy")
			map("p",       api.fs.paste,                     "Paste")

			-- copy paths ──────────────────────────────────────────────────
			map("yp",      api.fs.copy.relative_path,        "Copy Relative Path")
			map("yP",      api.fs.copy.absolute_path,        "Copy Absolute Path")
			map("<A-f>",   copy_absolute_path_with_spacing,  "Copy Absolute Path")
			map("<A-F>",   copy_absolute_path_with_spacing,  "Copy Absolute Path")

			-- tree settings ───────────────────────────────────────────────
			map("cd",      api.tree.change_root_to_node,     "CD")
			map("c.",      api.tree.change_root_to_parent,   "Up")
			map("I",       api.tree.toggle_gitignore_filter, "Toggle Git Ignore")
			map(".",       api.tree.toggle_hidden_filter,    "Toggle Dotfiles")
			map("R",       api.tree.reload,                  "Refresh")
			map("<C-k>",   api.node.show_info_popup,         "Info")
			map("?",       api.tree.toggle_help,             "Help")
			-- stylua: ignore end
		end

		nvim_tree.setup({
			on_attach = on_attach,
			disable_netrw = true,
			filesystem_watchers = {
				ignore_dirs = { ".git" },
			},
			hijack_cursor = true,
			update_focused_file = { enable = true },
			diagnostics = { enable = true },
			modified = { enable = true },
			view = {
				width = {
					min = 25,
					max = 45,
				},
				side = "left",
				number = false,
				relativenumber = false,
				signcolumn = "no",
			},
			renderer = {
				decorators = {
					"Git",
					"Open",
					"Hidden",
					"Modified",
					"Bookmark",
					"Diagnostics",
					"Copied",
					NestedGit,
					TestFiles,
					"Cut",
				},
				root_folder_label = false,
				highlight_git = "all",
				indent_markers = {
					enable = true,
					inline_arrows = true,
					icons = {
						corner = "└╾",
						edge = "│ ",
						item = "├",
						none = " ",
					},
				},
				icons = {
					git_placement = "after",
					web_devicons = {
						file = { color = false },
						folder = { color = true },
					},
					glyphs = {
						default = "",
						symlink = "",
						git = {
							unstaged = "",
							staged = "",
							unmerged = "",
							renamed = "",
							deleted = "󰮈",
							untracked = "",
							ignored = "",
						},
					},
				},
			},
			trash = {
				cmd = "trash-put",
				require_confirm = true,
			},
			actions = {
				open_file = {
					quit_on_open = false,
					window_picker = {
						chars = "asetniol",
					},
				},
			},
		})

		vim.api.nvim_create_autocmd("QuitPre", {
			callback = function()
				local tree_wins = {}
				local floating_wins = {}
				local wins = vim.api.nvim_list_wins()
				for _, w in ipairs(wins) do
					local bufname = vim.api.nvim_buf_get_name(vim.api.nvim_win_get_buf(w))
					if bufname:match("NvimTree_") ~= nil then
						table.insert(tree_wins, w)
					end
					if vim.api.nvim_win_get_config(w).relative ~= "" then
						table.insert(floating_wins, w)
					end
				end
				if #wins - #floating_wins - #tree_wins == 1 then
					for _, w in ipairs(tree_wins) do
						vim.api.nvim_win_close(w, true)
					end
				end
			end,
		})
	end,
}
