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

Plug 'mattn/emmet-vim'
Plug 'easymotion/vim-easymotion'
Plug 'yuttie/comfortable-motion.vim'
Plug 'tpope/vim-commentary'
Plug 'mhinz/vim-signify'
Plug 'tpope/vim-fugitive'
Plug 'tpope/vim-surround'
Plug 'jiangmiao/auto-pairs'
Plug 'vim-syntastic/syntastic'
Plug 'folke/trouble.nvim'
Plug 'mbbill/undotree'
Plug 'iamcco/markdown-preview.nvim', { 'do': 'cd app && yarn install'  }
Plug 'Chiel92/vim-autoformat'
Plug 'junegunn/vim-easy-align'
Plug 'mhinz/vim-startify'

Plug 'ncm2/ncm2'
Plug 'roxma/nvim-yarp'
Plug 'ncm2/ncm2-bufword'
Plug 'ncm2/ncm2-path'
Plug 'ncm2/ncm2-go'
Plug 'ncm2/ncm2-ultisnips'
Plug 'ncm2/ncm2-jedi'
Plug 'fisadev/vim-isort'
Plug 'jpalardy/vim-slime'

call plug#end()
