return {
	"nvim-tree/nvim-tree.lua",
	dependencies = { "nvim-tree/nvim-web-devicons" },
	config = function()
		local ok, nvim_tree = pcall(require, "nvim-tree")
		if not ok then
			return
		end

		local function on_attach(bufnr)
			local api = require("nvim-tree.api")
			local function map(lhs, rhs, d)
				vim.keymap.set(
					"n",
					lhs,
					rhs,
					{ desc = "nvim-tree: " .. d, buffer = bufnr, silent = true, nowait = true }
				)
			end

			-- stylua: ignore start
			map("<CR>",    api.node.open.edit,               "Open")
			map("o",       api.node.open.edit,               "Open")
			map("<Right>", api.node.open.edit,               "Open")
			map("zz",      api.tree.change_root_to_node,     "CD")
			map("<Up>",    api.node.navigate.sibling.prev,   "Previous Sibling")
			map("<Down>",  api.node.navigate.sibling.next,   "Next Sibling")
			map("<Left>",  api.node.navigate.parent,         "Parent Directory")
			map("<C-v>",   api.node.open.vertical,           "Open: Vertical Split")
			map("<C-h>",   api.node.open.horizontal,         "Open: Horizontal Split")
			map("<C-t>",   api.node.open.tab,                "Open: New Tab")
			map("zc",      api.node.navigate.parent_close,   "Close Directory")
			map("I",       api.tree.toggle_gitignore_filter, "Toggle Git Ignore")
			map(".",       api.tree.toggle_hidden_filter,    "Toggle Dotfiles")
			map("n",       api.fs.create,                    "Create")
			map("d",       function() pcall(api.fs.trash) end, "Trash")
			map("X",       api.fs.remove,                    "Delete")
			map("r",       api.fs.rename,                    "Rename")
			map("<C-r>",   api.fs.rename_sub,                "Rename: Omit Filename")
			map("R",       api.tree.reload,                  "Refresh")
			map("<C-x>",   api.fs.cut,                       "Cut")
			map("x",       api.fs.cut,                       "Cut")
			map("yy",      api.fs.copy.node,                 "Copy")
			map("p",       api.fs.paste,                     "Paste")
			map("yp",      api.fs.copy.relative_path,        "Copy Relative Path")
			map("yP",      api.fs.copy.absolute_path,        "Copy Absolute Path")
			map("[",       api.node.navigate.git.prev,       "Prev Git")
			map("]",       api.node.navigate.git.next,       "Next Git")
			map("O",       api.node.run.system,              "Run System")
			map("q",       api.tree.close,                   "Close")
			map("<Esc>",   api.tree.close,                   "Close")
			map("?",       api.tree.toggle_help,             "Help")
			map("zm",      api.tree.collapse_all,            "Collapse")
			map("zr",      api.tree.expand_all,              "Expand All")
			map("S",       api.tree.search_node,             "Search")
			map("<C-k>",   api.node.show_info_popup,         "Info")
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
				number = false,
				relativenumber = false,
				signcolumn = "no",
				float = {
					enable = true,
					open_win_config = function()
						local screen_w = vim.opt.columns:get()
						local screen_h = vim.opt.lines:get() - vim.opt.cmdheight:get()
						local window_w = screen_w * 0.66
						local window_h = screen_h * 0.66
						local window_w_int = math.floor(window_w)
						local window_h_int = math.floor(window_h)
						local center_x = (screen_w - window_w) / 2
						local center_y = ((vim.opt.lines:get() - window_h) / 2) - vim.opt.cmdheight:get()
						return {
							border = "rounded",
							relative = "editor",
							row = center_y,
							col = center_x,
							width = window_w_int,
							height = window_h_int,
						}
					end,
				},
			},
			renderer = {
				highlight_git = "icon",
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
					quit_on_open = true,
					window_picker = {
						chars = "asetniol",
					},
				},
			},
		})
	end,
}
