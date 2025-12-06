return {
	"folke/noice.nvim",
	event = "VeryLazy",
	dependencies = {
		"MunifTanjim/nui.nvim",
		"rcarriga/nvim-notify",
	},
	opts = {
		routes = {
			{ filter = { event = "msg_show", find = "written$" }, opts = { skip = true } },
			{ filter = { event = "msg_show", find = "^%d+ more lines?$" }, opts = { skip = true } },
			{ filter = { event = "msg_show", find = "^%d+ fewer lines?$" }, opts = { skip = true } },
			{ filter = { event = "msg_show", find = "^%d+ lines? yanked$" }, opts = { skip = true } },
			{ filter = { event = "msg_show", kind = "search_count" }, opts = { skip = true } },
			{ filter = { event = "msg_show", find = "^E486:" }, opts = { skip = true } },
			{ filter = { event = "msg_show", find = "^Already at" }, opts = { skip = true } },
			{ filter = { event = "notify", find = "Config Change Detected" }, opts = { skip = true } },
		},
		lsp = {
			override = {
				["vim.lsp.util.convert_input_to_markdown_lines"] = true,
				["vim.lsp.util.stylize_markdown"] = true,
			},
		},
		presets = {
			command_palette = true,
			long_message_to_split = true,
			lsp_doc_border = true,
		},
	},
}
