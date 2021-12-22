" ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
" ╠═╝│  │ ││ ┬││││└─┐
" ╩  ┴─┘└─┘└─┘┴┘└┘└─┘
call plug#begin('~/.vim/plugged')

Plug 'nvim-treesitter/nvim-treesitter'
Plug 'nvim-lua/plenary.nvim'
Plug 'nvim-telescope/telescope.nvim'
Plug 'nvim-telescope/telescope-fzf-native.nvim', { 'do': 'make' }
Plug 'ThePrimeagen/harpoon'

Plug 'cocopon/iceberg.vim'
Plug 'itchyny/lightline.vim'
Plug 'chrisbra/Colorizer'
Plug 'kyazdani42/nvim-web-devicons'

Plug 'lervag/vimtex'
Plug 'SirVer/ultisnips'

Plug 'easymotion/vim-easymotion'
Plug 'yuttie/comfortable-motion.vim'
Plug 'numToStr/Comment.nvim'
Plug 'mhinz/vim-signify'
Plug 'tpope/vim-fugitive'
Plug 'tpope/vim-surround'
Plug 'jiangmiao/auto-pairs'
Plug 'vim-syntastic/syntastic'
Plug 'folke/trouble.nvim'
Plug 'mbbill/undotree'
Plug 'iamcco/markdown-preview.nvim', { 'do': 'cd app && yarn install'  }
Plug 'mhinz/vim-startify'

Plug 'ncm2/ncm2'
Plug 'roxma/nvim-yarp'
Plug 'ncm2/ncm2-bufword'
Plug 'tpope/vim-commentary'
Plug 'ncm2/ncm2-path'
Plug 'ncm2/ncm2-go'
Plug 'ncm2/ncm2-ultisnips'
Plug 'ncm2/ncm2-match-highlight'

Plug 'ncm2/ncm2-jedi'
Plug 'averms/black-nvim', {'do': ':UpdateRemotePlugins'}
Plug 'fisadev/vim-isort'

call plug#end()

lua << EOF
require('Comment').setup()
EOF
