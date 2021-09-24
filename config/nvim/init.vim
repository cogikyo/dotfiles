" ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
" ╠═╝│  │ ││ ┬││││└─┐
" ╩  ┴─┘└─┘└─┘┴┘└┘└─┘
call plug#begin('~/.vim/plugged')

" Plug 'neovim/nvim-lspconfig'
" Plug 'nvim-lua/completion-nvim'

Plug 'nvim-lua/plenary.nvim'
Plug 'nvim-telescope/telescope.nvim'
Plug 'nvim-telescope/telescope-fzf-native.nvim', { 'do': 'make' }

Plug 'cocopon/iceberg.vim'
Plug 'itchyny/lightline.vim'
Plug 'nosvagor/iceberg-dark'
Plug 'chrisbra/Colorizer'
Plug 'kyazdani42/nvim-web-devicons'

Plug 'nvim-treesitter/nvim-treesitter', {'do': ':TSUpdate'}
Plug 'nvim-treesitter/playground'

Plug 'lervag/vimtex'
Plug 'KeitaNakamura/tex-conceal.vim', {'for': 'tex'}
Plug 'dpelle/vim-LanguageTool'

Plug 'SirVer/ultisnips'

" TODO: formatters

Plug 'tpope/vim-fugitive'
Plug 'tpope/vim-commentary'
Plug 'vim-syntastic/syntastic'
Plug 'folke/trouble.nvim'
Plug 'mbbill/undotree'
" Plug 'mattn/emmet-vim'
Plug 'folke/todo-comments.nvim'
Plug 'iamcco/markdown-preview.nvim', { 'do': 'cd app && yarn install'  }

call plug#end()
