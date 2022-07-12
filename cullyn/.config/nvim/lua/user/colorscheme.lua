vim.g.tokyonight_sidebars = { "undotree", "NvimTreee" }

local colorscheme = "tokyonight"

local status_ok, _ = pcall(vim.cmd, "colorscheme " .. colorscheme)
if not status_ok then
	vim.notify("colorscheme " .. colorscheme .. " not found!")
	return
end

vim.cmd([[
highlight ColorColumn guibg=#22263a
highlight CursorLine guibg=#22263a
highlight helpExample guifg=#7aa2f7
hi link   helpCommand Constant
highlight MsgArea guifg=#565f89
]])
