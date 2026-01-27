return {
	"nvim-telescope/telescope.nvim",
	dependencies = {
		"nvim-lua/plenary.nvim",
		"natecraddock/telescope-zf-native.nvim",
	},
	config = function()
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
						["<Tab>"] = actions.toggle_selection + actions.move_selection_worse,
						["<S-Tab>"] = actions.toggle_selection + actions.move_selection_better,
						["<C-k>"] = actions.move_selection_previous,
						["<C-j>"] = actions.move_selection_next,
						["<C-b>"] = actions.results_scrolling_up,
						["<C-f>"] = actions.results_scrolling_down,
					},
					n = {
						["<Down>"] = actions.move_selection_next,
						["<Up>"] = actions.move_selection_previous,
						["<PageUp>"] = actions.results_scrolling_up,
						["<PageDown>"] = actions.results_scrolling_down,
					},
				},
			},
			pickers = {
				find_files = {
					theme = "dropdown",
					previewer = true,
				},
				buffers = {
					theme = "dropdown",
					previewer = true,
				},
				live_grep = {
					theme = "dropdown",
				},
				current_buffer_fuzzy_find = {
					theme = "dropdown",
				},
			},
		})
	end,
}
