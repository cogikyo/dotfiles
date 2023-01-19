-- ============================================================================
-- 📦 Prepwork (auto install packer) 🢢 {{{
local install_path = vim.fn.stdpath 'data' .. '/site/pack/packer/start/packer.nvim'
local is_bootstrap = false
if vim.fn.empty(vim.fn.glob(install_path)) > 0 then
  is_bootstrap = true
  vim.fn.system { 'git', 'clone', '--depth', '1', 'https://github.com/wbthomason/packer.nvim', install_path }
  vim.cmd [[packadd packer.nvim]]
end

require('packer').startup(function(use)
-- }}}
-- ============================================================================

  -- 📚 LSP: {{{
  use {
    'neovim/nvim-lspconfig',
    requires = {
      -- Automatically install LSPs to stdpath for neovim
      'williamboman/mason.nvim',
      'williamboman/mason-lspconfig.nvim',

      -- Useful status updates for LSP
      'j-hui/fidget.nvim',

      'folke/neodev.nvim',
      -- Additional lua configuration, makes nvim stuff amazing
    },
  }
  -- }}}

  -- 🪄 Completion: {{{
  use {
    'hrsh7th/nvim-cmp',
    requires = {
      'hrsh7th/cmp-nvim-lsp',
      'L3MON4D3/LuaSnip',
      'saadparwaiz1/cmp_luasnip'
    },
  }
  -- }}}

  -- 🌲 Treesitter: {{{
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
  use 'nvim-lualine/lualine.nvim' -- Fancier statusline
  use { "catppuccin/nvim", as = "catppuccin" }
  -- }}} ⮭

  -- ✋ UX: ⮯ {{{
  use "lewis6991/impatient.nvim"
  use {
    "iamcco/markdown-preview.nvim",
    run = function() vim.fn["mkdp#util#install"]() end,
  }
  use 'numToStr/Comment.nvim' -- "gc" to comment visual regions/lines
  use "tpope/vim-surround"
  use "tpope/vim-repeat"
  use "windwp/nvim-autopairs"
  use "AndrewRadev/switch.vim"
  use 'tpope/vim-sleuth' -- Detect tabstop and shiftwidth automatically
  use 'mbbill/undotree'
  use "ggandor/lightspeed.nvim"
  use "mattn/emmet-vim"
  use "ThePrimeagen/harpoon"
  use {
    'cappyzawa/trim.nvim',
    config = function()
      require('trim').setup({
        disable = {},
        patterns = {
          -- replace multiple blank lines with a single line ⮯
          [[%s/\(\n\n\n\)\n\+/\1/]],
        },
      })
    end
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
  command = 'source <afile> | silent! LspStop | silent! LspStart | PackerCompile',
  group = packer_group,
  pattern = vim.fn.expand '$MYVIMRC',
})
-- }}} ⮭
-- ============================================================================


local config_files = {
  -- ============================================================================
  "settings",
  "keymaps",
  "autocmds",
  "simple-setup",
  -- "treesitter",
  -- "lsp",
  -- "cmp",
  -- "telescope",
  -- "colorscheme",
  -- "autopairs",
  -- "comment",
  -- "gitsigns",
  -- "nvim-tree",
  -- "toggleterm",
  -- "lualine",
  -- "alpha",
}

for _, file in ipairs(config_files) do
  require(file)
end
