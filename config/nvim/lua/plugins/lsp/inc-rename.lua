return {
	"smjonas/inc-rename.nvim",
	event = "LspAttach",
	dependencies = {
		-- dressing input avoids E184 race with sh.vim syntax loading on 0.11.x
		{ "stevearc/dressing.nvim", opts = {} },
	},
	opts = {
		input_buffer_type = "dressing",
	},
}
