" ╔═╗┌─┐┌┬┐┌─┐
" ╚═╗├┤  │ └─┐
" ╚═╝└─┘ ┴ └─┘
" 📼 vim basics
set relativenumber
set nu
set nohlsearch
set hidden
set noerrorbells
set termguicolors
set tabstop=2 softtabstop=2
set shiftwidth=2
set expandtab
set smartindent
set noswapfile
set nobackup
set undodir=~/.vim/undodir
set undofile
set incsearch
set scrolloff=19
set signcolumn=yes
set colorcolumn=80
set nowrap
set splitbelow
set laststatus=2

set statusline+=%#warningmsg#
set statusline+=%{SyntasticStatuslineFlag()}
set statusline+=%*

set conceallevel=2

set completeopt=menuone,noselect,noinsert
set shortmess+=c
