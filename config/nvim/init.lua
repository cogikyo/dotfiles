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
}
for _, file in ipairs(config_files) do
    require("user."..file)
end
