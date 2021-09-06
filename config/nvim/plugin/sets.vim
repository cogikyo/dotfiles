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

setlocal spell
set spelllang=en_us
set spelloptions=camel

"❗syntastic-sets
set statusline+=%#warningmsg#
set statusline+=%{SyntasticStatuslineFlag()}
set statusline+=%*

"⚙  utility-sets
set conceallevel=2

"💨 completion
set completeopt=menuone,noselect,noinsert
set shortmess+=c
