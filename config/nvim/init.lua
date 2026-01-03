vim.g.mapleader = " "
vim.g.maplocalleader = "\\"

_G.preq = function(module)
	local ok, mod = pcall(require, module)
	if not ok then
		vim.notify(string.format("could not load module %s: %s", module, mod), vim.log.levels.ERROR)
		return nil
	end
	return mod
end

local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not (vim.uv or vim.loop).fs_stat(lazypath) then
	local out = vim.fn.system({
		"git",
		"clone",
		"--filter=blob:none",
		"--branch=stable",
		"https://github.com/folke/lazy.nvim.git",
		lazypath,
	})
	if vim.v.shell_error ~= 0 then
		vim.api.nvim_echo({
			{ "Failed to clone lazy.nvim:\n", "ErrorMsg" },
			{ out, "WarningMsg" },
			{ "\nPress any key to exit..." },
		}, true, {})
		vim.fn.getchar()
		os.exit(1)
	end
end
vim.opt.rtp:prepend(lazypath)

require("lazy").setup({
	dev = { path = "~/nvim" },
	spec = {
		{
			"cogikyo/vagari.nvim",
			dev = true,
			priority = 1000,
			lazy = false,
			config = function()
				vim.cmd.colorscheme("vagari")
			end,
		},

		{ import = "plugins.dev" },
		-- kristijanhusak/vim-dadbod-ui: database explorer and query UI
		-- lewis6991/gitsigns.nvim: inline git hunks and blame
		-- iamcco/markdown-preview.nvim: live preview in browser
		-- folke/trouble.nvim: diagnostics and lists viewer
		-- mfussenegger/nvim-dap: core debugging (plus UI/adapters)

		{ import = "plugins.editor" },
		-- windwp/nvim-autopairs: auto-close and wrap pairs
		-- brenoprata10/nvim-highlight-colors: render color literals
		-- nvim-telescope/telescope.nvim: fuzzy finder and pickers
		-- nvim-treesitter/nvim-treesitter: syntax, textobjects, autotag, commentstring
		-- folke/which-key.nvim: popup keybinding hints

		"cappyzawa/trim.nvim", -- trim trailing whitespace/newlines on save
		"AndrewRadev/switch.vim", -- cycle pairs/variants (true/false, etc.)
		"numToStr/Comment.nvim", -- comment toggling (gc/gb)
		"tpope/vim-surround", -- edit surroundings (quotes/parentheses)
		"tpope/vim-repeat", -- repeat supported plugin actions with .
		"mattn/emmet-vim", -- emmet-style HTML/CSS expansion

		{ import = "plugins.lsp" },
		-- stevearc/conform.nvim: formatter orchestration
		-- mfussenegger/nvim-lint: linter orchestration
		-- mfussenegger/nvim-dap: debugging wiring
		-- neovim/nvim-lspconfig: language server setup
		-- saghen/blink.cmp: completion capabilities
		"ThePrimeagen/refactoring.nvim", -- refactoring helpers via treesitter
		"elkowar/yuck.vim", -- yuck/eww syntax highlighting

		{ import = "plugins.ui" },
		-- goolord/alpha-nvim: start screen dashboard
		-- gen740/smoothcursor.nvim: animated cursor trail
		-- nvim-lualine/lualine.nvim: statusline with LSP progress
		-- "ThePrimeagen/harpoon": quick file marks/teleport
		-- nvim-tree/nvim-tree.lua: file explorer
		"ggandor/lightspeed.nvim", -- fast motion/word jumps
		"mbbill/undotree", -- visual undo tree panel
	},
	ui = { border = "rounded" },
})

require("config.settings")
require("config.keymaps")
require("config.autocmds")
