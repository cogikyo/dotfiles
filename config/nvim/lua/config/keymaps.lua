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
	{ "<leader>b", group = "Buffer/Debug" },
	{ "<leader>c", group = "Code/Change" },
	{ "<leader>d", group = "Delete/Database" },
	{ "<leader>e", group = "Explorer" },
	{ "<leader>f", group = "Diagnostics" },
	{ "<leader>g", group = "General" },
	{ "<leader>h", group = "Git Hunk", mode = { "n", "v" } },
	{ "<leader>m", group = "Markdown/Manage" },
	{ "<leader>n", group = "Harpoon" },
	{ "<leader>p", group = "Treesitter" },
	{ "<leader>r", group = "Replace/Rename" },
	{ "<leader>s", group = "Search/Spell" },
	{ "<leader>t", group = "Telescope" },
	{ "<leader>u", group = "Undo" },

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
map("n", "<leader>mt", ":MarkdownPreviewToggle<CR>",               desc("Markdown preview"))
map("n", "<leader>et", function()
	local api = require("nvim-tree.api")
	local view = require("nvim-tree.view")
	if view.is_visible() then
		if vim.api.nvim_get_current_buf() == view.get_bufnr() then
			api.tree.close()
		else
			api.tree.focus()
		end
	else
		api.tree.open()
	end
end, desc("Toggle file tree"))
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
map("n", "<leader>Rs", ":LspRestart<CR>", desc("Restart LSP"))

return { groups = GROUPS }
