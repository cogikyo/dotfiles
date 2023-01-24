-- see :help 'option' for more details ⮯
local options = {
    -- 🔍 Search ==============================================================
    hlsearch = false, -- turn off persistent highlights when searching
    incsearch = true, -- (default on) but can be annoying.
    ignorecase = true, -- ignore case when searching
    smartcase = true, -- ignore, ignorecase if a captial letter is used
    -- ========================================================================

    -- 👉 Indentation =========================================================
    tabstop = 4, -- preferred tabstop length.
    shiftwidth = 4, -- set to same as tabstop ⮭
    expandtab = true, -- as per option 2 in :help 'tabstop'
    shiftround = true, -- ensures indent is a multipe of shiftwidth
    smartindent = true, -- an extension of 'autoindent'
    -- ========================================================================

    -- 📃 Text ================================================================
    wrap = false, -- disable text wrapping by default
    breakindent = true, -- continue to indent text (if wrapped line)
    linebreak = true, -- smarter (more control) over line breaking; see 'breakat'
    scrolloff = 22, -- minimum lines above/below cursor
    sidescrolloff = 6, -- minimum lines left/right of cursor
    spell = false, -- turn spell check off by default
    spelllang = { "en_us", "en_gb" }, -- easily add/remove dictionaries
    formatoptions = "tcqnpj1", -- see :help fo-table
    -- ========================================================================

    -- 👀 UI ==================================================================
    shortmess = "filnxtToOFcsS", -- control some status line message settings
    showtabline = 0, -- I don't use tabs; hide tabs
    number = true, -- show line numbers
    relativenumber = true, -- makes using {number}{motion} easier
    termguicolors = true, -- more colors, good; needs terminal support.
    signcolumn = "yes", -- allays display sign-column
    colorcolumn = "80", -- reminder for the ideal line-length maximum
    foldmethod = "marker", -- atomically fold on {{{ }}}
    updatetime = 420, -- timer until events execute when cursors stops (ms)
    -- ========================================================================

    -- ⌨️  UX ==================================================================
    wildmenu = true, -- command-line completion
    wildmode = { "list:lastused", "full" }, -- completion method
    mouse = "a", -- mouse can be useful, sometimes... it doesn't hurt.
    splitbelow = true, -- split below by default
    splitright = true, -- split right by default
    showmode = false, -- redundant if using lualine
    completeopt = { "menuone", "noselect", "noinsert" }, -- insert complete options
    iskeyword = "@,48-57,_,-,192-255", -- define what a ends a "word"
    swapfile = false, -- swapfiles are useless to me
    undofile = true, -- persistent undo, very useful with 'mbbill/undotree' plugin
    wildignore = ".back,~,.o,.h,.info,.swp,.obj,.pyc", -- don't check these files for "*" pattern matching
    -- ========================================================================
}

-- 🧮 ZHU LI, DO THE THING!
for k, v in pairs(options) do
    vim.opt[k] = v
end
