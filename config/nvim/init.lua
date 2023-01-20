-- ============================================================================
-- 📦 Prepwork (auto install packer) 🢢 {{{
local install_path = vim.fn.stdpath 'data' .. '/site/pack/packer/start/packer.nvim'
local is_bootstrap = false
if vim.fn.empty(vim.fn.glob(install_path)) > 0 then
  is_bootstrap = true
  vim.fn.system { 'git', 'clone', '--depth', '1', 'https://github.com/wbthomason/packer.nvim', install_path }
  vim.cmd [[packadd packer.nvim]]
end

-- Minor adjustments to packer config
require('packer').init({
  display = {
    open_fn = function()
      return require("packer.util").float({ border = "rounded" })
    end,
    keybindings = { -- Keybindings for the display window
      quit = '<Esc>',
      toggle_update = 'u', -- only in preview
      continue = 'c', -- only in preview
      toggle_info = 'i',
      diff = 'd',
      prompt_revert = 'r',
    }
  },

})

require('packer').startup(function(use)
-- }}}
-- ============================================================================

  -- 📚 LSP: ⮯ {{{
  use {
    'neovim/nvim-lspconfig',
    requires = {
      'williamboman/mason.nvim',
      'williamboman/mason-lspconfig.nvim',
      'j-hui/fidget.nvim',
      'folke/neodev.nvim',
    },
  }
  -- }}}

  -- 🪄 Completion: ⮯ {{{
  use {
    'hrsh7th/nvim-cmp',
    requires = {
      'hrsh7th/cmp-nvim-lsp',
      'hrsh7th/cmp-nvim-lua',
      'hrsh7th/nvim-cmp',
      'hrsh7th/cmp-buffer',
      'hrsh7th/cmp-path',
      'hrsh7th/cmp-cmdline',
      'hrsh7th/cmp-calc',
      'hrsh7th/cmp-emoji',
      'max397574/cmp-greek',
      'L3MON4D3/LuaSnip',
      'saadparwaiz1/cmp_luasnip',
      'rafamadriz/friendly-snippets',
    },
  }
  -- }}}

  -- 🌲 Treesitter: ⮯ {{{
  use {
    'nvim-treesitter/nvim-treesitter',
    run = function()
      pcall(require('nvim-treesitter.install').update { with_sync = true })
    end,
  }
  use { -- Additional text objects via treesitter
    'nvim-treesitter/nvim-treesitter-textobjects',
    after = 'nvim-treesitter',
  }
  use "nvim-treesitter/playground"
  use "JoosepAlviste/nvim-ts-context-commentstring"
  use "mfussenegger/nvim-ts-hint-textobject"
  -- }}}

  -- 🔱 Git: ⮯ {{{
  use 'tpope/vim-fugitive'
  use 'tpope/vim-rhubarb'
  use 'lewis6991/gitsigns.nvim'
  -- }}} ⮭

  -- 🔭 Fuzzy Finder: ⮯ {{{
  use { 'nvim-telescope/telescope.nvim', branch = '0.1.x', requires = { 'nvim-lua/plenary.nvim' } }
  use { 'nvim-telescope/telescope-fzf-native.nvim', run = 'make', cond = vim.fn.executable 'make' == 1 }
  -- }}} ⮭

  -- 👀 UI: ⮯ {{{
  use { "catppuccin/nvim", as = "catppuccin" }
  use 'nvim-lualine/lualine.nvim'
  use "kyazdani42/nvim-web-devicons"
  use "goolord/alpha-nvim"
  use "kyazdani42/nvim-tree.lua"
  use 'mbbill/undotree'
  use "ThePrimeagen/harpoon"
  -- }}} ⮭

  -- ✋ UX: ⮯ {{{
  use "ggandor/lightspeed.nvim"
  use "lewis6991/impatient.nvim"
  use 'numToStr/Comment.nvim'
  use "tpope/vim-surround"
  use "tpope/vim-repeat"
  use "mattn/emmet-vim"
  use "windwp/nvim-autopairs"
  use "AndrewRadev/switch.vim"
  use 'tpope/vim-sleuth'
  use 'cappyzawa/trim.nvim'
  use {
    "iamcco/markdown-preview.nvim",
    run = function() vim.fn["mkdp#util#install"]() end,
  }
-- }}} ⮭

-- ============================================================================
  -- 🧮 ZHU LI, DO THE THING! 🢢 {{{
  use 'wbthomason/packer.nvim'

  if is_bootstrap then
    require('packer').sync()
  end
end)

if is_bootstrap then
  print '=================================='
  print '    Plugins are being installed'
  print '    Wait until Packer completes,'
  print '       then restart nvim'
  print '=================================='
  return
end

-- Automatically source and re-compile packer whenever you save this init.lua
local packer_group = vim.api.nvim_create_augroup('Packer', { clear = true })
vim.api.nvim_create_autocmd('BufWritePost', {
  command = 'source <afile> | silent! LspStop | silent! LspStart | PackerSync | PackerCompile ',
  group = packer_group,
  pattern = vim.fn.expand '$MYVIMRC',
})

vim.api.nvim_create_autocmd('BufWritePost', {
  command = 'source <afile> | PackerSync',
  group = packer_group,
  pattern = vim.fn.expand '$MYVIMRC',
})

-- }}} ⮭
-- ============================================================================

local config_files = {
  "impatient", -- speeds up loading lua modules -> blazingly fast startup time.
  "setups", -- configuration and initialization of plugins
  "settings", -- edit default options/settings for neovim
  "keymaps", -- custom keymaps (some keymaps are defined in setups) ⮭
  "autocmds", -- custom automatic functions
}

-- 🧮 ZHU LI, DO THE THING!
for _, file in ipairs(config_files) do
  require(file)
end
