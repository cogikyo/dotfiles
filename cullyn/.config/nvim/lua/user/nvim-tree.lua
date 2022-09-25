-- vim.g.nvim_tree_icons = {
-- 	default = "",
-- 	symlink = "",
-- 	git = {
-- 		unstaged = "🞊",
-- 		staged = "",
-- 		unmerged = "",
-- 		renamed = "",
-- 		deleted = "",
-- 		untracked = "⧂",
-- 		ignored = "",
-- 	},
-- 	folder = {
-- 		arrow_open = "",
-- 		arrow_closed = "",
-- 		default = "",
-- 		open = "",
-- 		empty = "",
-- 		empty_open = "",
-- 		symlink = "",
-- 		symlink_open = "",
-- 	},
-- }
-- vim.g.nvim_tree_highlight_opened_files = 2
-- vim.g.nvim_tree_special_files = {
-- 	["Cargo.toml"] = true,
-- 	Makefile = true,
-- 	["README.md"] = true,
-- 	["readme.md"] = true,
-- }
--
local status_ok, nvim_tree = pcall(require, "nvim-tree")
if not status_ok then
	return
end

nvim_tree.setup({
	disable_netrw = true,
	hijack_netrw = true,
	open_on_setup = false,
	ignore_buffer_on_setup = false,
	ignore_ft_on_setup = {},
	auto_reload_on_write = true,
	open_on_tab = false,
	sort_by = "name",
	hijack_unnamed_buffer_when_opening = false,
	hijack_directories = {
		enable = true,
		auto_open = true,
	},
	hijack_cursor = true,
	renderer = {
		indent_markers = {
			enable = true,
			icons = {
				corner = "└╾ ",
				edge = "│ ",
				none = "  ",
			},
		},
	},
	update_cwd = false,
	update_focused_file = {
		enable = true,
		update_cwd = false,
		ignore_list = {},
	},
	system_open = {
		cmd = nil,
		args = {},
	},
	diagnostics = {
		enable = true,
		show_on_dirs = true,
		icons = {
			hint = "",
			info = "",
			warning = "",
			error = "",
		},
	},
	git = {
		enable = true,
		ignore = false,
		timeout = 420,
	},
	view = {
		hide_root_folder = false,
		width = 30,
		height = 30,
		side = "left",
		preserve_window_proportions = false,
		number = false,
		relativenumber = false,
		signcolumn = "yes",
		mappings = {
			custom_only = true,
			list = {
				{ key = { "o" }, action = "edit" },
				{ key = "]]", action = "cd" },
				{ key = "<C-v>", action = "vsplit" },
				{ key = "<C-h>", action = "split" },
				{ key = "<C-t>", action = "tabnew" },
				{ key = "<Up>", action = "prev_sibling" },
				{ key = "<Down>", action = "next_sibling" },
				{ key = "P", action = "parent_node" },
				{ key = "h", action = "close_node" },
				{ key = "<CR>", action = "preview" },
				{ key = "K", action = "first_sibling" },
				{ key = "J", action = "last_sibling" },
				{ key = "I", action = "toggle_git_ignored" },
				{ key = ".", action = "toggle_dotfiles" },
				{ key = "<C-r>", action = "refresh" },
				{ key = "t", action = "create" },
				{ key = "d", action = "trash" },
				{ key = "D", action = "remove" },
				{ key = "r", action = "rename" },
				{ key = "R", action = "full_rename" },
				{ key = "x", action = "cut" },
				{ key = "y", action = "copy" },
				{ key = "p", action = "paste" },
				{ key = "Y", action = "copy_path" },
				{ key = "gy", action = "copy_absolute_path" },
				{ key = "[c", action = "prev_git_item" },
				{ key = "]c", action = "next_git_item" },
				{ key = "-", action = "dir_up" },
				{ key = "O", action = "system_open" },
				{ key = "q", action = "close" },
				{ key = "?", action = "toggle_help" },
				{ key = "W", action = "collapse_all" },
				{ key = "S", action = "search_node" },
				{ key = "!", action = "run_file_command" },
				{ key = "<C-k>", action = "toggle_file_info" },
				{ key = "U", action = "toggle_custom" },
			},
		},
	},
	filters = {
		dotfiles = false,
		custom = {},
		exclude = {},
	},
	trash = {
		cmd = "trash-put",
		require_confirm = true,
	},
	actions = {
		change_dir = {
			enable = true,
			global = false,
		},
		open_file = {
			quit_on_open = true,
			resize_window = false,
			window_picker = {
				enable = true,
				chars = "asetniol",
				exclude = {
					filetype = { "notify", "packer", "qf", "diff", "fugitive", "fugitiveblame" },
					buftype = { "nofile", "terminal", "help" },
				},
			},
		},
	},
	log = {
		enable = false,
		truncate = false,
		types = {
			all = false,
			config = false,
			copy_paste = false,
			git = false,
			profile = false,
		},
	},
})
