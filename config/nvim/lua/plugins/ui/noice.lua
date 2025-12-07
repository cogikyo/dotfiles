return {
	"folke/noice.nvim",
	event = "VeryLazy",
	dependencies = {
		"MunifTanjim/nui.nvim",
		"rcarriga/nvim-notify",
	},
	config = function()
		require("notify").setup({
			timeout = 2000,
			stages = "slide",
			level = vim.log.levels.TRACE,
			icons = {
				ERROR = " ",
				WARN = " ",
				INFO = " ",
				DEBUG = " ",
				TRACE = " ",
			},
		})
		vim.api.nvim_create_autocmd("BufWritePost", {
			pattern = { "*/plugins/ui/noice.lua", "*/vagari.nvim/lua/vagari/highlights.lua" },
			callback = function()
				vim.defer_fn(function()
					local notify = require("notify")
					notify("This is an ERROR notification", "error", { title = "Error" })
					notify("This is a WARN notification", "warn", { title = "Warn" })
					notify("This is an INFO notification", "info", { title = "Info" })
					notify("This is a DEBUG notification", "debug", { title = "Debug" })
					notify("This is a TRACE notification", "trace", { title = "Trace" })
				end, 100)
			end,
		})

		require("noice").setup({
			cmdline = {
				enabled = true, -- enables the Noice cmdline UI
				view = "cmdline_popup", -- view for rendering the cmdline. Change to `cmdline` to get a classic cmdline at the bottom
				opts = {}, -- global options for the cmdline. See section on views
				format = {
					-- conceal: (default=true) This will hide the text in the cmdline that matches the pattern.
					-- view: (default is cmdline view)
					-- opts: any options passed to the view
					-- icon_hl_group: optional hl_group for the icon
					-- title: set to anything or empty string to hide
					cmdline = { pattern = "^:", icon = "", lang = "vim" },
					search_down = { title = "  /search ", pattern = "^/", icon = "󰬧", lang = "regex" },
					search_up = { title = "  ?search ", pattern = "^%?", icon = "", lang = "regex" },
					filter = { title = " ! filter ", pattern = "^:%s*!", icon = "$", lang = "bash" },
					lua = {
						title = " = lua ",
						pattern = { "^:%s*lua%s+", "^:%s*lua%s*=%s*", "^:%s*=%s*" },
						icon = "",
						lang = "lua",
					},
					help = { title = " h help ", pattern = "^:%s*he?l?p?%s+", icon = "󰾚 " },
					input = { view = "cmdline_input", icon = "󰥻 " },
				},
			},
			routes = {
				{ filter = { event = "msg_show", find = "written$" }, opts = { skip = true } },
				{ filter = { event = "msg_show", find = "^%d+ more lines?$" }, opts = { skip = true } },
				{ filter = { event = "msg_show", find = "^%d+ fewer lines?$" }, opts = { skip = true } },
				{ filter = { event = "msg_show", find = "lines? yanked$" }, opts = { skip = true } },
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
		})
	end,
}
