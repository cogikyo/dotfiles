-- load 'lewis6991/impatient.nvim', or silenlty fail
local status_ok, _ = pcall(require, "impatient")
if not status_ok then
    return
end -- ⮬ should only fail if packer is not yet installed



-- organized config files ⮯
local config_files = {
    -- ============================================================================
    "settings", -- adjust built in options/settings
    "keymaps", -- custom remaps, does not include plugins specific keymaps
    "autocmds", -- listeners, or other custom automatic functions
    "plugins", -- includes some basic plugin settings
    -- ============================================================================
    -- plugins with required setups (or major changes) are given their own file ⮯
    "cmp",
    "mason",
    "colorscheme",
    "telescope",
    "treesitter",
    "refactoring",
    "autopairs",
    "comment",
    "gitsigns",
    "nvim-tree",
    "toggleterm",
    "lualine",
    "alpha",
}

-- files are located under lua/{user} (e.g., nosvagor)
for _, file in ipairs(config_files) do
    require("nosvagor." .. file)
end
