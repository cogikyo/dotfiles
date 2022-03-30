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
keymap("n", "<C-s>", ":w<CR>", opts)
keymap("v", "<C-s>", "<Esc>:w<CR>", opts)

-- ✍  quit
keymap("n", "<leader>x", ":q<CR>", opts)
keymap("n", "<leader>q", ":q!<CR>", opts)

-- 🤖 copy copy (system clipboard)
keymap("v", "<leader>y", 'ml"+y`l', opts)
keymap("n", "<leader>y", '"+y', opts)
keymap("n", "<leader>Y", '"+y$', opts)
keymap("n", "<leader>gy", 'mlgg"+yG`lzt', opts)

-- 🦬 buffers
keymap("n", "<leader>w", ":bp<CR>", opts)
keymap("n", "<leader>v", ":bn<CR>", opts)
keymap("n", "<leader>bd", ":bd!<CR>", opts)

-- 🤯 comment headers
keymap("n", "<leader>hs", ":.! figlet -fshadow <Esc>:.,.+5Commentary<CR>5jo<Esc>0d$", opts)
keymap("n", "<leader>hc", ":.! figlet -fcalvin <Esc>:.,.+2Commentary<CR>2jo<Esc>0d$", opts)

-- 🌌 gimme space please (keep auto comments)
keymap("n", "o", "o<Esc>cc", opts)
keymap("n", "O", "O<Esc>cc", opts)
keymap("n", "<leader>o", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', opts)
keymap("n", "<leader>O", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', opts)

-- 💎 don't let go
keymap("n", "<leader>d", '"_d', opts)
keymap("v", "<leader>d", '"_d', opts)
keymap("v", "<", "<gv", opts)
keymap("v", ">", ">gv", opts)
keymap("v", "<Up>", ":m '<-2<CR>gv-gv", opts)
keymap("v", "<Down>", ":m '>+1<CR>gv-gv", opts)

-- 🎯 keep cursor 'centered'
keymap("n", "G", "Gzt,", opts)
keymap("n", "n", "nzt,", opts)
keymap("n", "N", "Nzt,", opts)
keymap("n", "}", "}zt,", opts)
keymap("n", "{", "{zt,", opts)
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
keymap("n", "<C-Up>", ":resize -2<CR>", opts)
keymap("n", "<C-Down>", ":resize +2<CR>", opts)
keymap("n", "<C-Left>", ":vertical resize -2<CR>", opts)
keymap("n", "<C-Right>", ":vertical resize +2<CR>", opts)

-- 👉 indent
keymap("n", "<leader>==", "ml=ip`lzt", opts)
keymap("n", "<leader>=y", "mlgg=G`lzt", opts)

-- 🔭 telescope
keymap("n", "<leader>t", "<cmd>lua require('telescope.builtin').find_files()<CR>", opts)
keymap("n", "<leader>T", "<cmd>lua require('telescope.builtin').live_grep()<CR>", opts)
keymap("n", "<leader>H", "<cmd>lua require('telescope.builtin').help_tags()<CR>", opts)
keymap("n", "<leader>G", "<cmd>lua require('telescope.builtin').git_files()<CR>", opts)
keymap("n", "<leader>M", "<cmd>lua require('telescope.builtin').man_pages()<CR>", opts)
keymap("n", "<leader>F", "<cmd>lua require('telescope.builtin').quickfix()<CR>", opts)
keymap("n", "<leader>L", "<cmd>lua require('telescope.builtin').loclist()<CR>", opts)
keymap("n", "<leader>:", "<cmd>lua require('telescope.builtin').commands()<CR>", opts)
keymap("n", "<leader>B", "<cmd>lua require('telescope.builtin').builtin()<CR>", opts)

keymap("n", "<leader>gr", "<cmd>lua require('telescope.builtin').lsp_references()<CR>", opts)
keymap("n", "<leader>gs", "<cmd>lua require('telescope.builtin').lsp_document_symbols()<CR>", opts)
keymap("n", "<leader>gS", "<cmd>lua require('telescope.builtin').lsp_dynamic_workspace_symbols()<CR>", opts)
keymap("n", "<leader>gi", "<cmd>lua require('telescope.builtin').lsp_implementations()<CR>", opts)
keymap("n", "<leader>gd", "<cmd>lua require('telescope.builtin').lsp_definitions()<CR>", opts)
keymap("n", "<leader>D", "<cmd>lua require('telescope.builtin').lsp_type_definitions()<CR>", opts)

keymap("n", "<leader>K", "<cmd>lua require('telescope.builtin').treesitter()<CR>", opts)
keymap("n", "<leader>P", "<cmd>lua require('telescope').extensions.media_files.media_files()<CR>", opts)

-- 🤲 toggle
keymap("n", "<leader>ut", ":UndotreeToggle<CR>", opts)
keymap("n", "<leader>pt", ":TSPlaygroundToggle<CR>", opts)
keymap("n", "<leader>ct", ":ColorizerToggle<CR>", opts)
keymap("n", "<leader>rt", ":call ToggleWrap()<CR>", opts)
keymap("n", "<leader>st", ":call ToggleSpellCheck()<CR>", opts)
keymap("n", "<leader>mt", ":MarkdownPreviewToggle<CR>,", opts)
keymap("n", "<leader>hl", ":TSHighlightCapturesUnderCursor<CR>", opts)
keymap("n", "<leader>lt", ":LspInstallInfo<CR>", opts)

-- 🔤 spelling
keymap("i", "<C-l>", "<c-g>u<Esc>[s1z=`]a<c-g>u", ff)

-- 🔃 reload
keymap("n", "<leader>rk", ":source ~/.config/nvim/lua/user/keymaps.lua<CR>", opts)

-- ⛵ reflow
keymap("n", "<leader>rx", "vipgqzt{j^", opts)
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
