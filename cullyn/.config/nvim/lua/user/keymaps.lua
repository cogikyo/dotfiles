-- ============================================================================
-- 📎 Setup {{{
local function map(mode, lhs, rhs, opts)
    opts = vim.tbl_extend("force", { noremap = true, silent = true }, opts or {})
    vim.api.nvim_set_keymap(mode, lhs, rhs, opts)
end

local ft = { noremap = false, silent = true }
local ff = { noremap = false, silent = false }
-- }}}
-- ============================================================================

-- 🌠 Leader {{{
map("", "<Space>", "<Nop>")
map("n", "<leader>s", "<Nop>")
map("n", "<leader>r", "<Nop>")
vim.g.mapleader = " "
-- }}}

-- 💾 Save {{{
map("i", "<C-s>", "<Esc>:w<CR>")
map("n", "<C-s>", ":lua vim.lsp.buf.format()<CR>:w<CR>")
map("n", "<leader>s<leader>", ":noa w<CR>")
map("v", "<C-s>", "<Esc>:w<CR>")
map("n", "<leader>rs", ":w | source %<CR>")
-- }}}

-- 🖐️ Quit {{{
map("n", "<leader>q", ":q!<CR>")
map("n", "<C-c>", "<Esc>")
-- }}}

-- 🍸 Vestigial {{{
map("n", "<C-z>", "u")
map("n", "<C-g>", '"*p')
map("i", "<C-v>", '<Esc>"+p')
-- }}}

-- 🤖 Copy copy {{{
map("v", "<leader>y", 'ml"+y`l')
map("n", "<leader>y", '"+y')
map("n", "<leader>Y", '"+y$')
map("n", "<leader>gy", 'mlgg"+yG`lzt')
map("n", "<leader>p", "`]p`]")
map("v", "<leader>p", "y`]p`]ml`[O<Esc>'lo<Esc>")
map("v", "<leader>P", "yP`]o<Esc>j")
-- }}}

-- 🦬 Buffers {{{
map("n", "<leader>b", ":bn<CR>zt")
map("n", "<leader>B", ":bp<CR>zt")
map("n", "<leader><C-b>", ":bd!<CR>zt")
-- }}}

-- 🌌 Gimme space please {{{
map("n", "<leader>o", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>')
map("n", "<leader>O", ':<C-u>call append(line(".")-1,   repeat([""], v:count1))<CR>')
map("n", "<leader>a", '<leader>o<leader>O', ft)
-- }}}

-- 💎 Don't let go {{{
map("n", "<leader>d", '"_d')
map("v", "<leader>d", '"_d')
map("n", "<leader>c", '"_c')
map("v", "<leader>c", '"_c')
map("n", "<leader>C", '"_C')
map("n", "<leader>x", '"_x')
map("n", "<leader>X", '"_X')
map("v", "<", "<gv")
map("v", ">", ">gv")
map("v", "<Up>", ":m '<-2<CR>gv-gv")
map("v", "<Down>", ":m '>+1<CR>gv-gv")
-- }}}

-- 🎯 Keep cursor 'centered' {{{
map("n", "G", "Gzt")
map("n", "n", "nzt")
map("n", "N", "Nzt")
map("n", "}", "}zt")
map("n", "{", "{zt")
map("n", "<C-j>", "mzJ`z")
map("n", "<C-o>", "<C-o>zt")
map("n", "<C-i>", "<C-i>zt")
map("n", "<C-f>", "<C-f>zt")
map("n", "<C-b>", "<C-b>zt")
map("n", "<C-d>", "<C-d>zt")
map("n", "<C-u>", "<C-u>zt")
-- }}}

-- 👈 Undo break points {{{
map("i", ",", ",<C-g>u")
map("i", ".", ".<C-g>u")
map("i", "!", "!<C-g>u")
map("i", "?", "?<C-g>u")
map("i", ";", ";<C-g>u")
map("i", ":", ":<C-g>u")
-- }}}

-- 🪟 Window movement {{{
map("n", "<Down>", "<C-w>j")
map("n", "<Up>", "<C-w>k")
map("n", "<Left>", "<C-w>h")
map("n", "<Right>", "<C-w>l")
map("n", "<C-Up>", ":resize +2<CR>")
map("n", "<C-Down>", ":resize -2<CR>")
map("n", "<C-Left>", ":vertical resize -2<CR>")
map("n", "<C-Right>", ":vertical resize +2<CR>")
-- }}}

-- 👉 Indent {{{
map("n", "<leader>==", "ml=ip`lzt")
map("n", "<leader>=y", "mlgg=G`lzt")
-- }}}

