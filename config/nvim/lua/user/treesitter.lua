-- ============================================================================
-- 🧰 Setup {{{
local ok, treesitter = pcall(require, "nvim-treesitter.configs")
if not ok then
	vim.api.nvim_echo({
		{
			"Error: nvim-treesitter plugin not found... skipping relevant setup()",
			"Error",
		},
	}, true, {})
	return
end
-- }}}
-- ============================================================================

treesitter.setup({
	ensure_installeid = {
		"c",
		"lua",
		"python",
        "md",
		"bash",
		"vim",
		"yaml",
		"help",
		"query",
		"comment",
		"vimdoc",
		"query",
		"typescript",
		"javascript",
		"json",
		"html",
		"css",
		"scss",
		"rust",
		"toml",
		"go",
        "java",
	},
	sync_install = true,
	auto_install = true,
	highlight = {
		enable = true,
		additional_vim_regex_highlighting = false,
	},
	indent = {
		enable = true,
		disable = { "python" },
	},
	incremental_selection = {
		enable = true,
		keymaps = {
			init_selection = "gn", -- normal: start incremental selection.
			node_incremental = "gn", -- visual: increment to the upper named parent.
			scope_incremental = "gy", -- visual: increment to the upper scope
			node_decremental = "gp", -- visual: decrement to the previous named node.
		},
	},
	textobjects = {
		select = {
			enable = true,
			lookahead = true, -- Automatically jump forward to textobj, similar to targets.vim
			keymaps = {
				-- You can use the capture groups defined in textobjects.scm
				["aa"] = "@parameter.outer",
				["ia"] = "@parameter.inner",
				["af"] = "@function.outer",
				["if"] = "@function.inner",
				["ac"] = "@class.outer",
				["ic"] = "@class.inner",
			},
		},
	},
	move = {
		enable = true,
		set_jumps = true, -- whether to set jumps in the jumplist
		goto_next_start = {
			["]m"] = "@function.outer",
			["]]"] = "@class.outer",
		},
		goto_next_end = {
			["]M"] = "@function.outer",
			["]["] = "@class.outer",
		},
		goto_previous_start = {
			["[m"] = "@function.outer",
			["[["] = "@class.outer",
		},
		goto_previous_end = {
			["[M"] = "@function.outer",
			["[]"] = "@class.outer",
		},
	},
	context_commentstring = {
		enable = true,
		enable_autocmd = false, -- integration with Comment.nvim
	},
	playground = {
		enable = true,
		disable = {},
		updatetime = 25, -- Debounced time for highlighting nodes in the playground from source code
		persist_queries = false, -- Whether the query persists across vim sessions
		keybindings = {
			toggle_query_editor = "o",
			toggle_hl_groups = "i",
			toggle_injected_languages = "t",
			toggle_anonymous_nodes = "a",
			toggle_language_display = "I",
			focus_language = "f",
			unfocus_language = "F",
			update = "R",
			goto_node = "<CR>",
			show_help = "?",
		},
	},
	autopairs = { enable = true },
	autotag = { enable = true },
})
