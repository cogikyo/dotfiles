-- ██╗  ██╗███████╗██╗   ██╗███╗   ███╗ █████╗ ██████╗ ███████╗
-- ██║ ██╔╝██╔════╝╚██╗ ██╔╝████╗ ████║██╔══██╗██╔══██╗██╔════╝
-- █████╔╝ █████╗   ╚████╔╝ ██╔████╔██║███████║██████╔╝███████╗
-- ██╔═██╗ ██╔══╝    ╚██╔╝  ██║╚██╔╝██║██╔══██║██╔═══╝ ╚════██║
-- ██║  ██╗███████╗   ██║   ██║ ╚═╝ ██║██║  ██║██║     ███████║
-- ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝     ╚══════╝

local opts = { noremap = true, silent = true }
local optsf = { noremap = false, silent = false }
local map = vim.api.nvim_set_keymap

-- ☄  leader
map("", "<Space>", "<Nop>", opts)
vim.g.mapleader = " "

-- ⚕  save
map("i", "<C-s>", "<Esc>:w<CR>", opts)
map("n", "<C-s>", "<Esc>:w<CR>", opts)
map("v", "<C-s>", "<Esc>:w<CR>", opts)

-- ✍  quit
map("n", "<leader>x", ":q<CR>", opts)
map("n", "<leader>q", ":q!<CR>", opts)

-- 🤖 copy copy (system clipboard)
map("v", "<leader>y", '"+y', opts) -- yank selection x:q
map("n", "<leader>y", 'ml^"+y$<Esc>`l', opts) -- yank whole line
map("n", "<leader>ly", '"+y$', opts) -- yank to the end of line only
map("n", "<leader>Y", 'gg"+yG', opts) -- yank whole file

-- 🦬 buffers
map("n", "<leader>w", ":bp<CR>", opts)
map("n", "<leader>v", ":bn<CR>", opts)
map("n", "<leader>bd", ":bd!<CR>", opts)

-- 🤯 comment headers
map("n", "<leader>hs", ":.! figlet -fshadow <Esc>:.,.+5Commentary<CR>5jo<Esc>0d$", opts) map("n", "<leader>hc", ":.! figlet -fcalvin <Esc>:.,.+2Commentary<CR>2jo<Esc>0d$", opts)

-- 🌌 gimmie space please (keep auto comments)
map("n", "o", "o<Esc>cc", opts)
map("n", "O", "O<Esc>cc", opts)
map("n", "<leader>o", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', opts)
map("n", "<leader>O", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', opts)

-- 💎 don't let go
map("n", "<leader>d", '"_d', opts)
map("v", "<leader>d", '"_d', opts)
map("v", "p", '"_dP', opts)
map("v", "<", "<gv", opts)
map("v", ">", ">gv", opts)
map("x", "<A-u>", ":move '>-2<CR>gv-gv", opts)
map("x", "<A-i>", ":move '<+1<CR>gv-gv", opts)


-- 🎯 keep cursor 'centered'
map("n", "G", "Gzt,", opts)
map("n", "n", "nzt,", opts)
map("n", "N", "Nzt,", opts)
map("n", "<C-j>", "mzJ`z", opts)
map("n", "<C-o>", "<C-o>zt", opts)
map("n", "<C-i>", "<C-i>zt", opts)
map("n", "<C-f>", "<C-f>zt", opts)
map("n", "<C-b>", "<C-b>zt", opts)
map("n", "<C-d>", "<C-d>zt", opts)
map("n", "<C-u>", "<C-u>zt", opts)

-- 👈 undo break points
map("i", ",", ",<C-g>u", opts)
map("i", ".", ".<C-g>u", opts)
map("i", "!", "!<C-g>u", opts)
map("i", "?", "?<C-g>u", opts)
map("i", ";", ";<C-g>u", opts)
map("i", ":", ":<C-g>u", opts)

-- 🪟 window movement
map("n", "<Down>", "<C-w>j", opts)
map("n", "<Up>", "<C-w>k", opts)
map("n", "<Left>", "<C-w>h", opts)
map("n", "<Right>", "<C-w>l", opts)

-- 🔭 telescope
map("n", "<leader>tf", "<cmd>Telescope buffers<CR>", opts)
map("n", "<leader>tg", "<cmd>Telescope live_grep<CR>", opts)
map("n", "<leader>tt", "<cmd>Telescope find_files<CR>", opts)
map("n", "<leader>th", "<cmd>Telescope help_tags<CR>", opts)
map("n", "<leader>te", "<cmd>Telescope git_files<CR>", opts)
map("n", "<leader>tm", "<cmd>Telescope man_pages<CR>", opts)
map("n", "<leader>tr", "<cmd>Telescope commands<CR>", opts)

-- 🤲 toggle
map("n", "<leader>ut", ":UndotreeToggle<CR>", opts)
map("n", "<leader>pt", ":TSPlaygroundToggle<CR>", opts)
map("n", "<leader>ct", ":ColorizerToggle<CR>", opts)
map("n", "<leader>tw", ":call ToggleWrap()<CR>", opts)
map("n", "<leader>st", ":call ToggleSpellCheck()<CR>", opts)
map("n", "<leader>mp", ":MarkdownPreviewToggle<CR>,", opts)
map("n", "<leader>tk", ":TSHighlightCapturesUnderCursor<CR>", opts)

-- 🔤 spelling
map("i", "<C-l>", "<c-g>u<Esc>[s1z=`]a<c-g>u,", opts)

-- 🔃 reload
map("n", "<leader>rs", ":call UltiSnips#RefreshSnippets()<CR>", opts)
map("n", "<leader>rv", ":source $MYVIMRC<CR>:w<CR>", opts)

-- ⛵ reflow
map("n", "<leader>rx", "vipgqzt{j^", opts) -- reflow line
map("n", "<leader>rf", "mzJ`zVgwzHzt_", opts)

-- 🔱 harpoon
map("n", "<leader>nn", ":lua require('harpoon.mark').add_file()<CR>", opts)
map("n", "<leader>nf", ":lua require('harpoon.ui').toggle_quick_menu()<CR>", opts)
map("n", "<leader>nt", ":lua require('harpoon.ui').nav_file(1)<CR>zt", opts)
map("n", "<leader>ne", ":lua require('harpoon.ui').nav_file(2)<CR>zt", opts)
map("n", "<leader>ns", ":lua require('harpoon.ui').nav_file(3)<CR>zt", opts)
map("n", "<leader>na", ":lua require('harpoon.ui').nav_file(4)<CR>zt", opts)
map("n", "<leader>nd", ":lua require('harpoon.ui').nav_file(5)<CR>zt", opts)

-- 👾 search and replace (better way to do this?)
map("v", "s", ":<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>", opts)
map("n", "<leader>sa", "m0ggVG:<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>", opts)

-- 🙏 folds
map("n", "<leader>bf", "zf%", opts)
map("n", "<leader>cf", "zfip", opts)
map("n", "<leader>mk", ":mkview<CR>", opts)
map("n", "<leader>lv", ":loadview<CR>", opts)

-- 🚦 easy align
map("x", "<leader>e", "<Plug>(EasyAlign)", optsf)
map("n", "<leader>e", "<Plug>(EasyAlign)", optsf)

