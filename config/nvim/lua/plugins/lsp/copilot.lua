return {
	"zbirenbaum/copilot.lua",
	cmd = "Copilot",
	event = "InsertEnter",
	opts = {
		suggestion = {
			enabled = true,
			auto_trigger = true,
			debounce = 75,
			keymap = {
				accept = "<Right>",
				accept_word = "<C-Right>",
				accept_line = "<C-S-Right>",
				next = "<PageDown>",
				prev = "<PageUp>",
				dismiss = "<Esc>",
			},
		},
		panel = { enabled = false },
		filetypes = {
			markdown = true,
			help = false,
			gitcommit = false,
			gitrebase = false,
			["."] = false,
		},
	},
}
