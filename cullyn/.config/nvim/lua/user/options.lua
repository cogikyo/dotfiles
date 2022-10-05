local options = {
    -- 👉 Indentation =========================================================
    autoindent = true,
    smartindent = true,
    expandtab = true,
    shiftround = true,
    smarttab = true,
    shiftwidth = 4,
    softtabstop = 4,
    tabstop = 4,
    -- ========================================================================

    -- 🔍 Search ==============================================================
    hlsearch = false,
    incsearch = true,
    ignorecase = false,
    smartcase = false,
    -- ========================================================================

    -- 📃 Text ================================================================
    display = { "lastline", "msgsep" },
    wrap = false,
    linebreak = true,
    scrolloff = 22,
    sidescrolloff = 6,
    spell = false,
    spelllang = { "en_us" },
    -- ========================================================================

    -- 👀 UI ==================================================================
    shortmess = "filnxtToOFcsS",
    laststatus = 2,
    showtabline = 0,
    number = true,
    relativenumber = true,
    cursorline = true,
    termguicolors = true,
    signcolumn = "yes",
    colorcolumn = { "80", "87", "100" },
    foldmethod = "marker",
    -- ========================================================================

    -- ⌨️  UX ==================================================================
    wildmenu = true,
    wildmode = { "longest:full", "full" },
    mouse = "a",
    updatetime = 1420,
    splitbelow = true,
    splitright = true,
    showmode = false,
    completeopt = { "menuone", "noselect", "noinsert" },
    iskeyword = "@,48-57,_,-,192-255",
    -- ========================================================================

    -- 🥷 Miscellaneous =======================================================
    backup = false,
    swapfile = false,
    undofile = true,
    wildignore = ".bak,~,.o,.h,.info,.swp,.obj,.pyc",
    -- ========================================================================
}

vim.cmd([[
set undodir=~/.local/share/nvim/undodir
]]) -- undotree needs this 

for k, v in pairs(options) do
    vim.opt[k] = v
end
