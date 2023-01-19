local options = {
	-- 🔍 Search ==============================================================
    hlsearch = false,
    incsearch = true,
    ignorecase = true,
    smartcase = true,
    -- ========================================================================

    -- 📃 Text ================================================================
    display = { "lastline", "msgsep" },
    wrap = false,
    breakindent = true,
    linebreak = true,
    scrolloff = 22,
    sidescrolloff = 6,
    spell = false,
    spelllang = { "en_us" },
    formatoptions = "tcqnpj1",
	-- ========================================================================

    -- 👀 UI ==================================================================
    shortmess = "filnxtToOFcsS",
    laststatus = 2,
    showtabline = 0,
    number = true,
    relativenumber = true,
    termguicolors = true,
    signcolumn = "yes",
    colorcolumn = "80",
    foldmethod = "marker",
    updatetime = 420,
    -- ========================================================================

    -- ⌨️  UX ==================================================================
    tabstop = 4,
    wildmenu = true,
    wildmode = { "longest:full", "full" },
    mouse = "a",
    splitbelow = true,
    splitright = true,
    showmode = false,
    completeopt = { "menuone", "noselect", "noinsert" },
    iskeyword = "@,48-57,_,-,.,192-255",
    swapfile = false,
    undofile = true,
    wildignore = ".bak,~,.o,.h,.info,.swp,.obj,.pyc",
    -- ========================================================================
}

-- Zhu Li, do the thing!
for k, v in pairs(options) do
    vim.opt[k] = v
end
