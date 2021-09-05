" ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
" ╠═╝│  │ ││ ┬││││└─┐
" ╩  ┴─┘└─┘└─┘┴┘└┘└─┘
call plug#begin('~/.vim/plugged')
" lsp
Plug 'nvim-lua/plenary.nvim'
Plug 'neovim/nvim-lspconfig'
Plug 'nvim-lua/completion-nvim'

" telescope
Plug 'nvim-telescope/telescope.nvim'
Plug 'nvim-telescope/telescope-fzf-native.nvim', { 'do': 'make' }

" themeing
Plug 'cocopon/iceberg.vim'
Plug 'itchyny/lightline.vim'
Plug 'nosvagor/iceberg-dark'
Plug 'chrisbra/Colorizer'
Plug 'kyazdani42/nvim-web-devicons'

" treesitter
Plug 'nvim-treesitter/nvim-treesitter', {'do': ':TSUpdate'}
Plug 'nvim-treesitter/playground'

" utility
Plug 'tpope/vim-commentary'
Plug 'vim-utils/vim-man'
Plug 'tpope/vim-fugitive'
Plug 'mbbill/undotree'
Plug 'mattn/emmet-vim'

" snippets
Plug 'SirVer/ultisnips'
Plug 'honza/vim-snippets'

call plug#end()


" ╔═╗┬  ┬ ┬┌─┐╦═╗╔═╗
" ╠═╝│  │ ││ ┬╠╦╝║
" ╩  ┴─┘└─┘└─┘╩╚═╚═╝
" emmet
let g:user_emmet_install_global = 0

" undotree
nnoremap <leader>ut :UndotreeToggle<CR>

" snippets
let g:UltiSnipsExpandTrigger="<c-l>"
let g:UltiSnipsJumpForwardTrigger="<c-b>"
let g:UltiSnipsJumpBackwardTrigger="<c-z>"

" ╔═╗┌┬┐┌┬┐┌─┐
" ║  │││ ││└─┐
" ╚═╝┴ ┴─┴┘└─┘
augroup AUTO_COMMANDS
    autocmd!
    autocmd BufWritePre * :call TrimWhitespace()
    autocmd FileType html,css EmmetInstall
    autocmd BufEnter * lua require'completion'.on_attach()
    autocmd TermOpen * setlocal nonumber norelativenumber signcolumn=no
augroup END

augroup highlight_yank
    autocmd!
    autocmd TextYankPost * silent! lua require'vim.highlight'.on_yank({timeout = 69})
augroup END

" ╦═╗┌─┐┌┬┐┌─┐┌─┐┌─┐
" ╠╦╝├┤ │││├─┤├─┘└─┐
" ╩╚═└─┘┴ ┴┴ ┴┴  └─┘
nnoremap <SPACE> <Nop>
let mapleader = " "

" 🕶️  switch buffers
nnoremap <leader>w :bp<CR>
nnoremap <leader>v :bn<CR>

" 💾 cant stop wont stop
nnoremap <C-s> :w<CR>

" 🤖 copy copy
nnoremap <leader>y "+y
vnoremap <Leader>y "+y
vnoremap <leader>Y gg"+yG

" 🚮 no reg delete
xnoremap <leader>p "_dP
nnoremap <leader>d "_d
vnoremap <leader>d "_d

" 🍉 terminal
tnoremap <C-y> <C-\><C-n>i
nnoremap <leader>a :term<CR>

" 🦘 jumplist mutations
nnoremap <expr> k (v:count > 5? "u" . v:count : "") . 'k'
nnoremap <expr> j (v:count > 5? "m" . v:count : "") . 'j'

" 🎯 keep cursor 'centered'
nnoremap j jzt
nnoremap k kzt
nnoremap } }zt
nnoremap { {zt
nnoremap G Gzt
nnoremap n nztzv
nnoremap N Nztzv
nnoremap J mzJ`z

" 👈 undo break points
inoremap , ,<c-g>u
inoremap . .<c-g>u
inoremap ! !<c-g>u
inoremap ? ?<c-g>u
inoremap ; ;<c-g>u
inoremap : :<c-g>u

" 🚙 moving text
vnoremap J :m '>+1<CR>gv=gv
vnoremap K :m '<-2<CR>gv=gv
inoremap <C-j> <esc> :m .+1<CR>==
inoremap <C-k> <esc> :m .-2<CR>==
nnoremap <leader>j :m .+1<CR>==
nnoremap <leader>k :m .-2<CR>==
