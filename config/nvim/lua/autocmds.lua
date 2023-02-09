local function global_autocmd(event, command)
	vim.api.nvim_create_autocmd(event, {
		group = vim.api.nvim_create_augroup(event, { clear = true }),
		command = command,
		pattern = "*",
	})
end

local function scoped_autocmd(event, group, pattern, command)
	vim.api.nvim_create_autocmd(event, {
		group = vim.api.nvim_create_augroup(group, { clear = true }),
		command = command,
		pattern = pattern,
	})
end

-- 'formatoptions' keeps getting overwritten by other plugins (I think)
global_autocmd("InsertEnter", "silent! set formatoptions=tcqnpj1")

-- Why is highlighting text yanking not an default feature?
global_autocmd("TextYankPost", "silent! lua vim.highlight.on_yank({ timeout = 69})")

-- Remove color column when entering help docs
scoped_autocmd("FileType", "HelpDocs", "help", "silent! setlocal cc=")

scoped_autocmd("BufWritePost", "EwwRestart", "eww.yuck,eww.scss", ":! eww-open & disown && exit<CR>")

scoped_autocmd(
	"BufWritePost",
	"DunstRestart",
	"dunstrc",
	":! pkill dunst && dunst & notify-send -u low dunst restarted"
)
