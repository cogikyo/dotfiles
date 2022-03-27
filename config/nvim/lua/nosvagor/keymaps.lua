-- ██╗  ██╗███████╗██╗   ██╗███╗   ███╗ █████╗ ██████╗ ███████╗
-- ██║ ██╔╝██╔════╝╚██╗ ██╔╝████╗ ████║██╔══██╗██╔══██╗██╔════╝
-- █████╔╝ █████╗   ╚████╔╝ ██╔████╔██║███████║██████╔╝███████╗
-- ██╔═██╗ ██╔══╝    ╚██╔╝  ██║╚██╔╝██║██╔══██║██╔═══╝ ╚════██║
-- ██║  ██╗███████╗   ██║   ██║ ╚═╝ ██║██║  ██║██║     ███████║
-- ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝     ╚══════╝

local tt = { noremap = true, silent = true }
local ff = { noremap = false, silent = false }
local ft = { noremap = false, silent = true }
local map = vim.api.nvim_set_keymap

-- ☄  leader
map("", "<Space>", "<Nop>", tt)
vim.g.mapleader = " "

-- ┌┬┐┌─┐┌┬┐┌─┐┌─┐
-- ││││ │ ││├┤ └─┐
-- ┴ ┴└─┘─┴┘└─┘└─┘ normal_mode = "n",
-- insert_mode = "i",
-- visual_mode = "v",
-- visual_block_mode = "x",
-- _mode = "t",
-- command_mode = "c",

-- ⚕  save
map("i", "<C-s>", "<Esc>:w<CR>", tt)
map("n", "<C-s>", "<Esc>:w<CR>", tt)
map("v", "<C-s>", "<Esc>:w<CR>", tt)

-- ✍  quit
map("n", "<leader>x", ":q<CR>", tt)
map("n", "<leader>q", ":q!<CR>", tt)

-- 🤖 copy copy (system clipboard)
map("v", "<leader>y", '"+y', tt) -- yank selection x:q
map("n", "<leader>y", 'ml^"+y$<Esc>`l', tt) -- yank whole line
map("n", "<leader>ly", '"+y$', tt) -- yank to the end of line only
map("n", "<leader>Y", 'gg"+yG', tt) -- yank whole file

-- 🦬 buffers
map("n", "<leader>w", ":bp<CR>", tt)
map("n", "<leader>v", ":bn<CR>", tt)
map("n", "<leader>bd", ":bd!<CR>", tt)

-- 🤯 comment headers
map("n", "<leader>hs", ":.! figlet -fshadow <Esc>:.,.+5Commentary<CR>5jo<Esc>0d$", tt) map("n", "<leader>hc", ":.! figlet -fcalvin <Esc>:.,.+2Commentary<CR>2jo<Esc>0d$", tt)

