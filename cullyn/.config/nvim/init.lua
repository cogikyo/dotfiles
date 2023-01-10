-- load 'lewis6991/impatient.nvim', or silenlty fail
local status_ok, _ = pcall(require, "impatient")
if not status_ok then
    return
end -- ⮬ should only fail if packer is not yet installed


-- organized config files ⮯
local config_files = {
    -- ============================================================================
    "settings", -- adjust built-in options/settings
    "keymaps", -- custom remaps, some minor plugin-specific keymaps
    "autocmds", -- listeners, usually for filetypes or on pre/post save
    "plugins", -- all plugins, and includes some basic plugin settings
    -- ============================================================================
    -- plugins with major changes or extensive configs are given their own file ⮯
    "treesitter",
    "lsp",
    "cmp",
    "telescope",
    "colorscheme",
    "autopairs",
    "comment",
    "gitsigns",
    "nvim-tree",
    "toggleterm",
    "lualine",
    "alpha",
}

-- files are located under lua/{user} (e.g., nosvagor).
for _, file in ipairs(config_files) do
    require("nosvagor." .. file)
end
