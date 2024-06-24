local ok, nvim_tree = pcall(require, "nvim-tree")

if not ok then
	vim.api.nvim_echo({
		{
			"Error: nvim-tree plugin not found... skipping relevant setup()",
			"Error",
		},
	}, true, {})
	return
end

local function on_attach(bufnr)
	local api = require("nvim-tree.api")

	local function opts(desc)
		return { desc = "nvim-tree: " .. desc, buffer = bufnr, noremap = true, silent = true, nowait = true }
	end

	vim.keymap.set("n", "<CR>", api.node.open.edit, opts("Open"))
	vim.keymap.set("n", "o", api.node.open.edit, opts("Open"))
	vim.keymap.set("n", "<Right>", api.node.open.edit, opts("Open"))
	vim.keymap.set("n", "zz", api.tree.change_root_to_node, opts("CD"))
	vim.keymap.set("n", "<Up>", api.node.navigate.sibling.prev, opts("Previous Sibling"))
	vim.keymap.set("n", "<Down>", api.node.navigate.sibling.next, opts("Next Sibling"))
	vim.keymap.set("n", "<Left>", api.node.navigate.parent, opts("Parent Directory"))
	vim.keymap.set("n", "<C-v>", api.node.open.vertical, opts("Open: Vertical Split"))
	vim.keymap.set("n", "<C-h>", api.node.open.horizontal, opts("Open: Horizontal Split"))
	vim.keymap.set("n", "<C-t>", api.node.open.tab, opts("Open: New Tab"))
	vim.keymap.set("n", "zc", api.node.navigate.parent_close, opts("Close Directory"))
	vim.keymap.set("n", "I", api.tree.toggle_gitignore_filter, opts("Toggle Git Ignore"))
	vim.keymap.set("n", ".", api.tree.toggle_hidden_filter, opts("Toggle Dotfiles"))
	vim.keymap.set("n", "n", api.fs.create, opts("Create"))
	vim.keymap.set("n", "d", api.fs.trash, opts("Trash"))
	vim.keymap.set("n", "X", api.fs.remove, opts("Delete"))
	vim.keymap.set("n", "r", api.fs.rename, opts("Rename"))
	vim.keymap.set("n", "<C-r>", api.fs.rename_sub, opts("Rename: Omit Filename"))
	vim.keymap.set("n", "R", api.tree.reload, opts("Refresh"))
	vim.keymap.set("n", "<C-x>", api.fs.cut, opts("Cut"))
	vim.keymap.set("n", "yy", api.fs.copy.node, opts("Copy"))
	vim.keymap.set("n", "p", api.fs.paste, opts("Paste"))
	vim.keymap.set("n", "yp", api.fs.copy.relative_path, opts("Copy Relative Path"))
	vim.keymap.set("n", "yP", api.fs.copy.absolute_path, opts("Copy Absolute Path"))
	vim.keymap.set("n", "[", api.node.navigate.git.prev, opts("Prev Git"))
	vim.keymap.set("n", "]", api.node.navigate.git.next, opts("Next Git"))
	vim.keymap.set("n", "O", api.node.run.system, opts("Run System"))
	vim.keymap.set("n", "q", api.tree.close, opts("Close"))
	vim.keymap.set("n", "<Esc>", api.tree.close, opts("Close"))
	vim.keymap.set("n", "?", api.tree.toggle_help, opts("Help"))
	vim.keymap.set("n", "zm", api.tree.collapse_all, opts("Collapse"))
	vim.keymap.set("n", "zr", api.tree.expand_all, opts("Expand All"))
	vim.keymap.set("n", "S", api.tree.search_node, opts("Search"))
	vim.keymap.set("n", "<C-k>", api.node.show_info_popup, opts("Info"))
end

nvim_tree.setup({
	on_attach = on_attach,
	disable_netrw = false,
	hijack_cursor = true,
	update_focused_file = { enable = true },
	diagnostics = { enable = true },
	modified = { enable = true },
	view = {
		number = true,
		cursorline = false,
		relativenumber = true,
		signcolumn = "yes",
		float = {
			enable = true,
			open_win_config = function()
				local screen_w = vim.opt.columns:get()
				local screen_h = vim.opt.lines:get() - vim.opt.cmdheight:get()
				local window_w = screen_w * 0.66
				local window_h = screen_h * 0.33
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
		highlight_git = true,
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
			glyphs = {
				default = "",
				symlink = "",
				git = {
					unstaged = "🞊",
					staged = "",
					unmerged = "",
					renamed = "",
					deleted = "",
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
