" ██████╗ ███████╗███╗   ███╗ █████╗ ██████╗ ███████╗
" ██╔══██╗██╔════╝████╗ ████║██╔══██╗██╔══██╗██╔════╝
" ██████╔╝█████╗  ██╔████╔██║███████║██████╔╝███████╗
" ██╔══██╗██╔══╝  ██║╚██╔╝██║██╔══██║██╔═══╝ ╚════██║
" ██║  ██║███████╗██║ ╚═╝ ██║██║  ██║██║     ███████║
" ╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝╚═╝  ╚═╝╚═╝     ╚══════╝

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
nnoremap <leader>q :q!<CR>

" 🦬 buffers
nnoremap <leader>w :bp<CR>
nnoremap <leader>v :bn<CR>
nnoremap <leader>bd :bd!<CR>

" 🤖 copy copy
nnoremap <leader>d "_d
vnoremap <leader>d "_d
vnoremap <leader>y "+y
nnoremap <leader>y ml^"+y$<Esc>`l
nnoremap <leader>ly "+y$
nnoremap <leader>Y gg"+yG
vnoremap <leader>p y']o<Esc>p`]o<Esc>

" 🤯 comment headers
nnoremap <leader>hs :.! figlet -fshadow <Esc>:.,.+5Commentary<CR>5jo<Esc>0d$
nnoremap <leader>hc :.! figlet -fcalvin <Esc>:.,.+2Commentary<CR>2jo<Esc>0d$

" 🌌 gimmie space please
nnoremap <leader>o o<Esc>"_Di
nnoremap <leader>O O<Esc>"_Di
inoremap <C-Space> <Esc>o<Esc>"_DkO<Esc>_DjA

" 🎯 keep cursor 'centered'
nnoremap G Gzt
nnoremap n nzt
nnoremap N Nzt
nnoremap <C-j> mzJ`z
nnoremap <C-o> <C-o>zt
nnoremap <C-i> <C-i>zt
nnoremap <C-f> <C-f>zt
nnoremap <C-b> <C-b>zt
nnoremap <C-d> <C-d>zt
nnoremap <C-u> <C-u>zt

" 👈 undo break points
inoremap , ,<c-g>u
inoremap . .<c-g>u
inoremap ! !<c-g>u
inoremap ? ?<c-g>u
inoremap ; ;<c-g>u
inoremap : :<c-g>u

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
nnoremap <leader>pt :TSPlaygroundToggle<CR>
nnoremap <leader>ct :ColorizerToggle<CR>
nnoremap <leader>tw :call ToggleWrap()<CR>
nnoremap <leader>st :call ToggleSpellCheck()<CR>
nnoremap <leader>mp :MarkdownPreviewToggle<CR>
nnoremap <leader>tk :TSHighlightCapturesUnderCursor<CR>

" 🔤 spelling
inoremap <C-l> <c-g>u<Esc>[s1z=`]a<c-g>u

" 📓 latex
augroup LATEX
  autocmd!
  autocmd FileType tex inoremap <C-t> <Esc>/end<CR>O<ESC>O\item<Space>
  autocmd FileType tex nnoremap <C-t> /end<CR>O<ESC>O\item<Space>
  autocmd FileType tex nnoremap <leader><CR> o<CR>\item<Space>
  autocmd FileType tex nnoremap <leader>al }koali
  autocmd FileType tex nnoremap <leader>en }koenum
  autocmd FileType tex nnoremap <leader>ni /\\item<CR>ztfm
  autocmd FileType tex nnoremap <leader>in ?\\item<CR>nztfm
  autocmd FileType tex inoremap <C-a> <Esc>F=i&<Esc>A<Space>\\<Esc>:w<cr>o
  autocmd FileType tex nnoremap <leader>np }inewpage
augroup END

" 🔃 reload
nnoremap <leader>rs :call UltiSnips#RefreshSnippets()<CR>
nnoremap <leader>rm :source $MYVIMRC<CR>:w<CR>

" ⛵ reflow
nnoremap <leader>rx vipgqzt{j^
inoremap <C-h> <Esc>VgwzHztA
nnoremap <C-h> VgwzHzt$
nnoremap <leader>rf mzJ`zVgwzHzt_
inoremap <C-g>g <Esc>"+pvipgqzt{j^

" 🔱 harpoon
nnoremap <leader>nn :lua require("harpoon.mark").add_file()<CR>
nnoremap <leader>nf :lua require("harpoon.ui").toggle_quick_menu()<CR>
nnoremap <leader>nt :lua require("harpoon.ui").nav_file(1)<CR>zt
nnoremap <leader>ne :lua require("harpoon.ui").nav_file(2)<CR>zt
nnoremap <leader>ns :lua require("harpoon.ui").nav_file(3)<CR>zt
nnoremap <leader>na :lua require("harpoon.ui").nav_file(4)<CR>zt
nnoremap <leader>nd :lua require("harpoon.ui").nav_file(5)<CR>zt

" 👾 regex
vnoremap s :<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>
nnoremap <leader>sa m0ggVG:<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>

" 🚦 easy align
nmap gl <Plug>(EasyAlign)
nmap <leader>et vip<Plug>(EasyAlign)
xmap gl <Plug>(EasyAlign)

" 🙏 folds
nnoremap <leader>bf t{zf%
nnoremap <leader>cf vipzf
nnoremap <leader>mk :mkview<CR>
nnoremap <leader>lv :loadview<CR>

