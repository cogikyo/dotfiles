local ok, telescope = pcall(require, "telescope")

if not ok then
	vim.api.nvim_echo({
		{
			"Error: telescope plugin not found... skipping relevant setup()",
			"Error",
		},
	}, true, {})
	return
end

telescope.load_extension("zf-native")

local actions = require("telescope.actions")

-- local open_with_trouble = require("troubile.sources.telescope").open
-- local add_to_trouble = require("trouble.sources.telescope").add

telescope.setup({
	defaults = {
		layout_strategy = "vertical",
		layout_config = {
			vertical = {
				prompt_position = "top",
			},
		},
		sorting_strategy = "ascending",
		prompt_prefix = " 󰭎  ",
		selection_caret = "  ",
		path_display = { "smart" },
		mappings = {
			i = {
				["<C-Down>"] = actions.cycle_history_next,
				["<C-Up>"] = actions.cycle_history_prev,
				["<Esc>"] = actions.close,
				["<C-c>"] = actions.close,
				["<Down>"] = actions.move_selection_next,
				["<Up>"] = actions.move_selection_previous,
				["<PageUp>"] = actions.results_scrolling_up,
				["<PageDown>"] = actions.results_scrolling_down,
				["<C-u>"] = actions.preview_scrolling_up,
				["<C-d>"] = actions.preview_scrolling_down,
				["<CR>"] = actions.select_default,
				["<C-h>"] = actions.select_horizontal,
				["<C-v>"] = actions.select_vertical,
				["<c-t>"] = actions.select_tab,
				["<Tab>"] = actions.toggle_selection + actions.move_selection_worse,
				["<S-Tab>"] = actions.toggle_selection + actions.move_selection_better,
				["<C-f>"] = actions.send_to_qflist + actions.open_qflist,
				["<C-q>"] = actions.send_selected_to_qflist + actions.open_qflist,
				["<C-l>"] = actions.complete_tag,
                -- ["<c-t>"] = open_with_trouble,
				["<C-Space>"] = actions.which_key,
			},
		},
	},
	extensions = {
		["zf-native"] = {
			file = {
				enable = true,
				highlight_results = true,
				match_filename = true,
			},
			generic = {
				enable = true,
				highlight_results = true,
				match_filename = false,
			},
		},
	},
})

telescope.load_extension("refactoring")
