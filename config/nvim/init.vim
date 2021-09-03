" ╔═╗┌─┐┌┬┐┌─┐
" ╚═╗├┤  │ └─┐
" ╚═╝└─┘ ┴ └─┘
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
set scrolloff=16
set signcolumn=yes
set colorcolumn=80
set nowrap


" ╔═╗┬  ┬ ┬┌─┐┬┌┐┌┌─┐
" ╠═╝│  │ ││ ┬││││└─┐
" ╩  ┴─┘└─┘└─┘┴┘└┘└─┘
call plug#begin('~/.vim/plugged')
Plug 'neovim/nvim-lspconfig'
Plug 'nvim-lua/completion-nvim'
Plug 'nvim-lua/plenary.nvim'
Plug 'nvim-telescope/telescope.nvim'
Plug 'cocopon/iceberg.vim'
Plug 'chrisbra/Colorizer'
Plug 'tpope/vim-commentary'
Plug 'nosvagor/iceberg-dark'
Plug 'itchyny/lightline.vim'
Plug 'tpope/vim-fugitive'
Plug 'mattn/emmet-vim'
Plug 'L3MON4D3/LuaSnip'
call plug#end()


" ╔╦╗┬ ┬┌─┐┌┬┐┌─┐
"  ║ ├─┤├┤ │││├┤
"  ╩ ┴ ┴└─┘┴ ┴└─┘
colorscheme iceberg
let g:colorizer_auto_filetype='vim,css,scss,html,tex,txt,cls,sty,rasi,conf'
let g:colorizer_colornames = 0
let g:lightline = {
\ 'separator': { 'left': '', 'right': '' },
\ 'subseparator': { 'left': '╼╾', 'right': '╼╾'},
\ 'colorscheme': 'icebergDark',
\ 'active': {
\   'left': [ [ 'mode', 'paste' ],
\             [ 'fugitive', 'readonly', 'filename', 'modified' ] ],
\   'right': [ [ 'lineinfo' ],
\              [ 'percent' ],
\              [ 'bufnum', ] ]
\ },
\ 'component_function': {
\   'fugitive': 'LightlineFugitive',
\   'filename': 'LightlineFilename',
\   'readonly': 'LightlineReadonly',
\   'modified': 'LightlineModified',
\ },
\ }

" this is fun
let g:lightline.mode_map = {
\ 'n' : '正常',
\ 'i' : '入れる',
\ 'R' : '交換',
\ 'v' : '見る',
\ 'V' : '行-見る',
\ "\<C-v>": '区画-見る',
\ 'c' : '令',
\ 's' : '撰ぶ',
\ 'S' : '行-選ぶ',
\ "\<C-s>": '区画-撰ぶ',
\ 't': 'ターミナル',
\ }

function! LightlineFugitive()
    try
	    if expand('%:t') !~? 'Tagbar\|Gundo\|NERD' && &ft !~? 'vimfiler' && exists('*FugitiveHead')
	        let mark = ' '   " edit here for cool mark
            let branch = FugitiveHead()
	        return branch !=# '' ? mark.branch : ''
	    endif
    catch
    endtry
    return ''
endfunction

function! LightlineFilename()
	return &ft ==# 'vimfiler' ? vimfiler#get_status_string() :
	      \  &ft ==# 'unite' ? unite#get_status_string() :
	      \ expand('%:t') !=# '' ? expand('%:t') : '名前?'
endfunction

function! LightlineReadonly()
	return &ft !~? 'help\|vimfiler' && &readonly ? '' : ''
endfunction

function! LightlineModified()
	return &modifiable && &modified ? '' : ''
endfunction

function! SyntasticCheckHook(errors)
    call lightline#update()
endfunction


" ╔═╗┬  ┬ ┬┌─┐╦═╗╔═╗
" ╠═╝│  │ ││ ┬╠╦╝║
" ╩  ┴─┘└─┘└─┘╩╚═╚═╝
" emmet
let g:user_emmet_install_global = 0
autocmd FileType html,css EmmetInstall


" ╦  ╔═╗╔═╗┌─┐┌─┐┌┐┌┌─┐┬┌─┐
" ║  ╚═╗╠═╝│  │ ││││├┤ ││ ┬
" ╩═╝╚═╝╩  └─┘└─┘┘└┘└  ┴└─┘
" enable servers
lua << EOF
--python
require'lspconfig'.pyright.setup{on_attach=require'completion'.on_attach}

--html/css
local capabilities = vim.lsp.protocol.make_client_capabilities()
capabilities.textDocument.completion.completionItem.snippetSupport = true
require'lspconfig'.html.setup {
    capabilities = capabilities,
    on_attach=require'completion'.on_attach
  }
require'lspconfig'.cssls.setup {
    capabilities = capabilities,
    on_attach=require'completion'.on_attach
}

--vim
require'lspconfig'.vimls.setup{on_attach=require'completion'.on_attach}

--typescript/javascript
require'lspconfig'.tsserver.setup{on_attach=require'completion'.on_attach}

--tex
require'lspconfig'.texlab.setup{on_attach=require'completion'.on_attach}

--bash
require'lspconfig'.bashls.setup{on_attach=require'completion'.on_attach}
EOF


" autocompletion
set completeopt=menuone,noinsert,noselect
set shortmess+=c

let g:completion_matching_strategy_list = ['exact','substring', 'fuzzy']
let g:completion_trigger_keyword_length = 3 " default = 1


" ╔═╗┌┬┐┌┬┐┌─┐
" ║  │││ ││└─┐
" ╚═╝┴ ┴─┴┘└─┘
" auto source config on change
" if has ('autocmd') " Remain compatible with earlier versions
"  augroup vimrc     " Source vim configuration upon save
"     autocmd! BufWritePost $MYVIMRC source % | echom "Reloaded " . $MYVIMRC | redraw
"     autocmd! BufWritePost $MYGVIMRC if has('gui_running') | so % | echom "Reloaded " . $MYGVIMRC | endif | redraw
"   augroup END
" endif " has autocmd

" auto remove trailing whitespace on save
autocmd BufWritePre * :%s/\s\+$//e


" ╦═╗┌─┐┌┬┐┌─┐┌─┐┌─┐
" ╠╦╝├┤ │││├─┤├─┘└─┐
" ╩╚═└─┘┴ ┴┴ ┴┴  └─┘
nnoremap <SPACE> <Nop>
let mapleader = " "

nnoremap <C-s> :w<CR>

" copy to end of line
nnoremap Y y$

" keep cursor centered
nnoremap n nzzzv
nnoremap N Nzzzv
nnoremap J mzJ`z

" undo break points
inoremap , ,<c-g>u
inoremap . .<c-g>u
inoremap ! !<c-g>u
inoremap ? ?<c-g>u
inoremap ; ;<c-g>u
inoremap : :<c-g>u

" jumplist mutations
nnoremap <expr> k (v:count > 5? "m'" . v:count : "") . 'k'
nnoremap <expr> j (v:count > 5? "m'" . v:count : "") . 'j'

" moving text
vnoremap J :m '>+1<CR>gv=gv
vnoremap K :m '<-2<CR>gv=gv
inoremap <C-j> <esc> :m .+1<CR>==
inoremap <C-k> <esc> :m .-2<CR>==
nnoremap <leader>j :m .+1<CR>==
nnoremap <leader>k :m .-2<CR>==
