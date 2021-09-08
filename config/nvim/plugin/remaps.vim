" ╦═╗┌─┐┌┬┐┌─┐┌─┐┌─┐
" ╠╦╝├┤ │││├─┤├─┘└─┐
" ╩╚═└─┘┴ ┴┴ ┴┴  └─┘
nnoremap <SPACE> <Nop>
let mapleader = " "

" 🦬 buffers
nnoremap <leader>w :bp<CR>
nnoremap <leader>v :bn<CR>
nnoremap <leader>dd :bd!<CR>

" 🤖 copy copy
nnoremap <leader>y "+y
vnoremap <leader>y "+y
vnoremap <leader>Y gg"+yG

" 🍉 terminal
tnoremap <C-t> <C-\><C-n>
nnoremap <leader>a :term<CR>i

" 🦘 jump list mutations
nnoremap <expr> j (v:count > 5? "m" . v:count : "") . 'j'
nnoremap <expr> k (v:count > 5? "u" . v:count : "") . 'k'

" 🎯 keep cursor 'centered'
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

" 🪟 window movement
nnoremap <Down> <C-w>j
nnoremap <Up> <C-w>k

" 🔭 telescope
nnoremap <leader>tf <cmd>Telescope find_files<cr>
nnoremap <leader>tt <cmd>Telescope live_grep<cr>
nnoremap <leader>tg <cmd>Telescope buffers<cr>
nnoremap <leader>td <cmd>Telescope help_tags<cr>

" 🤲 toggle
nnoremap <leader>ut :UndotreeToggle<CR>
nnoremap <leader>ct :ColorToggle<CR>
nnoremap <leader>mp :MarkdownPreviewToggle<CR>

" 🔤 spelling
inoremap <C-l> <c-g>u<Esc>[s1z=`]a<c-g>u
nnoremap <C-l> [s1z=<c-o>
vnoremap <leader>lt :LanguageToolCheck<CR>
nnoremap <leader>lt :LanguageToolClear<CR>
