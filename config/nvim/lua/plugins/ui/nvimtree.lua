return {
	"nvim-tree/nvim-tree.lua",
	dependencies = { "nvim-tree/nvim-web-devicons" },
	config = function()
		local ok, nvim_tree = pcall(require, "nvim-tree")
		if not ok then
			return
		end

		nvim_tree.setup({
			on_attach = require("config.keymaps").nvimtree_on_attach,
			disable_netrw = true,
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
					git_placement = "before",
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