-- 🔭 Telescope {{{
map("n", "<leader>t", "<Nop>")
map("n", "<leader>t<leader>", "<cmd>lua require('telescope.builtin').find_files({hidden = true})<CR>")
map("n", "<leader>T", "<cmd>lua require('telescope.builtin').live_grep()<CR>")
map("n", "<leader>th", "<cmd>lua require('telescope.builtin').help_tags()<CR>")
map("n", "<leader>te", "<cmd>lua require('telescope.builtin').git_files()<CR>")
map("n", "<leader>tm", "<cmd>lua require('telescope.builtin').man_pages()<CR>")
map("n", "<leader>L", "<cmd>lua require('telescope.builtin').quickfix()<CR>")
map("n", "<leader>tl", "<cmd>lua require('telescope.builtin').loclist()<CR>")
map("n", "<leader>tc", "<cmd>lua require('telescope.builtin').commands()<CR>")
map("n", "<leader>tr", "<cmd>lua require('telescope.builtin').oldfiles()<CR>")
map("n", "<leader>tp", "<cmd>lua require('telescope.builtin').builtin()<CR>")
map("n", "<leader>tb", "<cmd>lua require('telescope.builtin').buffers()<CR>")

map("n", "<leader>gr", "<cmd>lua require('telescope.builtin').lsp_references()<CR>")
map("n", "<leader>gs", "<cmd>lua require('telescope.builtin').lsp_document_symbols()<CR>")
map("n", "<leader>gS", "<cmd>lua require('telescope.builtin').lsp_dynamic_workspace_symbols()<CR>")
map("n", "<leader>gi", "<cmd>lua require('telescope.builtin').lsp_implementations()<CR>")
map("n", "<leader>gd", "<cmd>lua require('telescope.builtin').lsp_definitions()<CR>")
map("n", "<leader>D", "<cmd>lua require('telescope.builtin').lsp_type_definitions()<CR>")

map("n", "<leader>ts", "<cmd>lua require('telescope.builtin').treesitter()<CR>")
-- }}}

-- ℹ️  Null_ls {{{
map("n", "<leader>ca", ": lua vim.lsp.buf.code_action()<CR>")
map("n", "<leader>hi", ": lua vim.lsp.buf.hover()<CR>")
-- }}}

-- 🤲 Toggle {{{
map("n", "<leader>ut", ":UndotreeToggle<CR>")
map("n", "<leader>HT", ":TSPlaygroundToggle<CR>")
map("n", "<leader>ct", ":ColorizerToggle<CR>")
map("n", "<leader>rt", ":call ToggleWrap()<CR>")
map("n", "<leader>st", ":call ToggleSpellCheck()<CR>")
map("n", "<leader>mt", ":MarkdownPreviewToggle<CR>,")
map("n", "<leader>hl", ":TSHighlightCapturesUnderCursor<CR>")
map("n", "<leader>ls", ":LspInstallInfo<CR>")
map("n", "<leader>et", ":NvimTreeToggle<CR> :NvimTreeRefresh<CR>")
map("n", "<leader>tt", ":Switch<CR>") -- bool toggle false<->true
map("n", "<leader>it", ":NullLsInfo<CR>")
-- }}}

-- ⛵ Reflow TODO: FIXME {{{
map("n", "<leader>rx", "vipgqzt{j^")
map("n", "<leader>rf", "mzJ`zVgwzHzt_")
-- }}}

-- 🔱 Harpoon {{{
map("n", "<leader>n", "<Nop>")
map("n", "<leader>nn", ":lua require('harpoon.mark').add_file()<CR>")
map("n", "<leader>ng", ":lua require('harpoon.ui').toggle_quick_menu()<CR>")
map("n", "<leader>nt", ":lua require('harpoon.ui').nav_file(1)<CR>zt")
map("n", "<leader>ne", ":lua require('harpoon.ui').nav_file(2)<CR>zt")
map("n", "<leader>ns", ":lua require('harpoon.ui').nav_file(3)<CR>zt")
map("n", "<leader>na", ":lua require('harpoon.ui').nav_file(4)<CR>zt")
map("n", "<leader>nd", ":lua require('harpoon.ui').nav_file(5)<CR>zt")
-- }}}

-- 🔍 Replace {{{
map("n", "<leader>ir", "vip:<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>", ff)
map("v", "r", ":<BS><BS><BS><BS><BS>'<,'>s///g<Left><Left><Left>", ff)
map("n", "R", ":%s///g<Left><Left><Left>", ff)
-- }}}

-- 🤯 Comment headers {{{
map("n", "HS", '<leader>o<leader>O:.! figlet -fshadow <CR> gcip6j2"_dd', ft)
map("n", "HC", ":.! figlet -fcalvin <CR> gc2j2j", ft)
-- }}}

-- 🙏 Folds{{{
map("n", "<leader>zf", "vapzf")
map("n", "zh", "ggO<Esc>76i=<Esc>o<Esc>76i=<Esc>gck", ff)
-- }}}

-- 🔤 Spelling{{{
map("i", "<C-l>", "<c-g>u<Esc>[s1z=`]a<c-g>u", ft)
-- }}}

-- 🥂 Miscellaneous {{{
map("n", "gx", [[:silent execute '!xdg-open ' . shellescape(expand('<cfile>'), v:true)<CR>]], ft)
-- }}}
