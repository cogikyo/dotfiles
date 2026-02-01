return {
	"saghen/blink.cmp",
	version = "1.*",
	event = "InsertEnter",
	dependencies = {
		"rafamadriz/friendly-snippets",
	},
	opts = {
		keymap = {
			preset = "none",
			["<C-Space>"] = { "show", "show_documentation", "hide_documentation" },
			["<C-l>"] = {
				function(cmp)
					if not cmp.is_visible() then
						return false
					end
					cmp.select_next()
					vim.schedule(function()
						cmp.accept()
					end)
					return true
				end,
				"fallback",
			},
			["<Down>"] = { "select_next", "fallback" },
			["<Up>"] = { "select_prev", "fallback" },
			["<Right>"] = { "accept", "fallback" },
			["<Left>"] = { "hide", "fallback" },
		},
		completion = {
			list = { selection = { preselect = false, auto_insert = false } },
			accept = { auto_brackets = { enabled = true } },
			documentation = {
				auto_show = false,
				auto_show_delay_ms = 200,
				window = { border = "rounded" },
			},
			menu = {
				auto_show = false,
				border = "rounded",
				draw = {
					columns = { { "kind_icon" }, { "label", "label_description", gap = 1 } },
				},
			},
		},
		sources = {
			default = { "lsp", "path", "snippets", "buffer" },
		},
		signature = { enabled = true, window = { border = "rounded" } },
	},
}
