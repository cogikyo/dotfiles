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

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ formatting: indent, options, whitespace                                    │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

-- vim.schedule defers until after ftplugins override formatoptions
au("FileType", {
	group = "FormatOptions",
	callback = function()
		vim.schedule(function()
			-- q:gq n:numbered-lists j:join-leader r:enter-leader l:no-break 1:no-break-after-1-char
			vim.opt_local.formatoptions = "qnjrl1"
		end)
	end,
})

au("FileType", {
	group = "TwoSpaceIndent",
	pattern = { "lua", "yaml", "json", "jsonc", "html", "css", "scss", "markdown", "typescript", "javascript" },
	callback = function()
		vim.opt_local.tabstop = 2
		vim.opt_local.shiftwidth = 2
	end,
})

au("FileType", {
	group = "MarkdownNoAutoWrap",
	pattern = "markdown",
	callback = function()
		vim.opt_local.textwidth = 0
		vim.opt_local.formatoptions:remove({ "t", "a" })
	end,
})

au("BufWritePre", {
	group = "TrimWhitespace",
	callback = function()
		local pos = vim.api.nvim_win_get_cursor(0)
		vim.cmd([[%s/\s\+$//e]])
		vim.api.nvim_win_set_cursor(0, pos)
	end,
})

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ yank: highlight feedback                                                   │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

au("TextYankPost", {
	group = "HighlightYank",
	callback = function()
		if _G.context_yank then
			vim.highlight.on_yank({ higroup = "GitSignsChangeLn", timeout = 100 })
			_G.context_yank = false
		else
			vim.highlight.on_yank({ timeout = 69 })
		end
	end,
})

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ restore: cursor, splits, external changes                                  │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

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

au("VimResized", {
	group = "AutoResizeSplits",
	callback = function()
		vim.cmd("tabdo wincmd =")
	end,
})

au({ "FocusGained", "BufEnter", "CursorHold", "TermClose", "TermLeave" }, {
	group = "CheckExternalChanges",
	callback = function()
		if vim.o.buftype ~= "nofile" then
			vim.cmd("checktime")
		end
	end,
})

-- Replays external edits as buffer changes so they appear in undo history
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

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ buftype: close-with-q, terminal, large files                               │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

au("FileType", {
	group = "CloseWithQ",
	pattern = { "help", "qf", "man", "notify", "lspinfo", "checkhealth", "query" },
	callback = function(args)
		vim.bo[args.buf].buflisted = false
		vim.keymap.set("n", "q", "<cmd>close<cr>", { buffer = args.buf, silent = true })
	end,
})

au("TermOpen", {
	group = "TerminalSettings",
	callback = function()
		vim.opt_local.number = false
		vim.opt_local.relativenumber = false
		vim.opt_local.signcolumn = "no"
		vim.cmd("startinsert")
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

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ reload: restart services on config save                                    │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

au("BufWritePost", {
	group = "EwwRestart",
	pattern = { "eww.yuck", "eww.scss" },
	command = ":silent !ewwd open",
})

au("BufWritePost", {
	group = "DunstRestart",
	pattern = "dunstrc",
	command = ":silent !pkill dunst; dunst & dunstify -u low 'dunst restarted' 'config change detected'",
})

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ search: auto-clear highlights after timeout                                │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

local search_timer = nil
local search_timeout = 2000

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

_G.reset_search_timer = reset_search_timer

au("CmdlineLeave", {
	group = "AutoClearSearch",
	pattern = { "/", "?" },
	callback = reset_search_timer,
})
