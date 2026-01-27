return {
	"smjonas/inc-rename.nvim",
	cmd = "IncRename",
	dependencies = {
		-- use dressing input to avoid race condition with sh.vim syntax
		-- loading on Neovim 0.11.x (E184: ShFoldIfDoFor command not found)
		{ "stevearc/dressing.nvim", opts = {} },
	},
	opts = {
		input_buffer_type = "dressing",
	},
}
