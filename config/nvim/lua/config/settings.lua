local options = {
	tabstop = 4,
	shiftwidth = 4,
	expandtab = true,
	shiftround = true,

	wrap = false,
	breakindent = true,
	linebreak = true,
	scrolloff = 16,
	sidescrolloff = 6,
	spell = false,
	spelllang = { "en_us", "en_gb" },
	textwidth = 80,
	smoothscroll = true,

	hlsearch = true,
	showmode = false,
	shortmess = "filnxtToOFcsIC",
	number = true,
	relativenumber = true,
	signcolumn = "yes",
	termguicolors = true,
	colorcolumn = "100",
	foldmethod = "expr",
	foldexpr = "v:lua.vim.treesitter.foldexpr()",
	foldlevel = 6,
	updatetime = 300,
	timeoutlen = 500,
	fillchars = "vert: ,horiz: ,fold:⋅,eob: ,msgsep:‾",

	mouse = "a",
	splitbelow = false,
	splitright = true,
	completeopt = { "menu", "menuone", "noselect" },
	iskeyword = "@,48-57,_,-,192-255,#",
	swapfile = false,
	autoread = true,
	undofile = true,
	wildignore = ".back,~,.o,.h,.info,.swp,.obj,.pyc",
}

for k, v in pairs(options) do
	vim.opt[k] = v
end