-- 🌌 gimmie space please (keep auto comments)
map("n", "o", "o<Esc>cc", tt)
map("n", "O", "O<Esc>cc", tt)
map("n", "<leader>o", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', tt)
map("n", "<leader>O", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', tt)

-- 💎 don't let go
map("n", "<leader>d", '"_d', tt)
map("v", "<leader>d", '"_d', tt)
map("v", "p", '"_dP', tt)
map("v", "<", "<gv", tt)
map("v", ">", ">gv", tt)
map("x", "<Down>", ":m '>+1<CR>gv-gv", tt)
map("x", "<Up>", ":m '<-2<CR>gv-gv", tt)

-- 🎯 keep cursor 'centered'
map("n", "G", "Gzt,", tt)
map("n", "n", "nzt,", tt)
map("n", "N", "Nzt,", tt)
map("n", "<C-j>", "mzJ`z", tt)
map("n", "<C-o>", "<C-o>zt", tt)
map("n", "<C-i>", "<C-i>zt", tt)
map("n", "<C-f>", "<C-f>zt", tt)
map("n", "<C-b>", "<C-b>zt", tt)
map("n", "<C-d>", "<C-d>zt", tt)
map("n", "<C-u>", "<C-u>zt", tt)

-- 👈 undo break points
map("i", ",", ",<C-g>u", tt)
map("i", ".", ".<C-g>u", tt)
map("i", "!", "!<C-g>u", tt)
map("i", "?", "?<C-g>u", tt)
map("i", ";", ";<C-g>u", tt)
map("i", ":", ":<C-g>u", tt)

-- 🪟 window movement
map("n", "<Down>", "<C-w>j", tt)
map("n", "<Up>", "<C-w>k", tt)
map("n", "<Left>", "<C-w>h", tt)
map("n", "<Right>", "<C-w>l", tt)

-- 🔭 telescope
map("n", "<leader>tf", "<cmd>Telescope buffers<CR>", tt)
map("n", "<leader>tg", "<cmd>Telescope live_grep<CR>", tt)
map("n", "<leader>tt", "<cmd>Telescope find_files<CR>", tt)
map("n", "<leader>th", "<cmd>Telescope help_tags<CR>", tt)
map("n", "<leader>te", "<cmd>Telescope git_files<CR>", tt)
map("n", "<leader>tm", "<cmd>Telescope man_pages<CR>", tt)
map("n", "<leader>tr", "<cmd>Telescope commands<CR>", tt)

-- 🤲 toggle
map("n", "<leader>ut", ":UndotreeToggle<CR>", tt)
map("n", "<leader>pt", ":TSPlaygroundToggle<CR>", tt)
map("n", "<leader>ct", ":ColorizerToggle<CR>", tt)
map("n", "<leader>tw", ":call ToggleWrap()<CR>", tt)
map("n", "<leader>st", ":call ToggleSpellCheck()<CR>", tt)
map("n", "<leader>mp", ":MarkdownPreviewToggle<CR>,", tt)
map("n", "<leader>tk", ":TSHighlightCapturesUnderCursor<CR>", tt)

-- 🔤 spelling
map("i", "<C-l>", "<c-g>u<Esc>[s1z=`]a<c-g>u,", tt)

-- 🔃 reload
map("n", "<leader>rs", ":call UltiSnips#RefreshSnippets()<CR>", tt)
map("n", "<leader>rv", ":source $MYVIMRC<CR>:w<CR>", tt)

-- ⛵ reflow
map("n", "<leader>rx", "vipgqzt{j^", tt) -- reflow line
map("n", "<leader>rf", "mzJ`zVgwzHzt_", tt)

-- 🔱 harpoon
map("n", "<leader>nn", ":lua require('harpoon.mark').add_file()<CR>", tt)
map("n", "<leader>nf", ":lua require('harpoon.ui').toggle_quick_menu()<CR>", tt)
map("n", "<leader>nt", ":lua require('harpoon.ui').nav_file(1)<CR>zt", tt)
map("n", "<leader>ne", ":lua require('harpoon.ui').nav_file(2)<CR>zt", tt)
map("n", "<leader>ns", ":lua require('harpoon.ui').nav_file(3)<CR>zt", tt)
map("n", "<leader>na", ":lua require('harpoon.ui').nav_file(4)<CR>zt", tt)
map("n", "<leader>nd", ":lua require('harpoon.ui').nav_file(5)<CR>zt", tt)

-- 👾 search and replace (better way to do this?)
vim.cmd [[
vnoremap s :<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>
nnoremap <leader>sa m0ggVG:<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>
]]

-- 🙏 folds
map("n", "<leader>bf", "zf%", tt)
map("n", "<leader>cf", "zfip", tt)
map("n", "<leader>mk", ":mkview<CR>", tt)
map("n", "<leader>lv", ":loadview<CR>", tt)

-- 🚦 easy align
map("x", "<leader>e", "<Plug>(EasyAlign)", ff)
map("n", "<leader>e", "<Plug>(EasyAlign)", ff)
