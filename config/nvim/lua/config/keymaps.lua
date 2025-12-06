--- 🔧 helpers ---
local function map(mode, lhs, rhs, opts)
	local keymap_opts = vim.tbl_extend("force", { silent = true }, opts or {})
	keymap_opts.icon = nil
	vim.keymap.set(mode, lhs, rhs, keymap_opts)
end

local function with_desc(desc, extra)
	local o = { desc = desc }

	if extra then
		for k, v in pairs(extra) do
			o[k] = v
		end
	end
	return o
end
local function desc(desc)
	return with_desc(desc)
end
local function remap(desc)
	return with_desc(desc, { remap = true })
end
local function remap_explicit(desc)
	return with_desc(desc, { remap = true, silent = false })
end
local function expr(desc)
	return with_desc(desc, { expr = true })
end

local GROUPS = {
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

--- 💾 Save ---
map("n", "<C-s>", ":w<CR>", desc("Save"))
map("i", "<C-s>", "<Esc>:w<CR>", desc("Save"))
map("v", "<C-s>", "<Esc>:w<CR>", desc("Save"))
map("n", "<leader>ss", ":noa w<CR><CR", desc("Save (no autocmd)"))
map("n", "<leader><C-s>", ":lua vim.lsp.buf.format()<CR><C-s>", desc("Format and save"))
map(
	"v",
	"<leader><C-s>",
	":<C-u>lua vim.lsp.buf.format({ range = { start = vim.api.nvim_buf_get_mark(0, '<'), ['end'] = vim.api.nvim_buf_get_mark(0, '>') } })<CR>",
	desc("Format selection")
)
map("n", "<leader>so", ":w | source %<CR>", desc("Save and source"))

--- 🔔 Quit ---
map("n", "<leader>q", ":q!<CR>", remap_explicit("Force quit"))
map("n", "qq", ":q<CR>", desc("Quit"))
map("n", "q:", "<Nop>")
map("n", "<C-c>", "<Esc>", desc("Escape"))

--- 🍸 Vestigial ---
map("n", "<C-z>", "u", desc("Undo"))
map("n", "<C-y>", "<C-r>", desc("Redo"))
map("i", "<C-v>", '<Esc>"+p', desc("Paste (clipboard)"))
map("i", "<C-t>", '<Esc>"*p', desc("Paste (selection)"))

--- 🤖 Copy copy ---
map("v", "<leader>y", 'ml"+y`l', desc("Yank to clipboard"))
map("v", "<C-c>", 'ml"+y`l', desc("Yank to clipboard"))
map("n", "<leader>y", '"+y', desc("Yank to clipboard"))
map("n", "<leader>Y", '"+y$', desc("Yank line to clipboard"))
map("n", "<leader>gy", 'mlgg"+yG`lzvzt', desc("Yank file to clipboard"))
map("n", "<leader>wd", "dt<space>", desc("Delete word"))
map("x", "<leader>p", '"_dP', desc("Paste (preserve register)"))

--- 🌌 Gimme space please ---
map("n", "<leader>o", ':<C-u>call append(line("."),   repeat([""], v:count1))<CR>', desc("Add line below"))
map("n", "<leader>O", ':<C-u>call append(line(".")-1,   repeat([""], v:count1))<CR>', desc("Add line above"))
map("n", "<leader>a", "<leader>o<leader>O", remap("Add lines around"))

--- 💎 Don't let go ---
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

--- 🎯 Keep cursor 'centered' ---
local function search_move(reverse)
	local key = reverse and "N" or "n"
	local ok = pcall(vim.cmd.normal, { bang = true, args = { key } })
	if ok then
		vim.cmd.normal({ bang = true, args = { "zvzt" } })
	end
end
map("n", "G", "Gzvzt", desc("Go to EOF"))
map("n", "n", function()
	search_move(false)
end, desc("Next search result"))
map("n", "N", function()
	search_move(true)
end, desc("Prev search result"))
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

--- 👈 Undo break points ---
map("i", ",", ",<C-g>u")
map("i", ".", ".<C-g>u")
map("i", "!", "!<C-g>u")
map("i", "?", "?<C-g>u")
map("i", ";", ";<C-g>u")
map("i", ":", ":<C-g>u")

--- 🪟 Window movement ---
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

--- 👉 Indent ---
map("n", "<leader>==", "ml=ip`lzvzt", desc("Indent paragraph"))
map("n", "<leader>g=", "mlgg=G`lzvzt", desc("Indent file"))

--- 🤲 Toggle ---
map("n", "<leader>ut", ":UndotreeToggle<CR>", desc("Undo tree"))
map("n", "<leader>up", ":Lazy update<CR>", desc("Update plugins"))
map("n", "<leader>ct", ":HighlightColors Toggle<CR>", desc("Toggle colors"))
map("n", "<leader>st", ":set spell!<CR>", desc("Toggle spell"))
map("n", "<leader>sc", ":let @/ = ''<CR>", desc("Clear search"))
map("n", "<leader>wt", ":set wrap!<CR> :echo 'wrap toggled'<CR>", desc("Toggle wrap"))
map("n", "<leader>mt", ":MarkdownPreviewToggle<CR>,", desc("Markdown preview"))
map("n", "<leader>et", ":NvimTreeToggle<CR> :NvimTreeRefresh<CR>", desc("Toggle file tree"))
map("n", "<leader>bt", ":Switch<CR>", desc("Toggle variant"))

--- 🔍 Replace ---
map("v", "r", ":s///g<Left><Left><Left>", remap_explicit("Replace in selection"))
map("n", "<leader>r<leader>", ":%s///g<Left><Left><Left>", remap_explicit("Replace {custom pattern} in file"))
map("n", "<leader>rw", ":%s/<C-r><C-w>//g<Left><Left>", remap_explicit("Replace word in file"))
map("n", "<leader>rp", '"ryiwvip:s/<C-r>r//g<Left><Left>', remap_explicit("Replace word in paragraph"))
map("n", "<leader>rs", "1z=", desc("Fix spelling"))

--- 💾 Database ---
map("n", "<leader>db", ":DBUIToggle<CR>", desc("Toggle DB UI"))
map("n", "<leader>df", ":DBUIFindBuffer<CR>", desc("Find DB buffer"))

--- 🔭 Telescope ---
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

	-- LSP-only pickers (require attached server)
	tmap("<leader>tsr", telescope_builtin.lsp_references, "LSP references", "󰌹", "purple")
	tmap("<leader>tss", telescope_builtin.lsp_document_symbols, "LSP document symbols", "󰙅", "cyan")
	tmap("<leader>tsw", telescope_builtin.lsp_dynamic_workspace_symbols, "LSP workspace symbols", "󰙅", "cyan")
end

--- 🔱 Harpoon ---
map("n", "<leader>nn", ":lua require('harpoon.mark').add_file()<CR>", desc("Add file"))
map("n", "<leader>ng", ":lua require('harpoon.ui').toggle_quick_menu()<CR>", desc("Quick menu"))
map("n", "<leader>nt", ":lua require('harpoon.ui').nav_file(1)<CR>zt", desc("File 1"))
map("n", "<leader>ne", ":lua require('harpoon.ui').nav_file(2)<CR>zt", desc("File 2"))
map("n", "<leader>ns", ":lua require('harpoon.ui').nav_file(3)<CR>zt", desc("File 3"))
map("n", "<leader>na", ":lua require('harpoon.ui').nav_file(4)<CR>zt", desc("File 4"))
map("n", "<leader>nd", ":lua require('harpoon.ui').nav_file(5)<CR>zt", desc("File 5"))

--- 🔌 LSP ---
local M = { groups = GROUPS }

M.on_attach = function(event)
	local bmap = function(keys, func, d, mode)
		mode = mode or "n"
		vim.keymap.set(mode, keys, func, { buffer = event.buf, desc = "LSP: " .. d })
	end

	local ts = require("telescope.builtin")

	bmap("gd", vim.lsp.buf.definition, "Definition")
	bmap("gD", vim.lsp.buf.declaration, "Declaration")
	bmap("gi", vim.lsp.buf.implementation, "Implementation")
	bmap("gr", ts.lsp_references, "References")
	bmap("gt", ts.lsp_type_definitions, "Type Definition")
	bmap("gO", ts.lsp_document_symbols, "Document Symbols")
	bmap("gW", ts.lsp_dynamic_workspace_symbols, "Workspace Symbols")

	bmap("K", vim.lsp.buf.hover, "Hover")
	bmap("<C-k>", vim.lsp.buf.signature_help, "Signature Help")
	bmap("<leader>k", vim.diagnostic.open_float, "Diagnostic Float")

	bmap("<leader>rn", vim.lsp.buf.rename, "Rename")
	bmap("<leader>ca", vim.lsp.buf.code_action, "Code Action")
	bmap("<leader>cl", vim.lsp.codelens.run, "Code Lens")

	bmap("<leader>ci", ts.lsp_incoming_calls, "Incoming Calls")
	bmap("<leader>co", ts.lsp_outgoing_calls, "Outgoing Calls")

	bmap("[d", function()
		vim.diagnostic.jump({ count = -1 })
	end, "Previous Diagnostic")
	bmap("]d", function()
		vim.diagnostic.jump({ count = 1 })
	end, "Next Diagnostic")

	bmap("<leader>th", function()
		vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled({ bufnr = event.buf }))
	end, "Toggle Inlay Hints")

	bmap("<leader>gg", "<cmd>LspRestart<CR>", "Restart LSP")

	local client = vim.lsp.get_client_by_id(event.data.client_id)
	if not client then
		return
	end

	if client:supports_method(vim.lsp.protocol.Methods.textDocument_documentHighlight, event.buf) then
		local hl_group = vim.api.nvim_create_augroup("lsp-highlight", { clear = false })
		vim.api.nvim_create_autocmd({ "CursorHold", "CursorHoldI" }, {
			buffer = event.buf,
			group = hl_group,
			callback = vim.lsp.buf.document_highlight,
		})
		vim.api.nvim_create_autocmd({ "CursorMoved", "CursorMovedI" }, {
			buffer = event.buf,
			group = hl_group,
			callback = vim.lsp.buf.clear_references,
		})
		vim.api.nvim_create_autocmd("LspDetach", {
			group = vim.api.nvim_create_augroup("lsp-detach", { clear = true }),
			callback = function(e)
				vim.lsp.buf.clear_references()
				vim.api.nvim_clear_autocmds({ group = "lsp-highlight", buffer = e.buf })
			end,
		})
	end

	if client:supports_method(vim.lsp.protocol.Methods.textDocument_codeLens, event.buf) then
		vim.api.nvim_create_autocmd({ "BufEnter", "CursorHold", "InsertLeave" }, {
			buffer = event.buf,
			callback = vim.lsp.codelens.refresh,
		})
	end
end

M.ts_actions = function(event)
	local bmap = function(keys, func, d)
		vim.keymap.set("n", keys, func, { buffer = event.buf, desc = "TS: " .. d })
	end

	local action = function(name)
		return function()
			vim.lsp.buf.code_action({
				apply = true,
				context = { only = { name }, diagnostics = {} },
			})
		end
	end

	bmap("<leader>oi", action("source.organizeImports.ts"), "Organize Imports")
	bmap("<leader>ru", action("source.removeUnused.ts"), "Remove Unused")
	bmap("<leader>am", action("source.addMissingImports.ts"), "Add Missing Imports")
	bmap("<leader>fa", action("source.fixAll.ts"), "Fix All")
end

return M

