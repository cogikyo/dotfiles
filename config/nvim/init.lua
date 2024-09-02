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
			-- "chrisgrieser/cmp-nerdfont",
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
	},
	-- }}}

	{ --📰 Pretty-fold {{{
		-- "anuvyklack/pretty-fold.nvim",
		-- opts = {
		-- 	sections = {
		-- 		left = {
		-- 			"╘╾",
		-- 			"content",
		-- 			"⮯ ",
		-- 		},
		-- 		right = {
		-- 			" ",
		-- 			"number_of_folded_lines",
		-- 			": ",
		-- 			"percentage",
		-- 			" ╼╕",
		-- 		},
		-- 	},
		-- 	fill_char = "⋅",
		-- 	process_comment_signs = "delete",
		-- },
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
				changedelete = { text = "❍" },
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

	{ -- 🚦Trouble {{{
		"folke/trouble.nvim",
		opts = {}, -- for default options, refer to the configuration section for custom setup.
		cmd = "Trouble",
		keys = {
			{
				"<leader>fd",
				"<cmd>Trouble diagnostics toggle<cr>",
				desc = "Diagnostics (Trouble)",
			},
			{
				"<leader>ft",
				"<cmd>Trouble diagnostics toggle filter.buf=0<cr>",
				desc = "Buffer Diagnostics (Trouble)",
			},
			{
				"<leader>fs",
				"<cmd>Trouble symbols toggle focus=false<cr>",
				desc = "Symbols (Trouble)",
			},
			{
				"<leader>fp",
				"<cmd>Trouble lsp toggle focus=false win.position=right<cr>",
				desc = "LSP Definitions / references / ... (Trouble)",
			},
			{
				"<leader>fl",
				"<cmd>Trouble loclist toggle<cr>",
				desc = "Location List (Trouble)",
			},
			{
				"<leader>fq",
				"<cmd>Trouble qflist toggle<cr>",
				desc = "Quickfix List (Trouble)",
			},
		},
	},

	-- }}}

	{ --🪩 Simple setup {{{
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
		config = function()
			vim.g.mkdp_filetypes = { "markdown", "html" }
			vim.g.mkdp_auto_start = 1
			vim.g.mkdp_auto_close = 0
			vim.g.mkdp_refresh_slow = 1
		end,
	},
	{
		"kristijanhusak/vim-dadbod-ui",
		dependencies = {
			{ "tpope/vim-dadbod", lazy = true },
			{ "kristijanhusak/vim-dadbod-completion", ft = { "postgres" }, lazy = true }, -- Optional
		},
		cmd = {
			"DBUI",
			"DBUIToggle",
			"DBUIAddConnection",
			"DBUIFindBuffer",
		},
		init = function()
			-- Your DBUI configuration
			vim.g.db_ui_use_nerd_fonts = 1
		end,
	},

	{ "numToStr/Comment.nvim", opts = {} },
	{ "cappyzawa/trim.nvim", opts = {} },
	"ggandor/lightspeed.nvim",
	"mbbill/undotree",
	"ThePrimeagen/harpoon",
	{
		"ThePrimeagen/refactoring.nvim",
		dependencies = {
			{ "nvim-lua/plenary.nvim" },
			{ "nvim-treesitter/nvim-treesitter" },
		},
		opts = {},
	},
	"mattn/emmet-vim",

	{
		"barrett-ruth/live-server.nvim",
		config = true,
	},

	"tpope/vim-surround",
	"tpope/vim-repeat",
	"tpope/vim-fugitive",
	"tpope/vim-rhubarb",

	"AndrewRadev/switch.vim",
	"elkowar/yuck.vim",

	{
		"nvim-telescope/telescope.nvim",
		dependencies = {
			"nvim-lua/plenary.nvim",
			"natecraddock/telescope-zf-native.nvim",
		},
	},
	{
		"kyazdani42/nvim-tree.lua",
		dependencies = {
			{ "antosha417/nvim-lsp-file-operations", opts = {} },
			{ "kyazdani42/nvim-web-devicons", opts = {} },
			{ "nvim-lua/plenary.nvim" },
		},
	},
	{
		"nvim-lualine/lualine.nvim",
		dependencies = { "arkav/lualine-lsp-progress" },
	},
	{
		"goolord/alpha-nvim",
		dependencies = { "kyazdani42/nvim-web-devicons" },
	}, -- }}}
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
	"nvimtree", -- file explorer
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
