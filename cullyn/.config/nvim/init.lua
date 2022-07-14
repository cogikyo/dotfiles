local status_ok, impatient = pcall(require, "impatient")
if not status_ok then
	return
end

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
	-- "nvim-tree",
	"toggleterm",
	"lualine",
	"alpha",
	"spectre",
}

for _, file in ipairs(config_files) do
	require("user." .. file)
end
