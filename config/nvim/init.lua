require("impatient")

local config_files = {
	"plugins",
	"options",
	"keymaps",
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
}
for _, file in ipairs(config_files) do
	require("user." .. file)
end
