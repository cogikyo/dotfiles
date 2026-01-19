-- see :help '{option}' for more details ⮯

local options = {
	tabstop = 4, -- preferred tabstop length.
	shiftwidth = 4, -- set to same as tabstop ⮭
	expandtab = true, -- as per option 2 in :help 'tabstop'
	shiftround = true, -- ensures indent is a multiple of shiftwidth

	wrap = false, -- disable text wrapping by default
	breakindent = true, -- continue to indent text (if wrapped line)
	linebreak = true, -- smarter (more control) over line breaking; see 'breakat'
	scrolloff = 16, -- minimum lines above/below cursor
	sidescrolloff = 6, -- minimum lines left/right of cursor
	spell = false, -- turn spell check on by default
	spelllang = { "en_us", "en_gb" }, -- easily add/remove dictionaries
	textwidth = 80, -- max line length for formatting (gq)
	smoothscroll = true, -- scroll by screen line, not text line

	hlsearch = true, -- highlight all search matches
	shortmess = "filnxtToOFcsIC", -- shorten/suppress various messages
	number = true, -- show absolute line number
	relativenumber = true, -- show relative line numbers (easier motions)
	signcolumn = "yes", -- always show sign column (left of numbers)
	termguicolors = true, -- 24-bit RGB color support
	colorcolumn = "100", -- visual guides for line length
	foldmethod = "expr",
	foldexpr = "v:lua.vim.treesitter.foldexpr()",
	foldlevel = 6,
	updatetime = 250, -- timer until events execute when cursors stops (ms)
	timeoutlen = 500, -- timeout for mapped sequence to complete
	fillchars = "vert: ,horiz: ,fold:⋅,eob: ,msgsep:‾",

	mouse = "a", -- enable mouse support
	splitbelow = false, -- do split below by default
	splitright = true, -- split right by default
	completeopt = { "menu", "menuone", "noselect" }, -- insert complete options
	iskeyword = "@,48-57,_,-,192-255,#", -- define what a ends a "word"
	swapfile = false, -- swapfiles are useless to me
	undofile = true, -- persistent undo, very useful with 'mbbill/undotree' plugin
	wildignore = ".back,~,.o,.h,.info,.swp,.obj,.pyc", -- don't check these files for "*" pattern matching
}

for k, v in pairs(options) do
	vim.opt[k] = v
end
