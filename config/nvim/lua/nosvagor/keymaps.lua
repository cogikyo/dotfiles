-- ██╗  ██╗███████╗██╗   ██╗███╗   ███╗ █████╗ ██████╗ ███████╗
-- ██║ ██╔╝██╔════╝╚██╗ ██╔╝████╗ ████║██╔══██╗██╔══██╗██╔════╝
-- █████╔╝ █████╗   ╚████╔╝ ██╔████╔██║███████║██████╔╝███████╗
-- ██╔═██╗ ██╔══╝    ╚██╔╝  ██║╚██╔╝██║██╔══██║██╔═══╝ ╚════██║
-- ██║  ██╗███████╗   ██║   ██║ ╚═╝ ██║██║  ██║██║     ███████║
-- ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝     ╚═╝╚═╝  ╚═╝╚═╝     ╚══════╝

local opts = { noremap = true, silent = true }
local ff = { noremap = false, silent = false }
local keymap = vim.api.nvim_set_keymap

-- ☄  leader
keymap("", "<Space>", "<Nop>", opts)
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
keymap("i", "<C-s>", "<Esc>:w<CR>", opts)
keymap("n", "<C-s>", "<Esc>:w<CR>", opts)
keymap("v", "<C-s>", "<Esc>:w<CR>", opts)

-- ✍  quit
keymap("n", "<leader>x", ":q<CR>", opts)
keymap("n", "<leader>q", ":q!<CR>", opts)

-- 🤖 copy copy (system clipboard)
keymap("v", "<leader>y", '"+y', opts) -- yank selection x:q
keymap("n", "<leader>y", 'ml^"+y$<Esc>`l', opts) -- yank whole line
keymap("n", "<leader>ly", '"+y$', opts) -- yank to the end of line only
keymap("n", "<leader>Y", 'gg"+yG', opts) -- yank whole file

-- 🦬 buffers
keymap("n", "<leader>w", ":bp<CR>", opts)
keymap("n", "<leader>v", ":bn<CR>", opts)
keymap("n", "<leader>bd", ":bd!<CR>", opts)

-- 🤯 comment headers
keymap("n", "<leader>hs", ":.! figlet -fshadow <Esc>:.,.+5Commentary<CR>5jo<Esc>0d$", opts)
keymap("n", "<leader>hc", ":.! figlet -fcalvin <Esc>:.,.+2Commentary<CR>2jo<Esc>0d$", opts)

