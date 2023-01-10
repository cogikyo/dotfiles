-- ============================================================================
-- 🧰 Packer Setup -- {{{

-- Automatically install packer
local install_path = vim.fn.stdpath("data") .. "/site/pack/packer/start/packer.nvim"
if vim.fn.empty(vim.fn.glob(install_path)) > 0 then
    PACKER_BOOTSTRAP = vim.fn.system({
        "git",
        "clone",
        "--depth",
        "1",
        "https://github.com/wbthomason/packer.nvim",
        install_path,
    })
    print("Installing packer close and reopen Neovim...")
    vim.cmd([[packadd packer.nvim]])
end

-- Autocommand that reloads neovim whenever you save the plugins.lua file
vim.cmd([[
    augroup packer_user_config
    autocmd!
    autocmd BufWritePost plugins.lua source <afile> | PackerSync
    autocmd BufWinLeave plugins.lua mkview
    autocmd BufWinEnter plugins.lua loadview
    augroup end
]])
-- Automatically source and re-compile packer whenever you save this init.lua
local packer_group = vim.api.nvim_create_augroup('Packer', { clear = true })
vim.api.nvim_create_autocmd('BufWritePost', {
    command = 'source <afile> | silent! LspStop | silent! LspStart | PackerCompile',
    group = packer_group,
    pattern = vim.fn.expand '$MYVIMRC',
})

-- Use a protected call so we don"t error out on first use
local status_ok, packer = pcall(require, "packer")
if not status_ok then
    return
end

-- Have packer use a pop up window
packer.init({
    display = {
        open_fn = function()
            return require("packer.util").float({ border = "rounded" })
        end,
    },
})

-- }}}
-- ============================================================================

return packer.startup(function(use)
    -- 🔧 Utility ⮯ {{{
    use "wbthomason/packer.nvim"
    use "lewis6991/impatient.nvim"
    use "nvim-lua/popup.nvim"
    use "nvim-lua/plenary.nvim"
    use {
        'folke/neodev.nvim',
        config = function()
            require('neodev').setup({})
        end
    }
    -- }}}

    -- 📚 LSP ⮯ {{{
    use "neovim/nvim-lspconfig"
    use {
        'williamboman/mason.nvim',
        config = function()
            require("mason").setup({
                ui = {
                    icons = {
                        package_installed = "✓",
                        package_pending = "➜",
                        package_uninstalled = "✗"
                    }
                }
            })
        end,
    }
    use {
        'williamboman/mason-lspconfig.nvim',
        config = function()
            require("mason-nvim-dap").setup({
                automatic_setup = true,
            })
        end,
    }
    use 'j-hui/fidget.nvim'
    use 'mfussenegger/nvim-dap'
    use 'jayp0521/mason-nvim-dap.nvim'
    use 'mfussenegger/nvim-lint'
    use 'mhartington/formatter.nvim'
    -- }}}

    -- 🪄 Completion ⮯ {{{
    use "hrsh7th/nvim-cmp"
    use "hrsh7th/cmp-nvim-lsp"
    use "hrsh7th/cmp-nvim-lua"
    use "hrsh7th/cmp-buffer"
    use "hrsh7th/cmp-path"
    use "hrsh7th/cmp-cmdline"
    use "hrsh7th/cmp-calc"
    use "hrsh7th/cmp-emoji"
    use "max397574/cmp-greek"
    use "saadparwaiz1/cmp_luasnip"
    use "L3MON4D3/LuaSnip"
    use "rafamadriz/friendly-snippets"
    -- }}}

    -- 🌲 Treesitter ⮯ {{{
    use {
        "nvim-treesitter/nvim-treesitter",
        run = function()
            pcall(require('nvim-treesitter.install').update { with_sync = true })
        end
    }
    use {
        'nvim-treesitter/nvim-treesitter-textobjects',
        after = 'nvim-treesitter'
    }
    use "nvim-treesitter/playground"
    use "JoosepAlviste/nvim-ts-context-commentstring"
    use "mfussenegger/nvim-ts-hint-textobject"
    -- }}}

    -- 👀 UI ⮯ {{{
    use "folke/tokyonight.nvim"
    use {
        "kyazdani42/nvim-web-devicons",
        config = function()
            require("nvim-web-devicons").setup({
                default = true,
            })
        end,
    }
    use {
        "norcalli/nvim-colorizer.lua",
        config = function()
            require("colorizer").setup()
        end,
    }
    use {
        "lewis6991/gitsigns.nvim",
        tag = "release",
    }
    use "nvim-lualine/lualine.nvim"
    -- }}}

    -- ✋ UX ⮯ {{{
    use 'tpope/vim-sleuth' -- detect tabstop and shiftwidth automatically
    use "numToStr/Comment.nvim"
    use "ThePrimeagen/harpoon"
    use 'nvim-telescope/telescope.nvim'
    use "natecraddock/telescope-zf-native.nvim"
    use "mattn/emmet-vim"
    use "windwp/nvim-autopairs"
    use "kyazdani42/nvim-tree.lua"
    use "AndrewRadev/switch.vim"
    use "akinsho/toggleterm.nvim"
    use "goolord/alpha-nvim"
    use {
        "iamcco/markdown-preview.nvim",
        run = function() vim.fn["mkdp#util#install"]() end,
    }
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
        end,
    }
    use "tpope/vim-repeat"
    use "tpope/vim-surround"
    use "mbbill/undotree"
    use "ggandor/lightspeed.nvim"
    use "elkowar/yuck.vim"
    use "amadeus/vim-convert-color-to"
    use "dhruvasagar/vim-table-mode"
    -- }}}

    -- Automatically set up configuration after cloning packer.nvim
    if PACKER_BOOTSTRAP then
        require("packer").sync()
    end
end)
