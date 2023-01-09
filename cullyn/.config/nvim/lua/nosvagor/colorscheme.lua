vim.g.tokyonight_sidebars = { "undotree", "NvimTreee" }

local colorscheme = "tokyonight"

local status_ok, _ = pcall(vim.cmd, "colorscheme " .. colorscheme)
if not status_ok then
    vim.notify("colorscheme " .. colorscheme .. " not found!")
    return
end

vim.cmd([[
    highlight ColorColumn guibg=#23283b
    highlight helpExample guifg=#7aa2f7
    hi link   helpCommand Constant
    highlight MsgArea guifg=#565f89
    highlight Folded guibg=#24283b guifg=#565f89
]])
