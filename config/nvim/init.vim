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
" Plug 'nvim-treesitter/nvim-treesitter', {'do': ':TSUpdate'}
" Plug 'nvim-treesitter/playground'

" latex
Plug 'lervag/vimtex'

" utility
Plug 'tpope/vim-commentary'
Plug 'vim-utils/vim-man'
Plug 'tpope/vim-fugitive'
Plug 'mbbill/undotree'
Plug 'mattn/emmet-vim'
Plug 'folke/todo-comments.nvim'
Plug 'folke/trouble.nvim'

" snippets
Plug 'SirVer/ultisnips'

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
let g:UltiSnipsJumpForwardTrigger="<c-b>" " default
let g:UltiSnipsJumpBackwardTrigger="<c-z>" " default

" todo / trouble
lua << EOF
require("todo-comments").setup {
    colors = {
        error = { "LspDiagnosticsDefaultError", "ErrorMsg", "#e98989" },
        warning = { "LspDiagnosticsDefaultWarning", "WarningMsg", "#e9b189" },
        info = { "LspDiagnosticsDefaultInformation", "#91acd1" },
        hint = { "LspDiagnosticsDefaultHint", "#c0ca8e" },
        default = { "Identifier", "#ada0d3" },
    },
}
require("trouble").setup {}
EOF

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

" 🦬 buffers
nnoremap <leader>w :bp<CR>
nnoremap <leader>v :bn<CR>
nnoremap <leader>bd :bd!<CR>

" 🤖 copy copy
nnoremap <leader>y "+y
vnoremap <Leader>y "+y
vnoremap <leader>Y gg"+yG

" 🚮 no reg delete
xnoremap <leader>p "_dP
nnoremap <leader>d "_d
vnoremap <leader>d "_d
" 🍉 terminal
tnoremap <C-y> <C-\><C-n>
nnoremap <leader>a :term<CR>i

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

" 🔭 telescope
nnoremap <leader>nn <cmd>Telescope find_files<cr>
nnoremap <leader>ng <cmd>Telescope live_grep<cr>
nnoremap <leader>ni <cmd>Telescope buffers<cr>
nnoremap <leader>nh <cmd>Telescope help_tags<cr>
