return {
	"nvim-telescope/telescope.nvim",
	dependencies = {
		"nvim-lua/plenary.nvim",
		"natecraddock/telescope-zf-native.nvim",
	},
	-- stylua: ignore
	keys = {
		{ "<leader>t<leader>", "<cmd>Telescope find_files<cr>",     desc = "Find files" },
		{ "<leader>e<leader>", "<cmd>Telescope oldfiles<cr>",       desc = "Recent files" },
		{ "<leader>s<leader>", "<cmd>Telescope live_grep<cr>",      desc = "Live grep" },
		{ "<leader>g<leader>", "<cmd>Telescope grep_string<cr>",    desc = "Grep string under cursor" },
		{ "<leader>tg",        "<cmd>Telescope git_files<cr>",      desc = "Git files" },
		{ "<leader>tb",        "<cmd>Telescope buffers<cr>",        desc = "Buffers" },
		{ "<leader>th",        "<cmd>Telescope help_tags<cr>",      desc = "Help tags" },
		{ "<leader>tH",        "<cmd>Telescope highlights<cr>",     desc = "Highlight groups" },
		{ "<leader>td",        "<cmd>Telescope diagnostics<cr>",    desc = "Diagnostics" },
		{ "<leader>tp",        "<cmd>Telescope builtin<cr>",        desc = "Telescope builtins" },
		{ "<leader>tc",        "<cmd>Telescope commands<cr>",       desc = "Commands" },
		{ "<leader>tl",        "<cmd>Telescope loclist<cr>",        desc = "Location list" },
		{ "<leader>tq",        "<cmd>Telescope quickfix<cr>",       desc = "Quickfix list" },
		{ "<leader>tm",        "<cmd>Telescope man_pages<cr>",      desc = "Man pages" },
		{ "<leader>tt",        "<cmd>Telescope resume<cr>",         desc = "Resume last picker" },
		{ "<leader>t;",        "<cmd>Telescope marks<cr>",          desc = "Marks" },
		{ "<leader>tst",       "<cmd>Telescope treesitter<cr>",     desc = "Treesitter symbols" },
		{ "<leader>tk",        "<cmd>Telescope keymaps<cr>",        desc = "Keymaps" },
		{ "<leader>trg",       "<cmd>Telescope registers<cr>",      desc = "Registers" },
		{ "<leader>tco",       "<cmd>Telescope colorscheme<cr>",    desc = "Colorschemes" },
		{ "<leader>tj",        "<cmd>Telescope jumplist<cr>",       desc = "Jumplist" },
		{ "<leader>tsh",       "<cmd>Telescope search_history<cr>", desc = "Search history" },
		{ "<leader>tsp",       "<cmd>Telescope spell_suggest<cr>",  desc = "Spelling suggestions" },
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

		-- Hook Picker.new to apply dynamic layout based on columns
		local Picker = require("telescope.pickers")
		local config = require("telescope.config")
		local original_new = Picker.new

		Picker.new = function(self, opts)
			local cols = vim.o.columns
			if cols > 240 then
				config.values.layout_strategy = "horizontal"
				config.values.sorting_strategy = "ascending"
				config.values.layout_config.horizontal =
					vim.tbl_extend("force", config.values.layout_config.horizontal or {}, {
						prompt_position = "top",
						preview_width = 0.65,
						width = 0.65,
						height = 0.65,
					})
			elseif cols >= 140 then
				config.values.layout_strategy = "vertical"
				config.values.sorting_strategy = "descending"
				config.values.layout_config.vertical =
					vim.tbl_extend("force", config.values.layout_config.vertical or {}, {
						prompt_position = "bottom",
						preview_height = 0.65,
						mirror = true,
						width = 0.75,
						height = 0.75,
					})
			else
				config.values.layout_strategy = "vertical"
				config.values.sorting_strategy = "ascending"
				config.values.layout_config.horizontal =
					vim.tbl_extend("force", config.values.layout_config.horizontal or {}, {
						prompt_position = "top",
						preview_height = 0.65,
						width = 0.65,
						height = 0.65,
					})
			end
			return original_new(self, opts)
		end

		telescope.setup({
			defaults = {
				prompt_prefix = " 󰭎  ",
				selection_caret = "  ",
				path_display = { "smart" },
        -- stylua: ignore start
				file_ignore_patterns = {
					"%.png$", "%.jpg$", "%.jpeg$", "%.gif$", "%.bmp$", "%.ico$", "%.webp$", "%.svg$",
					"%.pdf$", "%.zip$", "%.tar$", "%.gz$", "%.7z$", "%.rar$", "%.webm$", "%.flv$",
					"%.exe$", "%.dll$", "%.so$", "%.dylib$", "%.o$", "%.a$", "%.pyc$",
					"%.woff$", "%.woff2$", "%.ttf$", "%.otf$", "%.eot$", "%.mpkg$", "%.ogg$",
					"%.mp3$", "%.mp4$", "%.wav$", "%.avi$", "%.mov$", "%.mkv$",
				},
				-- stylua: ignore end
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
			pickers = {},
			extensions = {},
		})
	end,
}
