-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ helpers: one liner keymaps                                                  │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local function map(mode, lhs, rhs, opts)
	local keymap_opts = vim.tbl_extend("force", { silent = true }, opts or {})
	keymap_opts.icon = nil
	vim.keymap.set(mode, lhs, rhs, keymap_opts)
end

local function with_desc(description, extra)
	local o = { desc = description }

	if extra then
		for k, v in pairs(extra) do
			o[k] = v
		end
	end
	return o
end
local function desc(description)
	return with_desc(description)
end
local function remap(description)
	return with_desc(description, { remap = true })
end
local function remap_explicit(description)
	return with_desc(description, { remap = true, silent = false })
end
local function expr(description)
	return with_desc(description, { expr = true })
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ groups: secondary leader groups                                             │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local GROUPS = {
	{ "<leader>b", group = "Buffer Controls" },
	{ "<leader>c", group = "Code/Change" },
	{ "<leader>d", group = "Delete/Database" },
	{ "<leader>e", group = "Explorer" },
	{ "<leader>f", group = "Diagnostics" },
	{ "<leader>g", group = "Git/Goto" },
	{ "<leader>h", group = "Git Hunk", mode = { "n", "v" } },
	{ "<leader>m", group = "Markdown/Mason" },
	{ "<leader>n", group = "Harpoon" },
	{ "<leader>p", group = "Treesitter" },
	{ "<leader>r", group = "Replace/Rename" },
	{ "<leader>s", group = "Search/Save" },
	{ "<leader>t", group = "Telescope/Toggle" },
	{ "<leader>u", group = "UndO" },

	{ "g", group = "LSP" },
	{ "q", group = "Quit" },
}

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ save: write files, format, source                                           │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<C-s>", ":w<CR>", desc("Save"))
map("i", "<C-s>", "<Esc>:w<CR>", desc("Save"))
map("v", "<C-s>", "<Esc>:w<CR>", desc("Save"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ quit: exit, force quit, escape                                              │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "q:", "<Nop>")
map("n", "qq", ":q<CR>", desc("Quit"))
map("n", "<C-c>", "<Esc>", desc("Escape"))
map("n", "<leader>q", ":q!<CR>", remap_explicit("Force quit"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ copy: clipboard yank operations                                             │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("v", "<leader>y", 'ml"+y`l', desc("Yank to clipboard"))
map("v", "<C-c>", 'ml"+y`l', desc("Yank to clipboard"))
map("n", "<leader>y", '"+y', desc("Yank to clipboard"))
map("n", "<leader>Y", '"+y$', desc("Yank line to clipboard"))
map("n", "<leader>gy", 'mlgg"+yG`lzvzt', desc("Yank file to clipboard"))
map("n", "<leader>wd", "dt<space>", desc("Delete word"))
map("x", "<leader>p", '"_dP', desc("Paste (preserve register)"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ context: yank with file path prepended                                      │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local function yank_path(motion)
	if motion then
		vim.cmd("normal! " .. motion)
	end
	vim.cmd('normal! "+y')
	local selection = vim.fn.getreg("+")
	local path = vim.fn.expand("%:p")
	vim.fn.setreg("+", path .. "\n\n" .. selection .. "\n")
end
local function yank_paragrah(motion)
	return function()
		yank_path(motion)
	end
end
local function yank_diagnostics()
	local path = vim.fn.expand("%:p")
	local line = vim.fn.line(".")
	local diagnostics = vim.diagnostic.get(0, { lnum = line - 1 }) -- 0-indexed

	if #diagnostics == 0 then
		vim.fn.setreg("+", path .. ":" .. line .. "\n  |- (no diagnostics)")
		vim.notify("No diagnostics on this line", vim.log.levels.WARN)
		return
	end

	local lines = { path .. ":" .. line }
	for _, d in ipairs(diagnostics) do
		table.insert(lines, "  |- " .. d.message:gsub("\n", " "))
	end
	vim.fn.setreg("+", table.concat(lines, "\n"))
	vim.notify("Yanked " .. #diagnostics .. " diagnostic(s)", vim.log.levels.INFO)
end

map("n", "<A-f>", ':let @+=expand("%:p")<CR>', desc("Yank file path"))
map("v", "<A-c>", yank_path, desc("Yank file path + selection"))
map("n", "<A-g>", yank_paragrah("gv"), desc("Yank file path + selection (last visual)"))
map("n", "<A-p>", yank_paragrah("vap"), desc("Yank file path + paragraph"))
map("n", "<A-w>", yank_diagnostics, desc("Yank file path + diagnostics"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ space: add empty lines above/below                                          │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>o", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', desc("Add line below"))
map("n", "<leader>O", ':<C-u>call append(line(".")-1,   repeat([""], v:count1))<CR>', desc("Add line above"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ hold: no register, indent, move selection                                   │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
-- stylua: ignore start
map("n", "<leader>d", '"_d',                       desc("Delete (no register)"))
map("v", "<leader>d", '"_d',                       desc("Delete (no register)"))
map("n", "<leader>c", '"_c',                       desc("Change (no register)"))
map("v", "<leader>c", '"_c',                       desc("Change (no register)"))
map("n", "<leader>C", '"_C',                       desc("Change to EOL (no register)"))
map("n", "<leader>x", '"_x',                       desc("Delete char (no register)"))
map("n", "<leader>X", '"_X',                       desc("Backspace (no register)"))
map("v", "<",         "<gv",                       desc("Indent left"))
map("v", ">",         ">gv",                       desc("Indent right"))
map("v", "<Up>",      ":m '<-2<CR>gv-gv",          desc("Move selection up"))
map("v", "<Down>",    ":m '>+1<CR>gv-gv",          desc("Move selection down"))
map("n", "k",         "v:count == 0 ? 'gk' : 'k'", expr("Up (display line)"))
map("n", "j",         "v:count == 0 ? 'gj' : 'j'", expr("Down (display line)"))
-- stylua: ignore end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ search: clear highlights automatically                                      │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local function search_move(reverse)
	return function()
		local key = reverse and "N" or "n"
		local ok = pcall(vim.cmd.normal, { bang = true, args = { key } })
		if ok then
			vim.cmd.normal({ bang = true, args = { "zvzt" } })
			if _G.reset_search_timer then
				_G.reset_search_timer()
			end
		end
	end
end
map("n", "G", "Gzvzt", desc("Go to EOF"))
map("n", "n", search_move(false), desc("Next search result"))
map("n", "N", search_move(true), desc("Prev search result"))

local function star_search(key)
	return function()
		local ok = pcall(vim.cmd.normal, { bang = true, args = { key } })
		if ok then
			vim.cmd.normal({ bang = true, args = { "zvzt" } })
			if _G.reset_search_timer then
				_G.reset_search_timer()
			end
		end
	end
end
map("n", "*", star_search("*"), desc("Search word forward"))
map("n", "#", star_search("#"), desc("Search word backward"))
map("n", "g*", star_search("g*"), desc("Search word forward (partial)"))
map("n", "g#", star_search("g#"), desc("Search word backward (partial)"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ center: keep cursor centered on screen                                      │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
-- stylua: ignore start
map("n", "J",     "mzJ`z",     desc("Join lines"))
map("n", "<C-o>", "<C-o>zvzz", desc("Jump back"))
map("n", "<C-i>", "<C-i>zvzz", desc("Jump forward"))
map("n", "<C-f>", "<C-f>zz",   desc("Page down"))
map("n", "<C-b>", "<C-b>zz",   desc("Page up"))
map("n", "<C-d>", "<C-d>zz",   desc("Half page down"))
map("n", "<C-u>", "<C-u>zz",   desc("Half page up"))
map("n", "}", ':<C-u>execute "keepjumps norm! " . v:count1 . "}"<CR>zvzt', desc("Next paragraph"))
map("n", "{", ':<C-u>execute "keepjumps norm! " . v:count1 . "{"<CR>zvzt', desc("Prev paragraph"))
-- stylua: ignore end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ undo: insert mode break points at punctuation                               │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("i", ",", ",<C-g>u")
map("i", ".", ".<C-g>u")
map("i", "!", "!<C-g>u")
map("i", "?", "?<C-g>u")
map("i", ";", ";<C-g>u")
map("i", ":", ":<C-g>u")

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ window: navigation, tabs, resize, buffers                                   │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
-- stylua: ignore start
map("n", "<leader><C-o>",  ":bp<CR>zvzt",             desc("Previous buffer"))
map("n", "<leader><C-i>",  ":bn<CR>zvzt",             desc("Next buffer"))
map("n", "<leader>b<C-w>", ":bd!<CR>zvzt",            desc("Delete buffer"))
map("n", "<leader>pt",     ":InspectTree<CR>",        desc("Tree sitter inspect"))
map("n", "<leader>pc",     ":Inspect<CR>",            desc("TS inspect"))
map("n", "<Down>",         "<C-w>j",                  desc("Window down"))
map("n", "<Up>",           "<C-w>k",                  desc("Window up"))
map("n", "<Left>",         "<C-w>h",                  desc("Window left"))
map("n", "<Right>",        "<C-w>l",                  desc("Window right"))
map("n", "<PageUp>",       "gt",                      desc("Next tab"))
map("n", "<PageDown>",     "gT",                      desc("Prev tab"))
map("n", "<leader><C-t>",  "<C-w>T",                  desc("Move to new tab"))
map("n", "<C-Up>",         ":resize +2<CR>",          desc("Increase height"))
map("n", "<C-Down>",       ":resize -2<CR>",          desc("Decrease height"))
map("n", "<C-Left>",       ":vertical resize -2<CR>", desc("Decrease width"))
map("n", "<C-Right>",      ":vertical resize +2<CR>", desc("Increase width"))
-- stylua: ignore end
map("n", "gx", [[:silent execute '!xdg-open ' . shellescape(expand('<cfile>'), v:true)<CR>]], remap("Open URL"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ indent: format paragraph, file                                              │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>==", "ml=ip`lzvzt", desc("Indent paragraph"))
map("n", "<leader>g=", "mlgg=G`lzvzt", desc("Indent file"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ toggle: features, UI elements, settings                                     │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
-- stylua: ignore start
map("n", "<leader>mu", ":Lazy update<CR>",                         desc("Update plugins"))
map("n", "<leader>ut", ":UndotreeToggle<CR>",                      desc("Undo tree"))
map("n", "<leader>ct", ":HighlightColors Toggle<CR>",              desc("Toggle colors"))
map("n", "<leader>st", ":set spell!<CR>",                          desc("Toggle spell"))
map("n", "<leader>sc", ":let @/ = ''<CR>",                         desc("Clear search"))
map("n", "<leader>wt", ":set wrap!<CR> :echo 'wrap toggled'<CR>",  desc("Toggle wrap"))
map("n", "<leader>mt", ":MarkdownPreviewToggle<CR>,",              desc("Markdown preview"))
map("n", "<leader>et", ":NvimTreeToggle<CR> :NvimTreeRefresh<CR>", desc("Toggle file tree"))
map("n", "<leader>bt", ":Switch<CR>",                              desc("Toggle variant"))
-- stylua: ignore end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ replace: search and substitute patterns                                     │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("v", "r", ":s///g<Left><Left><Left>", remap_explicit("Replace in selection"))
map("n", "<leader>r<leader>", ":%s///g<Left><Left><Left>", remap_explicit("Replace {custom pattern} in file"))
map("n", "<leader>rw", ":%s/<C-r><C-w>//g<Left><Left>", remap_explicit("Replace word in file"))
map("n", "<leader>rp", '"ryiwvip:s/<C-r>r//g<Left><Left>', remap_explicit("Replace word in paragraph"))
map("n", "<leader>rs", "1z=", desc("Fix spelling"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ database: DB UI controls                                                    │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>db", ":DBUIToggle<CR>", desc("Toggle DB UI"))
map("n", "<leader>df", ":DBUIFindBuffer<CR>", desc("Find DB buffer"))

-- ╭─────────────────────────────────────────────────────────────────────────────
-- │ telescope: fuzzy finder pickers
-- ╰─────────────────────────────────────────────────────────────────────────────
local telescope_ok, t = pcall(require, "telescope.builtin")
if telescope_ok then
	local tmap = function(lhs, picker, d)
		map("n", lhs, picker, desc(d))
	end

  -- stylua: ignore start
	tmap("<leader>t<leader>", t.find_files,  "Find files")
	tmap("<leader>e<leader>", t.oldfiles,    "Recent files")
	tmap("<leader>s<leader>", t.live_grep,   "Live grep")
	tmap("<leader>g<leader>", t.grep_string, "Grep string under cursor")

	tmap("<leader>tg",  t.git_files,      "Git files")
	tmap("<leader>tb",  t.buffers,        "Buffers")
	tmap("<leader>th",  t.help_tags,      "Help tags")
	tmap("<leader>tH",  t.highlights,     "Highlight groups")
	tmap("<leader>td",  t.diagnostics,    "Diagnostics")
	tmap("<leader>tp",  t.builtin,        "Telescope builtins")
	tmap("<leader>tc",  t.commands,       "Commands")
	tmap("<leader>tl",  t.loclist,        "Location list")
	tmap("<leader>tq",  t.quickfix,       "Quickfix list")
	tmap("<leader>tm",  t.man_pages,      "Man pages")
	-- tmap("<leader>tj",  t.media_files,    "Media Files")
	tmap("<leader>tt",  t.resume,         "Resume last picker")
	tmap("<leader>t;",  t.marks,          "Marks")
	tmap("<leader>tst", t.treesitter,     "Treesitter symbols")
	tmap("<leader>tk",  t.keymaps,        "Keymaps")
	tmap("<leader>trg", t.registers,      "Registers")
	tmap("<leader>tco", t.colorscheme,    "Colorschemes")
	tmap("<leader>tj",  t.jumplist,       "Jumplist")
	tmap("<leader>tsh", t.search_history, "Search history")
	tmap("<leader>tsp", t.spell_suggest,  "Spelling suggestions")
	-- stylua: ignore end
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ harpoon: quick file navigation                                              │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

map("n", "<leader>nn", ":lua require('harpoon.mark').add_file()<CR>", desc("Add file"))
map("n", "<leader>ng", ":lua require('harpoon.ui').toggle_quick_menu()<CR>", desc("Quick menu"))
map("n", "<leader>nt", ":lua require('harpoon.ui').nav_file(1)<CR>zt", desc("File 1"))
map("n", "<leader>ne", ":lua require('harpoon.ui').nav_file(2)<CR>zt", desc("File 2"))
map("n", "<leader>ns", ":lua require('harpoon.ui').nav_file(3)<CR>zt", desc("File 3"))
map("n", "<leader>na", ":lua require('harpoon.ui').nav_file(4)<CR>zt", desc("File 4"))
map("n", "<leader>nd", ":lua require('harpoon.ui').nav_file(5)<CR>zt", desc("File 5"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ nvim-tree: file explorer (buffer-local)                                     │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local M = { groups = GROUPS }

M.nvimtree_on_attach = function(bufnr)
	local api = require("nvim-tree.api")
	local function map(lhs, rhs, d)
		vim.keymap.set("n", lhs, rhs, { desc = "nvim-tree: " .. d, buffer = bufnr, silent = true, nowait = true })
	end

  -- stylua: ignore start
	map("<CR>",    api.node.open.edit,               "Open")
	map("o",       api.node.open.edit,               "Open")
	map("<Right>", api.node.open.edit,               "Open")
	map("zz",      api.tree.change_root_to_node,     "CD")
	map("<Up>",    api.node.navigate.sibling.prev,   "Previous Sibling")
	map("<Down>",  api.node.navigate.sibling.next,   "Next Sibling")
	map("<Left>",  api.node.navigate.parent,         "Parent Directory")
	map("<C-v>",   api.node.open.vertical,           "Open: Vertical Split")
	map("<C-h>",   api.node.open.horizontal,         "Open: Horizontal Split")
	map("<C-t>",   api.node.open.tab,                "Open: New Tab")
	map("zc",      api.node.navigate.parent_close,   "Close Directory")
	map("I",       api.tree.toggle_gitignore_filter, "Toggle Git Ignore")
	map(".",       api.tree.toggle_hidden_filter,    "Toggle Dotfiles")
	map("n",       api.fs.create,                    "Create")
	map("d",       api.fs.trash,                     "Trash")
	map("X",       api.fs.remove,                    "Delete")
	map("r",       api.fs.rename,                    "Rename")
	map("<C-r>",   api.fs.rename_sub,                "Rename: Omit Filename")
	map("R",       api.tree.reload,                  "Refresh")
	map("<C-x>",   api.fs.cut,                       "Cut")
	map("yy",      api.fs.copy.node,                 "Copy")
	map("p",       api.fs.paste,                     "Paste")
	map("yp",      api.fs.copy.relative_path,        "Copy Relative Path")
	map("yP",      api.fs.copy.absolute_path,        "Copy Absolute Path")
	map("[",       api.node.navigate.git.prev,       "Prev Git")
	map("]",       api.node.navigate.git.next,       "Next Git")
	map("O",       api.node.run.system,              "Run System")
	map("q",       api.tree.close,                   "Close")
	map("<Esc>",   api.tree.close,                   "Close")
	map("?",       api.tree.toggle_help,             "Help")
	map("zm",      api.tree.collapse_all,            "Collapse")
	map("zr",      api.tree.expand_all,              "Expand All")
	map("S",       api.tree.search_node,             "Search")
	map("<C-k>",   api.node.show_info_popup,         "Info")
	-- stylua: ignore end
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ gitsigns: git hunk navigation and actions (buffer-local)                    │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local function nav_hunk(direction)
	return function()
		local gs = require("gitsigns")
		if vim.wo.diff then
			vim.cmd.normal({ direction == "next" and "]h" or "[h", bang = true })
		else
			gs.nav_hunk(direction)
		end
	end
end
local function stage_hunk_visual()
	require("gitsigns").stage_hunk({ vim.fn.line("."), vim.fn.line("v") })
end
local function reset_hunk_visual()
	require("gitsigns").reset_hunk({ vim.fn.line("."), vim.fn.line("v") })
end
local function blame_full()
	require("gitsigns").blame_line({ full = true })
end
local function diff_head()
	require("gitsigns").diffthis("~")
end

M.gitsigns_on_attach = function(bufnr)
	local gs = require("gitsigns")

	local function gsmap(mode, l, r, opts)
		opts = opts or {}
		opts.buffer = bufnr
		vim.keymap.set(mode, l, r, opts)
	end

	gsmap("n", "]h", nav_hunk("next"), { desc = "Next hunk" })
	gsmap("n", "[h", nav_hunk("prev"), { desc = "Previous hunk" })
	gsmap("n", "<leader>hs", gs.stage_hunk, { desc = "Stage hunk" })
	gsmap("n", "<leader>hr", gs.reset_hunk, { desc = "Reset hunk" })
	gsmap("v", "<leader>hs", stage_hunk_visual, { desc = "Stage hunk" })
	gsmap("v", "<leader>hr", reset_hunk_visual, { desc = "Reset hunk" })
	gsmap("n", "<leader>hS", gs.stage_buffer, { desc = "Stage buffer" })
	gsmap("n", "<leader>hu", gs.undo_stage_hunk, { desc = "Undo stage hunk" })
	gsmap("n", "<leader>hR", gs.reset_buffer, { desc = "Reset buffer" })
	gsmap("n", "<leader>hp", gs.preview_hunk, { desc = "Preview hunk" })
	gsmap("n", "<leader>hb", blame_full, { desc = "Blame line" })
	gsmap("n", "<leader>hB", gs.toggle_current_line_blame, { desc = "Toggle line blame" })
	gsmap("n", "<leader>hw", gs.toggle_word_diff, { desc = "Toggle word diff" })
	gsmap("n", "<leader>hl", gs.toggle_linehl, { desc = "Toggle line highlight" })
	gsmap("n", "<leader>hd", gs.toggle_deleted, { desc = "Toggle deleted" })
	gsmap("n", "<leader>hD", gs.diffthis, { desc = "Diff this" })
	gsmap("n", "<leader>hH", diff_head, { desc = "Diff against HEAD~" })
	gsmap({ "o", "x" }, "ih", ":<C-U>Gitsigns select_hunk<CR>", { desc = "Select hunk" })
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ lsp: language server protocol mappings                                      │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local function diag_jump(count)
	return function()
		vim.diagnostic.jump({ count = count })
	end
end

M.on_attach = function(event)
	local lspmap = function(keys, func, d, mode)
		mode = mode or "n"
		vim.keymap.set(mode, keys, func, { buffer = event.buf, desc = "LSP: " .. d })
	end
	local function toggle_inlay()
		vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled({ bufnr = event.buf }))
	end
	local function inc_rename()
		return ":IncRename " .. vim.fn.expand("<cword>")
	end

	local ts = require("telescope.builtin")

	lspmap("gd", vim.lsp.buf.definition, "Definition")
	lspmap("gD", vim.lsp.buf.declaration, "Declaration")
	lspmap("gi", vim.lsp.buf.implementation, "Implementation")
	lspmap("<F12>", ts.lsp_references, "References")
	lspmap("gt", ts.lsp_type_definitions, "Type Definition")
	lspmap("gO", ts.lsp_document_symbols, "Document Symbols")
	lspmap("gW", ts.lsp_dynamic_workspace_symbols, "Workspace Symbols")

	lspmap("K", vim.lsp.buf.hover, "Hover")
	lspmap("<C-k>", vim.lsp.buf.signature_help, "Signature Help")
	lspmap("<leader>k", vim.diagnostic.open_float, "Diagnostic Float")

	vim.keymap.set("n", "<f2>", inc_rename, { buffer = event.buf, desc = "LSP: Rename", expr = true })
	lspmap("<leader>ca", vim.lsp.buf.code_action, "Code Action")
	lspmap("<leader>cl", vim.lsp.codelens.run, "Code Lens")

	lspmap("<leader>ci", ts.lsp_incoming_calls, "Incoming Calls")
	lspmap("<leader>co", ts.lsp_outgoing_calls, "Outgoing Calls")

	lspmap("[d", diag_jump(-1), "Previous Diagnostic")
	lspmap("]d", diag_jump(1), "Next Diagnostic")

	lspmap("<leader>th", toggle_inlay, "Toggle Inlay Hints")
	lspmap("<leader>gg", "<cmd>LspRestart<CR>", "Restart LSP")

	local client = vim.lsp.get_client_by_id(event.data.client_id)
	if not client then
		return
	end

	if client:supports_method(vim.lsp.protocol.Methods.textDocument_codeLens, event.buf) then
		vim.api.nvim_create_autocmd({ "BufEnter", "InsertLeave" }, {
			buffer = event.buf,
			callback = vim.lsp.codelens.refresh,
		})
	end
end

M.ts_actions = function(event)
	local tsmap = function(keys, func, d)
		vim.keymap.set("n", keys, func, { buffer = event.buf, desc = "TS: " .. d })
	end
	local action = function(name)
		return function()
			vim.lsp.buf.code_action({ apply = true, context = { only = { name }, diagnostics = {} } })
		end
	end

	tsmap("<leader>oi", action("source.organizeImports.ts"), "Organize Imports")
	tsmap("<leader>ru", action("source.removeUnused.ts"), "Remove Unused")
	tsmap("<leader>am", action("source.addMissingImports.ts"), "Add Missing Imports")
	tsmap("<leader>fa", action("source.fixAll.ts"), "Fix All")
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ PLUGIN KEYS (lazy-loaded, defined in plugin specs)                          │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

-- ── trouble.nvim ──────────────────────────────────────────────────────────────
-- <leader>fd    Diagnostics (Trouble)
-- <leader>ft    Buffer Diagnostics (Trouble)
-- <leader>fs    Symbols (Trouble)
-- <leader>fp    LSP Definitions / references / ... (Trouble)
-- <leader>fl    Location List (Trouble)
-- <leader>fq    Quickfix List (Trouble)

-- ── nvim-dap ──────────────────────────────────────────────────────────────────
-- <F5>          Debug: Start/Continue
-- <F1>          Debug: Step Into
-- <F2>          Debug: Step Over
-- <F3>          Debug: Step Out
-- <leader>b     Debug: Toggle Breakpoint
-- <leader>B     Debug: Conditional Breakpoint
-- <F7>          Debug: Toggle UI

-- ── comment-box.nvim ──────────────────────────────────────────────────────────
-- gcb           Comment box (n, v)
-- gcq           Comment quote (n, v)
-- gcl           Comment line (n, v)

-- ── vim-easy-align ────────────────────────────────────────────────────────────
-- ga            Easy Align (n, x)

-- ── treesitter (incremental selection) ───────────────────────────────────────
-- <C-n>         Init/Node incremental selection
-- <C-p>         Node decremental
-- <C-l>         Scope incremental

-- ── treesitter textobjects (select) ───────────────────────────────────────────
-- af/if         Function outer/inner          ac/ic         Class outer/inner
-- aa/ia         Parameter outer/inner         aL/iL         Loop outer/inner
-- ae/iE         Conditional outer/inner       aB/iB         Block outer/inner

-- ── treesitter textobjects (move) ─────────────────────────────────────────────
-- ]f  ]F        Next function start/end       [f  [F        Prev function start/end
-- ]c  ]C        Next class start/end          [c  [C        Prev class start/end
-- ]p  ]P        Next parameter start/end      [p  [P        Prev parameter start/end
-- ]l  ]L        Next loop start/end           [l  [L        Prev loop start/end
-- ]e  ]E        Next conditional start/end    [e  [E        Prev conditional start/end
-- ]b  ]B        Next block start/end          [b  [B        Prev block start/end

-- ── treesitter textobjects (swap) ─────────────────────────────────────────────
-- <leader>ra    Swap parameter next           <leader>rA    Swap parameter prev
-- <leader>rf    Swap function next            <leader>rF    Swap function prev
-- <leader>rc    Swap class next               <leader>rC    Swap class prev

-- ── treesitter lsp interop (peek) ─────────────────────────────────────────────
-- <leader>hf    Peek function definition
-- <leader>hc    Peek class definition

return M