-- 🌌 gimmie space please (keep auto comments)
keymap("n", "o", "o<Esc>cc", opts)
keymap("n", "O", "O<Esc>cc", opts)
keymap("n", "<leader>o", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', opts)
keymap("n", "<leader>O", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', opts)

-- 💎 don't let go
keymap("n", "<leader>d", '"_d', opts)
keymap("v", "<leader>d", '"_d', opts)
keymap("v", "p", '"_dP', opts)
keymap("v", "<", "<gv", opts)
keymap("v", ">", ">gv", opts)
keymap("x", "<Down>", ":m '>+1<CR>gv-gv", opts)
keymap("x", "<Up>", ":m '<-2<CR>gv-gv", opts)

-- 🎯 keep cursor 'centered'
keymap("n", "G", "Gzt,", opts)
keymap("n", "n", "nzt,", opts)
keymap("n", "N", "Nzt,", opts)
keymap("n", "<C-j>", "mzJ`z", opts)
keymap("n", "<C-o>", "<C-o>zt", opts)
keymap("n", "<C-i>", "<C-i>zt", opts)
keymap("n", "<C-f>", "<C-f>zt", opts)
keymap("n", "<C-b>", "<C-b>zt", opts)
keymap("n", "<C-d>", "<C-d>zt", opts)
keymap("n", "<C-u>", "<C-u>zt", opts)

-- 👈 undo break points
keymap("i", ",", ",<C-g>u", opts)
keymap("i", ".", ".<C-g>u", opts)
keymap("i", "!", "!<C-g>u", opts)
keymap("i", "?", "?<C-g>u", opts)
keymap("i", ";", ";<C-g>u", opts)
keymap("i", ":", ":<C-g>u", opts)

-- 🪟 window movement
keymap("n", "<Down>", "<C-w>j", opts)
keymap("n", "<Up>", "<C-w>k", opts)
keymap("n", "<Left>", "<C-w>h", opts)
keymap("n", "<Right>", "<C-w>l", opts)

-- 🔭 telescope
keymap("n", "<leader>tf", "<cmd>Telescope buffers<CR>", opts)
keymap("n", "<leader>tg", "<cmd>Telescope live_grep<CR>", opts)
keymap("n", "<leader>tt", "<cmd>Telescope find_files<CR>", opts)
keymap("n", "<leader>th", "<cmd>Telescope help_tags<CR>", opts)
keymap("n", "<leader>te", "<cmd>Telescope git_files<CR>", opts)
keymap("n", "<leader>tm", "<cmd>Telescope man_pages<CR>", opts)
keymap("n", "<leader>tr", "<cmd>Telescope commands<CR>", opts)

-- 🤲 toggle
keymap("n", "<leader>ut", ":UndotreeToggle<CR>", opts)
keymap("n", "<leader>pt", ":TSPlaygroundToggle<CR>", opts)
keymap("n", "<leader>ct", ":ColorizerToggle<CR>", opts)
keymap("n", "<leader>tw", ":call ToggleWrap()<CR>", opts)
keymap("n", "<leader>st", ":call ToggleSpellCheck()<CR>", opts)
keymap("n", "<leader>mp", ":MarkdownPreviewToggle<CR>,", opts)
keymap("n", "<leader>tk", ":TSHighlightCapturesUnderCursor<CR>", opts)

-- 🔤 spelling
keymap("i", "<C-l>", "<c-g>u<Esc>[s1z=`]a<c-g>u", opts)

-- 🔃 reload
keymap("n", "<leader>rs", ":call UltiSnips#RefreshSnippets()<CR>", opts)
keymap("n", "<leader>rv", ":source $MYVIMRC<CR>:w<CR>", opts)

-- ⛵ reflow
keymap("n", "<leader>rx", "vipgqzt{j^", opts) -- reflow line
keymap("n", "<leader>rf", "mzJ`zVgwzHzt_", opts)

-- 🔱 harpoon
keymap("n", "<leader>nn", ":lua require('harpoon.mark').add_file()<CR>", opts)
keymap("n", "<leader>nf", ":lua require('harpoon.ui').toggle_quick_menu()<CR>", opts)
keymap("n", "<leader>nt", ":lua require('harpoon.ui').nav_file(1)<CR>zt", opts)
keymap("n", "<leader>ne", ":lua require('harpoon.ui').nav_file(2)<CR>zt", opts)
keymap("n", "<leader>ns", ":lua require('harpoon.ui').nav_file(3)<CR>zt", opts)
keymap("n", "<leader>na", ":lua require('harpoon.ui').nav_file(4)<CR>zt", opts)
keymap("n", "<leader>nd", ":lua require('harpoon.ui').nav_file(5)<CR>zt", opts)

-- 👾 search and replace (better way to do this?)
vim.cmd [[
vnoremap s :<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>
nnoremap <leader>sa m0ggVG:<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>
]]

-- 🙏 folds
keymap("n", "<leader>bf", "zf%", opts)
keymap("n", "<leader>cf", "zfip", opts)
keymap("n", "<leader>mk", ":mkview<CR>", opts)
keymap("n", "<leader>lv", ":loadview<CR>", opts)

-- 🚦 easy align
keymap("x", "<leader>ea", "<Plug>(EasyAlign)", ff)
keymap("n", "<leader>ea", "<Plug>(EasyAlign)", ff)
