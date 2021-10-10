" ╦═╗┌─┐┌┬┐┌─┐┌─┐┌─┐
" ╠╦╝├┤ │││├─┤├─┘└─┐
" ╩╚═└─┘┴ ┴┴ ┴┴  └─┘
nnoremap <SPACE> <Nop>
let mapleader = " "

" ⚕  esc
inoremap <C-c> <esc>
inoremap <C-s> <esc>
nnoremap <C-s> <esc>
vnoremap <C-s> <esc>

" 💨 completion
inoremap <expr> <CR> (pumvisible() ? "\<c-y>\<cr>" : "\<CR>")
inoremap <silent> <expr> <CR> ncm2_ultisnips#expand_or("\<CR>", 'n')

" 🔍 go to letter(s)
nmap t <Plug>(easymotion-bd-f)
nmap T <Plug>(easymotion-bd-f2)

" temp:, : needs to be on base layer of keyboard
nnoremap <leader>s :w<CR>
nnoremap <leader>x :q<CR>
nnoremap <leader>q :wq<CR>

" 🦬 buffers
nnoremap <leader>w :bp<CR>
nnoremap <leader>v :bn<CR>
nnoremap <leader>dd :bd!<CR>

" 🤖 copy copy
nnoremap <leader>y "+y
vnoremap <leader>y "+y
vnoremap <leader>Y gg"+yG
nmap gy `]
vnoremap <leader>p y']o<Esc>p

" 🤯 comment headers
nnoremap <leader>hs :.! figlet -fshadow<CR>O<Esc>j:.,.+5Commentary<CR>5jo<Esc>d0
nnoremap <leader>hc :.! figlet -fcalvin<CR>O<Esc>j:.,.+2Commentary<CR>2jo<Esc>d0

" 🌌 gimmie space please
nnoremap <leader>o o<Esc>"_Dk$
nnoremap <leader>O O<Esc>"_Dj$
inoremap <C-Space> <Esc>o<Esc>"_DkO<Esc>_DjA

" 🎯 keep cursor 'centered'
nnoremap } }zt
nnoremap { {zt
nnoremap G Gzt
nnoremap n nztzv
nnoremap N Nztzv
nnoremap <C-j> mzJ`z

" 👈 undo break points
inoremap , ,<c-g>u
inoremap . .<c-g>u
inoremap ! !<c-g>u
inoremap ? ?<c-g>u
inoremap ; ;<c-g>u
inoremap : :<c-g>u

" 🚙 moving text
vnoremap u :m '<-2<CR>gv=gv
vnoremap m :m '>+1<cr>gv=gv
inoremap <C-k> <esc> :m .-2<CR>==i
inoremap <C-j> <esc> :m .+1<CR>==i
nnoremap <leader>uu :m .-2<CR>==
nnoremap <leader>mm :m .+1<CR>==

" 🍉 terminal
tnoremap <C-t> <C-\><C-n>:bd!<CR>
nnoremap <leader>tp :term<CR>isl<CR>niol<CR>
nnoremap <leader>ta :term<CR>i

" 🪟 window movement
nnoremap <Down> <C-w>j
nnoremap <Up> <C-w>k
nnoremap <Left> <C-w>h
nnoremap <Right> <C-w>l

" 🔭 telescope
nnoremap <leader>tf <cmd>Telescope buffers<cr>
nnoremap <leader>tg <cmd>Telescope live_grep<cr>
nnoremap <leader>tt <cmd>Telescope find_files<cr>
nnoremap <leader>th <cmd>Telescope help_tags<cr>
nnoremap <leader>te <cmd>Telescope git_files<cr>
nnoremap <leader>tm <cmd>Telescope man_pages<cr>
nnoremap <leader>tr <cmd>Telescope commands<cr>

" 🤲 toggle
nnoremap <leader>ut :UndotreeToggle<CR>
nnoremap <leader>ct :ColorToggle<CR>
nnoremap <leader>mp :MarkdownPreviewToggle<CR>
nnoremap <silent> <leader>ts :call ToggleSpellCheck()<CR>

" 🔤 spelling
inoremap <C-l> <c-g>u<Esc>[s1z=`]a<c-g>u
nnoremap <C-l> [s1z=<c-o>

" 📓 latex
inoremap <C-t> <Esc>o<CR>\item<Space>
nnoremap <C-t> }i\item<Space><Esc>o<Esc>"_DkO<Esc>_DjA
nnoremap <leader>al }koali
nnoremap <leader>ni /\\item<CR>zt
nnoremap <leader>pi ?\\item<CR>zt
nnoremap <leader>na /\\item<CR><esc>0ztA<Space>
nnoremap <leader>pa ?\\item<CR>n<esc>0ztA<Space>
inoremap <C-f> <Esc>: silent exec '.!inkscape-figures create "'.getline('.').'" "'.b:vimtex.root.'/figures/"'<CR><CR>:w<CR>
nnoremap <C-f> : silent exec '!inkscape-figures edit "'.b:vimtex.root.'/figures/" > /dev/null 2>&1 &'<CR><CR>:redraw!<CR>
inoremap <C-a> <Esc>F=i&<Esc>A<Space>\\<Esc><C-s>o
inoremap <C-g> <Esc>"+pa<Esc>03w

" 🔃 reload
nnoremap <leader>rs :call UltiSnips#RefreshSnippets()<CR>
nnoremap <leader>rm :source $HOME/.config/nvim/plugin/remaps.vim<CR>

" ⛵ reflow
inoremap <C-h> <Esc>VgwzHzzA
nnoremap <C-h> VgwzHzzg_
