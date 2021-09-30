" ╦═╗┌─┐┌┬┐┌─┐┌─┐┌─┐
" ╠╦╝├┤ │││├─┤├─┘└─┐
" ╩╚═└─┘┴ ┴┴ ┴┴  └─┘
nnoremap <SPACE> <Nop>
let mapleader = " "

" temp:, : needs to be on base layer of keyboard
nnoremap <leader>s :w<CR>
nnoremap <leader>x :q<CR>

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

" 🌌 gimmie space please
nnoremap <Leader>o o<Esc>"_Dk$
nnoremap <Leader>O O<Esc>"_Dj$
inoremap <C-Space> <Esc>o<Esc>"_DkO<Esc>_DjA

" 🦘 jump list mutations
nnoremap <expr> j (v:count > 5 ? "m`" . v:count : "") . 'j'
nnoremap <expr> k (v:count > 5 ? "m`" . v:count : "") . 'k'

" 🎯 keep cursor 'centered'
nnoremap } }zt
nnoremap { {zt
nnoremap G Gzt
nnoremap n nztzv
nnoremap N Nztzv
nnoremap J mzJ`z
nnoremap <C-d> <C-d>zt
nnoremap <C-u> <C-u>zt

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
nnoremap <C-Left> <C-w>h
nnoremap <C-Right> <C-w>l

" 🔭 telescope
nnoremap <leader>tf <cmd>Telescope find_files<cr>
nnoremap <leader>tg <cmd>Telescope live_grep<cr>
nnoremap <leader>tt <cmd>Telescope buffers<cr>
nnoremap <leader>td <cmd>Telescope help_tags<cr>
nnoremap <leader>te <cmd>Telescope git_files<cr>
nnoremap <leader>tm <cmd>Telescope man_pages<cr>
nnoremap <leader>tx <cmd>Telescope man_pages<cr>

" 🤲 toggle
nnoremap <leader>ut :UndotreeToggle<CR>
nnoremap <leader>ct :ColorToggle<CR>
nnoremap <leader>mp :MarkdownPreviewToggle<CR>
nnoremap <leader>ei :TroubleToggle<CR>

" 🔤 spelling
inoremap <C-l> <c-g>u<Esc>[s1z=`]a<c-g>u
nnoremap <C-l> [s1z=<c-o>
vnoremap <leader>lt :LanguageToolCheck<CR>
nnoremap <leader>lt :LanguageToolClear<CR>

" 📓 latex
inoremap <C-y> <CR>\item<Space>
nnoremap <C-y> A<CR>\item<Space>
nnoremap <leader>ni /\\item<CR>zt
nnoremap <leader>ny ?\\item<CR>zt
nnoremap <leader>na /\\item<CR><esc>0ztA
inoremap <C-f> <Esc>: silent exec '.!inkscape-figures create "'.getline('.').'" "'.b:vimtex.root.'/figures/"'<CR><CR>:w<CR>
nnoremap <C-f> : silent exec '!inkscape-figures edit "'.b:vimtex.root.'/figures/" > /dev/null 2>&1 &'<CR><CR>:redraw!<CR>

" 🐍 python
nnoremap <buffer> <leader>rr :w<CR>:exec '!python3' shellescape(@%, 1)<CR>

" 🔃 reload
nnoremap <leader>rs :call UltiSnips#RefreshSnippets()<CR>
nnoremap <leader>rm :source $HOME/.config/nvim/plugin/remaps.vim<CR>

" ⛵ reflow
inoremap <C-h> <esc>VgwzHzzA
nnoremap <C-h> VgwzHzzg_
