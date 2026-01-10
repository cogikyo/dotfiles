-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Helpers: one liner keymaps                                                  │
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
-- │ Groups: secondary leader groups                                             │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local GROUPS = {
	{ "<leader>a", group = "AI/Copilot" },
	{ "<leader>b", group = "Buffer Controls" },
	{ "<leader>c", group = "Code/Change" },
	{ "<leader>d", group = "Delete/Database" },
	{ "<leader>e", group = "Explorer" },
	{ "<leader>f", group = "Find/Trouble" },
	{ "<leader>g", group = "Git/Goto" },
	{ "<leader>h", group = "Git Hunk", mode = { "n", "v" } },
	{ "<leader>m", group = "Markdown/Mason" },
	{ "<leader>n", group = "Harpoon" },
	{ "<leader>p", group = "Treesitter" },
	{ "<leader>r", group = "Replace/Rename" },
	{ "<leader>s", group = "Search/Save" },
	{ "<leader>t", group = "Telescope/Toggle" },
	{ "<leader>u", group = "Undo/Update" },
	{ "gr", group = "LSP" },
	{ "z", group = "Folds" },
}

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Save: write files, format, source                                           │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<C-s>", ":w<CR>", desc("Save"))
map("i", "<C-s>", "<Esc>:w<CR>", desc("Save"))
map("v", "<C-s>", "<Esc>:w<CR>", desc("Save"))
map("n", "<leader>ss", ":noa w<CR><CR", desc("Save (no autocmd)"))
map("n", "<leader><C-s>", ":lua vim.lsp.buf.format()<CR><C-s>", desc("Format and save"))
map("n", "<leader>so", ":w | source %<CR>", desc("Save and source"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Quit: exit, force quit, escape                                              │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>q", ":q!<CR>", remap_explicit("Force quit"))
map("n", "qq", ":q<CR>", desc("Quit"))
map("n", "q:", "<Nop>")
map("n", "q", "<Nop>")
map("n", "<C-c>", "<Esc>", desc("Escape"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Vestigial: common editor shortcuts (undo, redo, paste)                      │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<C-z>", "u", desc("Undo"))
map("n", "<C-y>", "<C-r>", desc("Redo"))
map("i", "<C-v>", '<Esc>"+p', desc("Paste (clipboard)"))
map("i", "<C-t>", '<Esc>"*p', desc("Paste (selection)"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Copy: clipboard yank operations                                             │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("v", "<leader>y", 'ml"+y`l', desc("Yank to clipboard"))
map("v", "<C-c>", 'ml"+y`l', desc("Yank to clipboard"))
map("n", "<leader>y", '"+y', desc("Yank to clipboard"))
map("n", "<leader>Y", '"+y$', desc("Yank line to clipboard"))
map("n", "<leader>gy", 'mlgg"+yG`lzvzt', desc("Yank file to clipboard"))
map("n", "<leader>wd", "dt<space>", desc("Delete word"))
map("x", "<leader>p", '"_dP', desc("Paste (preserve register)"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Context: yank with file path prepended                                      │
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
map("n", "<A-f>", ':let @+=expand("%:p")<CR>', desc("Yank file path"))
map("v", "<A-c>", yank_path, desc("Yank file path + selection"))
map("n", "<A-g>", yank_paragrah("gv"), desc("Yank file path + selection (last visual)"))
map("n", "<A-p>", yank_paragrah("vap"), desc("Yank file path + paragraph"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Space: add empty lines above/below                                          │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>o", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', desc("Add line below"))
map("n", "<leader>O", ':<C-u>call append(line(".")-1,   repeat([""], v:count1))<CR>', desc("Add line above"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Hold: no register, indent, move selection                                   │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>d", '"_d', desc("Delete (no register)"))
map("v", "<leader>d", '"_d', desc("Delete (no register)"))
map("n", "<leader>c", '"_c', desc("Change (no register)"))
map("v", "<leader>c", '"_c', desc("Change (no register)"))
map("n", "<leader>C", '"_C', desc("Change to EOL (no register)"))
map("n", "<leader>x", '"_x', desc("Delete char (no register)"))
map("n", "<leader>X", '"_X', desc("Backspace (no register)"))
map("v", "<", "<gv", desc("Indent left"))
map("v", ">", ">gv", desc("Indent right"))
map("v", "<Up>", ":m '<-2<CR>gv-gv", desc("Move selection up"))
map("v", "<Down>", ":m '>+1<CR>gv-gv", desc("Move selection down"))
map("n", "k", "v:count == 0 ? 'gk' : 'k'", expr("Up (display line)"))
map("n", "j", "v:count == 0 ? 'gj' : 'j'", expr("Down (display line)"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Center: keep cursor centered on screen                                      │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local function search_move(reverse)
	return function()
		local key = reverse and "N" or "n"
		local ok = pcall(vim.cmd.normal, { bang = true, args = { key } })
		if ok then
			vim.cmd.normal({ bang = true, args = { "zvzt" } })
		end
	end
end
map("n", "G", "Gzvzt", desc("Go to EOF"))
map("n", "n", search_move(false), desc("Next search result"))
map("n", "N", search_move(true), desc("Prev search result"))
map("n", "}", ':<C-u>execute "keepjumps norm! " . v:count1 . "}"<CR>zvzt', desc("Next paragraph"))
map("n", "{", ':<C-u>execute "keepjumps norm! " . v:count1 . "{"<CR>zvzt', desc("Prev paragraph"))
map("n", "J", "mzJ`z", desc("Join lines"))
map("n", "<C-o>", "<C-o>zvzt", desc("Jump back"))
map("n", "<C-i>", "<C-i>zvzt", desc("Jump forward"))
map("n", "<C-f>", "<C-f>zt", desc("Page down"))
map("n", "<C-b>", "<C-b>zt", desc("Page up"))
map("n", "<C-d>", "<C-d>zt", desc("Half page down"))
map("n", "<C-u>", "<C-u>zt", desc("Half page up"))
map("n", "zm", "zmzt", desc("Fold more"))
map("n", "za", "zazt", desc("Toggle fold"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Undo: insert mode break points at punctuation                               │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("i", ",", ",<C-g>u")
map("i", ".", ".<C-g>u")
map("i", "!", "!<C-g>u")
map("i", "?", "?<C-g>u")
map("i", ";", ";<C-g>u")
map("i", ":", ":<C-g>u")

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Window: navigation, tabs, resize, buffers                                   │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader><C-o>", ":bp<CR>zvzt", desc("Previous buffer"))
map("n", "<leader><C-i>", ":bn<CR>zvzt", desc("Next buffer"))
map("n", "<leader>b<C-w>", ":bd!<CR>zvzt", desc("Delete buffer"))
map("n", "<leader>pt", ":InspectTree<CR>", desc("Tree sitter inspect"))
map("n", "<leader>pc", ":Inspect<CR>", desc("TS inspect"))
map("n", "<leader>pn", ":Inspect<CR>", desc("TS node under cursor"))
map("n", "<Down>", "<C-w>j", desc("Window down"))
map("n", "<Up>", "<C-w>k", desc("Window up"))
map("n", "<Left>", "<C-w>h", desc("Window left"))
map("n", "<Right>", "<C-w>l", desc("Window right"))
map("n", "<PageUp>", "gt", desc("Next tab"))
map("n", "<PageDown>", "gT", desc("Prev tab"))
map("n", "<leader><C-t>", "<C-w>T", desc("Move to new tab"))
map("n", "<C-Up>", ":resize +2<CR>", desc("Increase height"))
map("n", "<C-Down>", ":resize -2<CR>", desc("Decrease height"))
map("n", "<C-Left>", ":vertical resize -2<CR>", desc("Decrease width"))
map("n", "<C-Right>", ":vertical resize +2<CR>", desc("Increase width"))
map("n", "gx", [[:silent execute '!xdg-open ' . shellescape(expand('<cfile>'), v:true)<CR>]], remap("Open URL"))

local function open_doc_link()
	vim.lsp.buf.hover()
	vim.defer_fn(function()
		for _, win in ipairs(vim.api.nvim_list_wins()) do
			if vim.api.nvim_win_get_config(win).relative ~= "" then
				local buf = vim.api.nvim_win_get_buf(win)
				local lines = vim.api.nvim_buf_get_lines(buf, 0, -1, false)
				for i = #lines, 1, -1 do
					local url = lines[i]:match("https?://[%w%.%-%_~:/?#%[%]@!$&'%(%)%*%+,;=%%]+")
					if url then
						url = url:gsub("[%)%]>]+$", "")
						vim.fn.jobstart({ "xdg-open", url }, { detach = true })
						vim.api.nvim_win_close(win, true)
						return
					end
				end
			end
		end
	end, 100)
end

map("n", "<leader>gk", open_doc_link, desc("Open LSP doc link"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Indent: format paragraph, file                                              │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>==", "ml=ip`lzvzt", desc("Indent paragraph"))
map("n", "<leader>g=", "mlgg=G`lzvzt", desc("Indent file"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Toggle: features, UI elements, settings                                     │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>ut", ":UndotreeToggle<CR>", desc("Undo tree"))
map("n", "<leader>up", ":Lazy update<CR>", desc("Update plugins"))
map("n", "<leader>ct", ":HighlightColors Toggle<CR>", desc("Toggle colors"))
map("n", "<leader>st", ":set spell!<CR>", desc("Toggle spell"))
map("n", "<leader>sc", ":let @/ = ''<CR>", desc("Clear search"))
map("n", "<leader>wt", ":set wrap!<CR> :echo 'wrap toggled'<CR>", desc("Toggle wrap"))
map("n", "<leader>mt", ":MarkdownPreviewToggle<CR>,", desc("Markdown preview"))
map("n", "<leader>et", ":NvimTreeToggle<CR> :NvimTreeRefresh<CR>", desc("Toggle file tree"))
map("n", "<leader>bt", ":Switch<CR>", desc("Toggle variant"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Copilot: AI assistant controls                                              │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>at", ":Copilot toggle<CR>", desc("Toggle Copilot"))
map("n", "<leader>as", ":Copilot status<CR>", desc("Copilot status"))
map("n", "<leader>ap", ":Copilot panel<CR>", desc("Copilot panel"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Replace: search and substitute patterns                                     │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("v", "r", ":s///g<Left><Left><Left>", remap_explicit("Replace in selection"))
map("n", "<leader>r<leader>", ":%s///g<Left><Left><Left>", remap_explicit("Replace {custom pattern} in file"))
map("n", "<leader>rw", ":%s/<C-r><C-w>//g<Left><Left>", remap_explicit("Replace word in file"))
map("n", "<leader>rp", '"ryiwvip:s/<C-r>r//g<Left><Left>', remap_explicit("Replace word in paragraph"))
map("n", "<leader>rs", "1z=", desc("Fix spelling"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Database: DB UI controls                                                    │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>db", ":DBUIToggle<CR>", desc("Toggle DB UI"))
map("n", "<leader>df", ":DBUIFindBuffer<CR>", desc("Find DB buffer"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Telescope: fuzzy finder pickers                                             │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local telescope_ok, telescope_builtin = pcall(require, "telescope.builtin")
if telescope_ok then
	local tmap = function(lhs, picker, d)
		map("n", lhs, picker, desc(d))
	end

	tmap("<leader>t<leader>", telescope_builtin.find_files, "Find files")
	tmap("<leader>e<leader>", telescope_builtin.oldfiles, "Recent files")
	tmap("<leader>s<leader>", telescope_builtin.live_grep, "Live grep")
	tmap("<leader>g<leader>", telescope_builtin.grep_string, "Grep string under cursor")

	tmap("<leader>tg", telescope_builtin.git_files, "Git files")
	tmap("<leader>tb", telescope_builtin.buffers, "Buffers")
	tmap("<leader>th", telescope_builtin.help_tags, "Help tags")
	tmap("<leader>tH", telescope_builtin.highlights, "Highlight groups")
	tmap("<leader>td", telescope_builtin.diagnostics, "Diagnostics")
	tmap("<leader>tp", telescope_builtin.builtin, "Telescope builtins")
	tmap("<leader>tc", telescope_builtin.commands, "Commands")
	tmap("<leader>tl", telescope_builtin.loclist, "Location list")
	tmap("<leader>tq", telescope_builtin.quickfix, "Quickfix list")
	tmap("<leader>tm", telescope_builtin.man_pages, "Man pages")
	tmap("<leader>tt", telescope_builtin.resume, "Resume last picker")
	tmap("<leader>tf", telescope_builtin.current_buffer_fuzzy_find, "Find in this file")
	tmap("<leader>t;", telescope_builtin.marks, "Marks")
	tmap("<leader>tst", telescope_builtin.treesitter, "Treesitter symbols")
	tmap("<leader>tk", telescope_builtin.keymaps, "Keymaps")
	tmap("<leader>trg", telescope_builtin.registers, "Registers")
	tmap("<leader>tco", telescope_builtin.colorscheme, "Colorschemes")
	tmap("<leader>tj", telescope_builtin.jumplist, "Jumplist")
	tmap("<leader>tsh", telescope_builtin.search_history, "Search history")
	tmap("<leader>tsp", telescope_builtin.spell_suggest, "Spelling suggestions")

	tmap("<leader>tsr", telescope_builtin.lsp_references, "LSP references")
	tmap("<leader>tss", telescope_builtin.lsp_document_symbols, "LSP document symbols")
	tmap("<leader>tsw", telescope_builtin.lsp_dynamic_workspace_symbols, "LSP workspace symbols")
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ Harpoon: quick file navigation                                              │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
map("n", "<leader>nn", ":lua require('harpoon.mark').add_file()<CR>", desc("Add file"))
map("n", "<leader>ng", ":lua require('harpoon.ui').toggle_quick_menu()<CR>", desc("Quick menu"))
map("n", "<leader>nt", ":lua require('harpoon.ui').nav_file(1)<CR>zt", desc("File 1"))
map("n", "<leader>ne", ":lua require('harpoon.ui').nav_file(2)<CR>zt", desc("File 2"))
map("n", "<leader>ns", ":lua require('harpoon.ui').nav_file(3)<CR>zt", desc("File 3"))
map("n", "<leader>na", ":lua require('harpoon.ui').nav_file(4)<CR>zt", desc("File 4"))
map("n", "<leader>nd", ":lua require('harpoon.ui').nav_file(5)<CR>zt", desc("File 5"))

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ LSP: language server protocol mappings                                      │
-- ╰─────────────────────────────────────────────────────────────────────────────╯
local M = { groups = GROUPS }

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
	lspmap("gr", ts.lsp_references, "References")
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

return M
