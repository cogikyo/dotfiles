require("impatient")

local config_files = {
	"plugins",
	"cmp",
	"colorscheme",
	"lsp",
	"telescope",
	"treesitter",
	"autopairs",
	"comment",
	"gitsigns",
	"nvim-tree",
	"toggleterm",
	"lualine",
	"alpha",
	"spectre",
	"options",
	"keymaps",
}

for _, file in ipairs(config_files) do
	require("user." .. file)
end
