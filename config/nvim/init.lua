-- ============================================================================
-- 📦 Bootstrap lazy.nvim {{{

vim.g.mapleader = " "
vim.g.maplocalleader = " "

local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.loop.fs_stat(lazypath) then
	vim.fn.system({
		"git",
		"clone",
		"--filter=blob:none",
		"https://github.com/folke/lazy.nvim.git",
		"--branch=stable", -- latest stable release
		lazypath,
	})
end
vim.opt.rtp:prepend(lazypath)

-- }}}
-- ============================================================================

require("lazy").setup({
	--	-----------------------------------------------------------------------
	{ --📚 LSP {{{
		"neovim/nvim-lspconfig",
		dependencies = {
			"williamboman/mason.nvim",
			"williamboman/mason-lspconfig.nvim",
			"mfussenegger/nvim-dap",
			"jose-elias-alvarez/null-ls.nvim",
			{ "folke/neodev.nvim", opts = {} },
		},
	}, -- }}}

	{ --🪄 Completion {{{
		"hrsh7th/nvim-cmp",
		dependencies = {
			"hrsh7th/nvim-cmp",
			"hrsh7th/cmp-nvim-lsp",
			"hrsh7th/cmp-nvim-lua",
			"hrsh7th/cmp-buffer",
			"hrsh7th/cmp-path",
			"hrsh7th/cmp-emoji",
			"chrisgrieser/cmp-nerdfont",
			"L3MON4D3/LuaSnip",
			"saadparwaiz1/cmp_luasnip",
			"rafamadriz/friendly-snippets",
			{
				"zbirenbaum/copilot.lua",
				cmd = "Copilot",
				event = "InsertEnter",
				dependencies = { "zbirenbaum/copilot-cmp" },
			},
		},
	}, -- }}}

	{ --🎄 Treesitter {{{
		"nvim-treesitter/nvim-treesitter",
		dependencies = {
			"nvim-treesitter/nvim-treesitter-textobjects", -- use treesitter text objects to do cools things
			"nvim-treesitter/playground", -- view treesitter info directly in nvim
			"JoosepAlviste/nvim-ts-context-commentstring", -- dynamic 'commentstring' based on cursor location rather than file:
			"windwp/nvim-ts-autotag", -- auto close tags
			"windwp/nvim-autopairs", -- treesitter aware pairing and more
		},
		config = function()
			pcall(require("nvim-treesitter.install").update({ with_sync = true }))
		end,
	}, -- }}}

	{ --🎨 Colorscheme {{{
		"nosvagor/vagari.nvim",
		priority = 1000,
		config = function()
			vim.cmd.colorscheme("vagari")
		end,
	}, -- }}}

	{ --📰 Pretty-fold {{{
		"anuvyklack/pretty-fold.nvim",
		opts = {
			sections = {
				left = {
					"╘╾",
					"content",
					"⮯ ",
				},
				right = {
					" ",
					"number_of_folded_lines",
					": ",
					"percentage",
					" ╼╕",
				},
			},
			fill_char = "⋅",
			process_comment_signs = "delete",
		},
	}, -- }}}

	{ --🧿 GitSigns {{{
		"lewis6991/gitsigns.nvim",
		opts = {

			signs = {
				add = { text = "" },
				change = { text = "🞊" },
				untracked = { text = "" },
				delete = { text = "" },
				topdelete = { text = "🕱" },
				changedelete = { text = "" },
			},

			signcolumn = true, -- Toggle with `:Gitsigns toggle_signs`
			numhl = false, -- Toggle with `:Gitsigns toggle_numhl`
			linehl = false, -- Toggle with `:Gitsigns toggle_linehl`
			word_diff = false, -- Toggle with `:Gitsigns toggle_word_diff`
			current_line_blame = false, -- Toggle with `:Gitsigns toggle_current_line_blame`

			-- Keymaps
			on_attach = function(bufnr)
				local gs = package.loaded.gitsigns

				local function map(mode, l, r, opts)
					opts = opts or {}
					opts.buffer = bufnr
					vim.keymap.set(mode, l, r, opts)
				end

				-- Navigation
				map("n", "]c", function()
					if vim.wo.diff then
						return "]c"
					end
					vim.schedule(function()
						gs.next_hunk()
					end)
					return "<Ignore>"
				end, { expr = true })

				map("n", "[c", function()
					if vim.wo.diff then
						return "[c"
					end
					vim.schedule(function()
						gs.prev_hunk()
					end)
					return "<Ignore>"
				end, { expr = true })

				-- Actions
				map({ "n", "v" }, "<leader>hs", ":Gitsigns stage_hunk<CR>")
				map({ "n", "v" }, "<leader>hr", ":Gitsigns reset_hunk<CR>")
				map("n", "<leader>hS", gs.stage_buffer)
				map("n", "<leader>hu", gs.undo_stage_hunk)
				map("n", "<leader>hR", gs.reset_buffer)
				map("n", "<leader>hp", gs.preview_hunk)
				map("n", "<leader>hb", function()
					gs.blame_line({ full = true })
				end)
				map("n", "<leader>hB", gs.toggle_current_line_blame)
				map("n", "<leader>hd", gs.toggle_deleted)
				map("n", "<leader>hD", gs.diffthis)

				-- Text object
				map({ "o", "x" }, "ih", ":<C-U>Gitsigns select_hunk<CR>")
			end,
		},
	}, -- }}}

	{ --🗂️ Nvim-Tree {{{

		"kyazdani42/nvim-tree.lua",
		dependencies = {
			{ "antosha417/nvim-lsp-file-operations", opts = {} },
			{ "kyazdani42/nvim-web-devicons", opts = {} },
			"nvim-lua/plenary.nvim",
		},
		opts = {
			disable_netrw = false,
			hijack_cursor = true,
			update_focused_file = { enable = true },
			diagnostics = { enable = true },
			modified = { enable = true },
			remove_keymaps = true,
			view = {
				number = true,
				cursorline = false,
				relativenumber = true,
				signcolumn = "yes",
				mappings = {
					custom_only = true,
					list = {
						{ key = { "<CR>", "o", "<Right>" }, action = "edit" },
						{ key = "zz", action = "cd" },
						{ key = "<Up>", action = "prev_sibling" },
						{ key = "<Down>", action = "next_sibling" },
						{ key = "<Left>", action = "parent_node" },
						{ key = "<C-v>", action = "vsplit" },
						{ key = "<C-h>", action = "split" },
						{ key = "<C-t>", action = "tabnew" },
						{ key = "zc", action = "close_node" },
						{ key = "I", action = "toggle_git_ignored" },
						{ key = ".", action = "toggle_dotfiles" },
						{ key = "n", action = "create" },
						{ key = "x", action = "trash" },
						{ key = "X", action = "remove" },
						{ key = "r", action = "rename" },
						{ key = "<C-r>", action = "full_rename" },
						{ key = "R", action = "refresh" },
						{ key = "d", action = "cut" },
						{ key = "yy", action = "copy" },
						{ key = "p", action = "paste" },
						{ key = "yp", action = "copy_path" },
						{ key = "yP", action = "copy_absolute_path" },
						{ key = "[", action = "prev_git_item" },
						{ key = "]", action = "next_git_item" },
						{ key = "O", action = "system_open" },
						{ key = { "q", "<Esc>" }, action = "close" },
						{ key = "?", action = "toggle_help" },
						{ key = "zm", action = "collapse_all" },
						{ key = "zr", action = "expand_all" },
						{ key = "S", action = "search_node" },
						{ key = "<C-k>", action = "toggle_file_info" },
					},
				},
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
							renamed = "",
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
		},
	}, -- }}}

	-- 🧩 -> /lua/user/{plugin}.lua {{{

	{
		"nvim-telescope/telescope.nvim",
		dependencies = {
			"nvim-lua/plenary.nvim",
			"natecraddock/telescope-zf-native.nvim",
		},
	},
	{
		"nvim-lualine/lualine.nvim",
		dependencies = { "arkav/lualine-lsp-progress" },
	},
	{
		"goolord/alpha-nvim",
		dependencies = { "kyazdani42/nvim-web-devicons" },
	},

	-- }}}

	-- 🪩    simple setup {{{
	{
		"brenoprata10/nvim-highlight-colors",
		opts = {
			render = "background",
			enable_named_colors = false,
			enable_tailwind = false,
		},
	},

	{
		"iamcco/markdown-preview.nvim",
		build = function()
			vim.fn["mkdp#util#install"]()
		end,
	},

	{ "numToStr/Comment.nvim", opts = {} },
	{ "folke/trouble.nvim", ops = {} },
	{ "cappyzawa/trim.nvim", opts = {} },

	"mbbill/undotree",
	"ThePrimeagen/harpoon",
	"mattn/emmet-vim",

	"tpope/vim-surround",
	"tpope/vim-repeat",
	"tpope/vim-fugitive",
	"tpope/vim-rhubarb",

	"AndrewRadev/switch.vim",
	"elkowar/yuck.vim",

	-- }}}
	--	-----------------------------------------------------------------------
}, { -- opts:
	ui = { border = "rounded" },
	checker = {
		enabled = true,
		notify = false,
	},
})

-- ============================================================================
local user_config = {

	-- custom ⮯ ---------------------------------------------------------------
	"settings", -- edit default options/settings for neovim
	"keymaps", -- most custom keymaps (some are defined in plugin opts above)
	"autocmds", -- custom automatic functions
	-- ------------------------------------------------------------------------

	-- plugins ⮯ --------------------------------------------------------------
	"alpha", -- welcome screen
	"cmp", -- completion, snippets, and related
	"lualine", -- status line
	"telescope", -- fuzzy finder
	"treesitter", -- treesitter and related
	"autopairs", -- autopair configs and custom functions
	"lsp", -- lsp and related config
	-- ------------------------------------------------------------------------
}

for _, file in ipairs(user_config) do
	require("user." .. file)
end
-- ============================================================================
