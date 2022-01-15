" ╦═╗┌─┐┌┬┐┌─┐┌─┐┌─┐
" ╠╦╝├┤ │││├─┤├─┘└─┐
" ╩╚═└─┘┴ ┴┴ ┴┴  └─┘
nnoremap <SPACE> <Nop>
let mapleader = " "

" ⚕  esc
inoremap <C-c> <esc>
inoremap <C-s> <esc>:w<CR>
nnoremap <C-s> <esc>:w<CR>
vnoremap <C-s> <esc>:w<CR>

" 🔍 go to letter(s)
nmap t <Plug>(easymotion-bd-f)
nmap T <Plug>(easymotion-bd-f2)

" ✍  quit/write
nnoremap <leader>x :q<CR>
nnoremap <leader>q :wq<CR>

" 🦬 buffers
nnoremap <leader>w :bp<CR>
nnoremap <leader>v :bn<CR>
nnoremap <leader>dd :bd!<CR>

" 🤖 copy copy
nnoremap <leader>y ^"+y$
vnoremap <leader>y "+y
nnoremap <leader>ay g"+yG
nmap gy `]
vnoremap <leader>p y']o<Esc>p`]o<Esc>
inoremap <C-g>g <Esc>"+pa<Esc>VgwzHztA<Esc>vipgq

" 🤯 comment headers
nnoremap <leader>hs :.! figlet -fshadow <Esc>:.,.+5Commentary<CR>5jo<Esc>0d$
nnoremap <leader>hc :.! figlet -fcalvin <Esc>:.,.+2Commentary<CR>2jo<Esc>0d$

" 🌌 gimmie space please
nnoremap <leader>o o<Esc>"_Dk$
nnoremap <leader>O O<Esc>"_Dj$
inoremap <C-Space> <Esc>o<Esc>"_DkO<Esc>_DjA

" 🎯 keep cursor 'centered'
nnoremap } }zt
nnoremap { {zt
nnoremap G Gzt
nnoremap <leader>gg Gzt
nnoremap n nztzv
nnoremap N Nztzv
nnoremap <C-j> mzJ`z
" noremap j jzt
" noremap k kzt

" 🔖 temp mark
nnoremap <leader>ss m0
nnoremap <leader>rr `0zt

" 👈 undo break points
inoremap , ,<c-g>u
inoremap . .<c-g>u
inoremap ! !<c-g>u
inoremap ? ?<c-g>u
inoremap ; ;<c-g>u
inoremap : :<c-g>u

" 📢 capilization
nnoremap <leader>el gue
nnoremap <leader>eu gUe

" 🚙 moving text
vnoremap <Up> :m '<-2<CR>gv=gv
vnoremap <Down> :m '>+1<cr>gv=gv

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
augroup LATEX
  autocmd!
  autocmd FileType tex inoremap <C-t> <Esc>}i\item<Space><Esc>o<Esc>"_DkO<Esc>_DjA
  autocmd FileType tex nnoremap <C-t> }i\item<Space><Esc>o<Esc>"_DkO<Esc>_DjA
  autocmd FileType tex nnoremap <leader><CR> o<CR>\item<Space>
  autocmd FileType tex nnoremap <leader>al }koali
  autocmd FileType tex nnoremap <leader>ni /\\item<CR>zt
  autocmd FileType tex nnoremap <leader>in ?\\item<CR>zt
  "autocmd FiletYpe tex inoremap <C-f> <Esc>: silent exec '.!inkscape-figures create "'.getline('.').'" "'.b:vimtex.root.'/figures/"'<CR><CR>:w<CR>
  "autocmd FiletYpe tex nnoremap <C-f> : silent exec '!inkscape-figures edit "'.b:vimtex.root.'/figures/" > /dev/null 2>&1 &'<CR><CR>:redraw!<CR>
  autocmd FileType tex inoremap <C-a> <Esc>F=i&<Esc>A<Space>\\<Esc>:w<cr>o
  autocmd FileType tex nnoremap <leader>np }inewpage
augroup END

" 🔃 reload
nnoremap <leader>rs :call UltiSnips#RefreshSnippets()<CR>
nnoremap <leader>rm :source $HOME/.config/nvim/plugin/remaps.vim<CR>

" ⛵ reflow
nnoremap <leader>rf vipgqzz

" 🔱 harpoon
nnoremap <leader>nn :lua require("harpoon.mark").add_file()<CR>
nnoremap <leader>nf :lua require("harpoon.ui").toggle_quick_menu()<CR>
nnoremap <leader>nt :lua require("harpoon.ui").nav_file(1)<CR>
nnoremap <leader>ne :lua require("harpoon.ui").nav_file(2)<CR>
nnoremap <leader>ns :lua require("harpoon.ui").nav_file(3)<CR>
nnoremap <leader>na :lua require("harpoon.ui").nav_file(4)<CR>
nnoremap <leader>nd :lua require("harpoon.ui").nav_file(5)<CR>

" 👾 regex
vnoremap s :<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>
nnoremap <leader>sa m0ggVG:<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>

" 🛰 eeasy align
vnoremap <leader>et :EasyAlign<CR>
nnoremap <leader>eta <ESC>vip:EasyAlign<CR>**&<CR>

" 📜 comfortable motion
let g:comfortable_motion_scroll_down_key = "j"
let g:comfortable_motion_scroll_up_key = "k"
let g:comfortable_motion_friction = 150
let g:comfortable_motion_air_drag = 0.5
nnoremap <silent> <C-d> :call comfortable_motion#flick(75)<CR>zt
nnoremap <silent> <C-u> :call comfortable_motion#flick(-75)<CR>zt
noremap <silent> <ScrollWheelDown> :call comfortable_motion#flick(100)<CR>
noremap <silent> <ScrollWheelUp>   :call comfortable_motion#flick(-100)<CR>
