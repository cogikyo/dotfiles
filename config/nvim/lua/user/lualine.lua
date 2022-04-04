local status_ok, lualine = pcall(require, "lualine")
if not status_ok then
	return
end

local diagnostics = {
	"diagnostics",
	sources = { "nvim_diagnostic" },
	sections = { "error", "warn" },
	symbols = { error = " ", warn = " " },
	colored = true,
	update_in_insert = false,
}

lualine.setup({
	options = {
		icons_enabled = true,
		theme = "auto",
		component_separators = { left = "╼╾", right = "╼╾" },
		section_separators = { left = "", right = "" },
		disabled_filetypes = {},
		always_divide_middle = true,
		globalstatus = false,
	},
	sections = {
		lualine_a = { "mode" },
		lualine_b = { "branch", "filename", diagnostics },
		lualine_c = { "" },
		lualine_x = { "" },
		lualine_y = { "filetype" },
		lualine_z = { "filename" },
	},
	inactive_sections = {
		lualine_a = { " " },
		lualine_b = {},
		lualine_c = {},
		lualine_x = {},
		lualine_y = {},
		lualine_z = { "filetype" },
	},
	tabline = {},
	extensions = {
		"nvim-tree",
		"quickfix",
		"toggleterm",
	},
})
