set exrc
set relativenumber
set nu
set nohlsearch
set hidden
set noerrorbells
set termguicolors
set tabstop=4 softtabstop=4
set shiftwidth=4
set expandtab
set smartindent
set noswapfile
set nobackup
set undodir=~/.vim/undodir
set undofile
set incsearch
set scrolloff=9
set signcolumn=yes
set colorcolumn=80
set nowrap

call plug#begin('~/.vim/plugged')
Plug 'nvim-lua/plenary.nvim'
Plug 'nvim-telescope/telescope.nvim'
Plug 'cocopon/iceberg.vim'
Plug 'chrisbra/Colorizer'
Plug 'tpope/vim-commentary'
Plug 'vim-airline/vim-airline'
Plug 'gkeep/iceberg-dark'
call plug#end()

colorscheme iceberg
let g:colorizer_auto_filetype='css,scss,html,tex,txt,cls'
let g:colorizer_colornames = 0
let g:airline_theme='icebergDark'

if has ('autocmd') " Remain compatible with earlier versions
 augroup vimrc     " Source vim configuration upon save
    autocmd! BufWritePost $MYVIMRC source % | echom "Reloaded " . $MYVIMRC | redraw
    autocmd! BufWritePost $MYGVIMRC if has('gui_running') | so % | echom "Reloaded " . $MYGVIMRC | endif | redraw
  augroup END
endif " has autocmd


