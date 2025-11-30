local ok, treesitter = pcall(require, "nvim-treesitter.configs")
if not ok then
	return
end

treesitter.setup({
	ensure_installed = {
		"bash",
		"c",
		"css",
		"diff",
		"dockerfile",
		"gitignore",
		"go",
		"gomod",
		"gosum",
		"html",
		"javascript",
		"jsdoc",
		"json",
		"jsonc",
		"lua",
		"luadoc",
		"markdown",
		"markdown_inline",
		"python",
		"query",
		"regex",
		"sql",
		"templ",
		"toml",
		"tsx",
		"typescript",
		"vim",
		"vimdoc",
		"xml",
		"yaml",
	},
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
			init_selection = "gn",
			node_incremental = "gn",
			scope_incremental = "gy",
			node_decremental = "gp",
		},
	},
	textobjects = {
		select = {
			enable = true,
			lookahead = true,
			keymaps = {
				["aa"] = { query = "@parameter.outer", desc = "around argument" },
				["ia"] = { query = "@parameter.inner", desc = "inside argument" },
				["af"] = { query = "@function.outer", desc = "around function" },
				["if"] = { query = "@function.inner", desc = "inside function" },
				["ac"] = { query = "@class.outer", desc = "around class" },
				["ic"] = { query = "@class.inner", desc = "inside class" },
				["at"] = { query = "@tag.outer", desc = "around tag" },
				["it"] = { query = "@tag.inner", desc = "inside tag" },
				["ab"] = { query = "@block.outer", desc = "around block" },
				["ib"] = { query = "@block.inner", desc = "inside block" },
				["al"] = { query = "@loop.outer", desc = "around loop" },
				["il"] = { query = "@loop.inner", desc = "inside loop" },
				["a/"] = { query = "@comment.outer", desc = "around comment" },
				["i/"] = { query = "@comment.inner", desc = "inside comment" },
				["ai"] = { query = "@conditional.outer", desc = "around conditional" },
				["ii"] = { query = "@conditional.inner", desc = "inside conditional" },
			},
		},
		swap = {
			enable = true,
			swap_next = {
				["<leader>sa"] = { query = "@parameter.inner", desc = "Swap with next argument" },
				["<leader>sf"] = { query = "@function.outer", desc = "Swap with next function" },
			},
			swap_previous = {
				["<leader>sA"] = { query = "@parameter.inner", desc = "Swap with previous argument" },
				["<leader>sF"] = { query = "@function.outer", desc = "Swap with previous function" },
			},
		},
		move = {
			enable = true,
			set_jumps = true,
			goto_next_start = {
				["]f"] = { query = "@function.outer", desc = "Next function start" },
				["]c"] = { query = "@class.outer", desc = "Next class start" },
				["]a"] = { query = "@parameter.inner", desc = "Next argument" },
				["]l"] = { query = "@loop.outer", desc = "Next loop" },
				["]i"] = { query = "@conditional.outer", desc = "Next conditional" },
			},
			goto_next_end = {
				["]F"] = { query = "@function.outer", desc = "Next function end" },
				["]C"] = { query = "@class.outer", desc = "Next class end" },
			},
			goto_previous_start = {
				["[f"] = { query = "@function.outer", desc = "Previous function start" },
				["[c"] = { query = "@class.outer", desc = "Previous class start" },
				["[a"] = { query = "@parameter.inner", desc = "Previous argument" },
				["[l"] = { query = "@loop.outer", desc = "Previous loop" },
				["[i"] = { query = "@conditional.outer", desc = "Previous conditional" },
			},
			goto_previous_end = {
				["[F"] = { query = "@function.outer", desc = "Previous function end" },
				["[C"] = { query = "@class.outer", desc = "Previous class end" },
			},
		},
	},
})

require("nvim-ts-autotag").setup({
	opts = {
		enable_close = true,
		enable_rename = true,
		enable_close_on_slash = true,
	},
})

require("treesitter-context").setup({
	enable = true,
	max_lines = 3,
	multiline_threshold = 1,
	trim_scope = "outer",
	mode = "cursor",
})

vim.keymap.set("n", "[x", function()
	require("treesitter-context").go_to_context(vim.v.count1)
end, { desc = "Jump to context" })
