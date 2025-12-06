return {
	"folke/noice.nvim",
	event = "VeryLazy",
	dependencies = {
		"MunifTanjim/nui.nvim",
		{ "rcarriga/nvim-notify", opts = { timeout = 2000 } },
	},
	opts = {
		cmdline = {
			enabled = true, -- enables the Noice cmdline UI
			view = "cmdline_popup", -- view for rendering the cmdline. Change to `cmdline` to get a classic cmdline at the bottom
			opts = {}, -- global options for the cmdline. See section on views
			---@type table<string, CmdlineFormat>
			format = {
				-- conceal: (default=true) This will hide the text in the cmdline that matches the pattern.
				-- view: (default is cmdline view)
				-- opts: any options passed to the view
				-- icon_hl_group: optional hl_group for the icon
				-- title: set to anything or empty string to hide
				cmdline = { pattern = "^:", icon = "", lang = "vim" },
				search_down = { title = "  Search ", pattern = "^/", icon = "󰬧 ", lang = "regex" },
				search_up = { title = "  Search ", pattern = "^%?", icon = " ", lang = "regex" },
				filter = { title = " ", pattern = "^:%s*!", icon = "$", lang = "bash" },
				lua = { pattern = { "^:%s*lua%s+", "^:%s*lua%s*=%s*", "^:%s*=%s*" }, icon = "", lang = "lua" },
				help = { pattern = "^:%s*he?l?p?%s+", icon = "" },
				input = { view = "cmdline_input", icon = "󰥻 " }, -- Used by input()
				-- lua = false, -- to disable a format, set to `false`
			},
		},
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
