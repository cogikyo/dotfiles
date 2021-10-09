" ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
" ╠═╝│  │ ││ ┬││││└─┐
" ╩  ┴─┘└─┘└─┘┴┘└┘└─┘
call plug#begin('~/.vim/plugged')

Plug 'nvim-lua/plenary.nvim'
Plug 'nvim-telescope/telescope.nvim'
Plug 'nvim-telescope/telescope-fzf-native.nvim', { 'do': 'make' }

Plug 'cocopon/iceberg.vim'
Plug 'itchyny/lightline.vim'
Plug 'nosvagor/iceberg-dark'
Plug 'chrisbra/Colorizer'
Plug 'kyazdani42/nvim-web-devicons'

Plug 'lervag/vimtex'
Plug 'KeitaNakamura/tex-conceal.vim', {'for': 'tex'}
Plug 'SirVer/ultisnips'

Plug 'easymotion/vim-easymotion'
Plug 'yuttie/comfortable-motion.vim'
Plug 'tpope/vim-commentary'
Plug 'tpope/vim-fugitive'
Plug 'vim-syntastic/syntastic'
Plug 'folke/trouble.nvim'
Plug 'mbbill/undotree'
Plug 'iamcco/markdown-preview.nvim', { 'do': 'cd app && yarn install'  }

call plug#end()

