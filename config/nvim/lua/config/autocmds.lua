local augroup = vim.api.nvim_create_augroup
local autocmd = vim.api.nvim_create_autocmd

local function au(event, opts)
	local group = opts.group or event
	if type(group) == "string" then
		group = augroup(group, { clear = true })
	end
	opts.group = group
	autocmd(event, opts)
end

-- ftplugins override formatoptions, vim.schedule defers until after they run
-- c: auto-wrap comments
-- q: format with gq
-- n: numbered lists
-- j: remove comment leader on join
-- r: insert comment leader on Enter
-- l: don't break long lines
-- 1: don't break after one-letter word
au("FileType", {
	group = "FormatOptions",
	callback = function()
		vim.schedule(function()
			vim.opt_local.formatoptions = "cqnjrl1"
		end)
	end,
})

-- two-space indent for heaivly indeneted languages
au("FileType", {
	group = "TwoSpaceIndent",
	pattern = { "lua", "yaml", "json", "jsonc", "html", "css", "scss", "markdown", "typescript", "javascript" },
	callback = function()
		vim.opt_local.tabstop = 2
		vim.opt_local.shiftwidth = 2
	end,
})

au("TextYankPost", {
	group = "HighlightYank",
	callback = function()
		vim.highlight.on_yank({ timeout = 69 })
	end,
})

-- restore cursor to last position when opening a buffer
au("BufReadPost", {
	group = "RestoreCursor",
	callback = function(args)
		local mark = vim.api.nvim_buf_get_mark(args.buf, '"')
		local line_count = vim.api.nvim_buf_line_count(args.buf)
		if mark[1] > 0 and mark[1] <= line_count then
			pcall(vim.api.nvim_win_set_cursor, 0, mark)
		end
	end,
})

-- auto-resize splits when terminal window is resized
au("VimResized", {
	group = "AutoResizeSplits",
	callback = function()
		vim.cmd("tabdo wincmd =")
	end,
})

-- check if file changed outside nvim and auto-reload (requires autoread option)
au({ "FocusGained", "BufEnter", "CursorHold", "TermClose", "TermLeave" }, {
	group = "CheckExternalChanges",
	callback = function()
		if vim.o.buftype ~= "nofile" then
			vim.cmd("checktime")
		end
	end,
})

-- reload external changes while preserving undo history (makes external edits undoable)
au("FileChangedShell", {
	group = "ForceReloadExternal",
	callback = function(args)
		local bufnr = args.buf
		local filename = vim.api.nvim_buf_get_name(bufnr)
		local ok, new_lines = pcall(vim.fn.readfile, filename)
		if not ok then
			vim.v.fcs_choice = "reload"
			return
		end
		vim.api.nvim_buf_set_lines(bufnr, 0, -1, false, new_lines)
		vim.bo[bufnr].modified = false
		vim.v.fcs_choice = ""
	end,
})

-- create parent directories when saving a file
au("BufWritePre", {
	group = "AutoCreateDir",
	callback = function(args)
		if args.match:match("^%w%w+:[\\/][\\/]") then
			return
		end
		local file = vim.uv.fs_realpath(args.match) or args.match
		vim.fn.mkdir(vim.fn.fnamemodify(file, ":p:h"), "p")
	end,
})

-- close these buffer types with just 'q'
au("FileType", {
	group = "CloseWithQ",
	pattern = { "help", "qf", "man", "notify", "lspinfo", "checkhealth", "query" },
	callback = function(args)
		vim.bo[args.buf].buflisted = false
		vim.keymap.set("n", "q", "<cmd>close<cr>", { buffer = args.buf, silent = true })
	end,
})

-- terminal: no line numbers, auto insert mode
au("TermOpen", {
	group = "TerminalSettings",
	callback = function()
		vim.opt_local.number = false
		vim.opt_local.relativenumber = false
		vim.opt_local.signcolumn = "no"
		vim.cmd("startinsert")
	end,
})

-- spell and wrap in prose-oriented filetypes
au("FileType", {
	group = "SpellCheck",
	pattern = { "markdown", "text", "gitcommit", "NeogitCommitMessage" },
	callback = function()
		vim.opt_local.spell = true
		vim.opt_local.wrap = true
	end,
})

local large_file_threshold = 1024 * 1024 -- 1MB

au("BufReadPre", {
	group = "LargeFile",
	callback = function(args)
		local file = args.match
		local size = vim.fn.getfsize(file)
		if size > large_file_threshold or size == -2 then
			vim.opt_local.swapfile = false
			vim.opt_local.foldmethod = "manual"
			vim.opt_local.undolevels = -1
			vim.opt_local.undoreload = 0
			vim.opt_local.list = false
			vim.b[args.buf].large_file = true
		end
	end,
})

au("BufReadPost", {
	group = "LargeFileSyntax",
	callback = function(args)
		if vim.b[args.buf].large_file then
			vim.cmd("syntax off")
		end
	end,
})

au("BufWritePost", {
	group = "EwwRestart",
	pattern = { "eww.yuck", "eww.scss" },
	command = ":silent !eww-open",
})

au("BufWritePost", {
	group = "DunstRestart",
	pattern = "dunstrc",
	command = ":silent !pkill dunst; dunst & dunstify -u low 'dunst restarted' 'config change detected'",
})

-- auto-clear search highlight due to inactivity
local search_timer = nil
local search_timeout = 2000 -- ms

local function clear_search_hl()
	if vim.v.hlsearch == 1 then
		vim.cmd("nohlsearch")
	end
end

local function reset_search_timer()
	if search_timer then
		search_timer:stop()
	end
	search_timer = vim.defer_fn(clear_search_hl, search_timeout)
end

-- expose for keymaps to use
_G.reset_search_timer = reset_search_timer

au("CmdlineLeave", {
	group = "AutoClearSearch",
	pattern = { "/", "?" },
	callback = reset_search_timer,
})

-- remove trailing whitespace on save
au("BufWritePre", {
	group = "TrimWhitespace",
	callback = function()
		local pos = vim.api.nvim_win_get_cursor(0)
		vim.cmd([[%s/\s\+$//e]])
		vim.api.nvim_win_set_cursor(0, pos)
	end,
})
