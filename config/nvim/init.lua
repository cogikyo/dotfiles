local config_files = {
    "plugins",
    "options",
    "keymaps",
    "completions",
    "colorscheme",
    "lsp",
    "telescope",
    "treesitter",
    "autopairs",
    "comment",
    "gitsigns",
    "nvim-tree"
}
for _, file in ipairs(config_files) do
    require("user."..file)
end
