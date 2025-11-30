-- ============================================================================
-- 📦 Bootstrap lazy.nvim {{{

vim.g.mapleader = " "
vim.g.maplocalleader = " "

local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.uv.fs_stat(lazypath) then
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
			{ "stevearc/conform.nvim", event = { "BufWritePre" }, config = true },
			{ "mfussenegger/nvim-lint", event = { "BufWritePost", "BufReadPost" } },
			{ "mason-org/mason.nvim", opts = {} },
			"mason-org/mason-lspconfig.nvim",
			"WhoIsSethDaniel/mason-tool-installer.nvim",
			{ "j-hui/fidget.nvim", opts = {} },
			"mfussenegger/nvim-dap",
			"saghen/blink.cmp",
		},
	}, -- }}}

	{ --🪄 Completion {{{
		"saghen/blink.cmp",
		event = "VimEnter",
		version = "1.*",
		dependencies = {
			{
				"L3MON4D3/LuaSnip",
				version = "2.*",
				build = (function()
					if vim.fn.has("win32") == 1 or vim.fn.executable("make") == 0 then
						return
					end
					return "make install_jsregexp"
				end)(),
			},
			{
				"folke/lazydev.nvim",
				ft = "lua",
				opts = {
					library = {
						{ path = "${3rd}/luv/library", words = { "vim%.uv" } },
					},
				},
			},
		},
		opts = {
			keymap = { preset = "default" },
			appearance = { nerd_font_variant = "mono" },
			completion = { documentation = { auto_show = true, auto_show_delay_ms = 200 } },
			sources = {
				default = { "lsp", "path", "snippets", "lazydev" },
				providers = {
					lazydev = { module = "lazydev.integrations.blink", score_offset = 100 },
				},
			},
			snippets = { preset = "luasnip" },
			fuzzy = { implementation = "prefer_rust_with_warning" },
			signature = { enabled = true },
		},
	}, -- }}}

	{ --🎄 Treesitter {{{
		"nvim-treesitter/nvim-treesitter",
		build = ":TSUpdate",
		dependencies = {
			"nvim-treesitter/nvim-treesitter-textobjects",
			"nvim-treesitter/nvim-treesitter-context",
			"JoosepAlviste/nvim-ts-context-commentstring",
			"windwp/nvim-ts-autotag",
			"windwp/nvim-autopairs",
		},
	}, -- }}}

	{ --🎨 Colorscheme {{{
		"nosvagor/vagari.nvim",
		priority = 1000,
		config = function()
			vim.cmd.colorscheme("vagari")
		end,
	},
	-- }}}


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


	{ 
		"brenoprata10/nvim-highlight-colors",
		opts = {
			render = "background",
			enable_named_colors = false,
			enable_tailwind = true,
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

	"lewis6991/gitsigns.nvim",
	"tpope/vim-surround",
	"tpope/vim-repeat",
	"tpope/vim-fugitive",
	"tpope/vim-rhubarb",

	"AndrewRadev/switch.vim",
	"elkowar/yuck.vim",
	"gen740/SmoothCursor.nvim",

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
	}, 

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
	"gitsigns", -- git signs and hunk actions
	"lualine", -- status line
	"nvimtree", -- file explorer
	"telescope", -- fuzzy finder
	"treesitter", -- treesitter and related
	"autopairs", -- autopair configs and custom functions
	"lsp", -- lsp and related config
	"cursor", -- smooth cursor
	-- ------------------------------------------------------------------------
}

for _, file in ipairs(user_config) do
	require("user." .. file)
end
-- ============================================================================
