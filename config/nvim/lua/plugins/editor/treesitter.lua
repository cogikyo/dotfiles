local function setup()
	require("nvim-treesitter.configs").setup({
		-- stylua: ignore
		ensure_installed = {
			"c", "lua", "luadoc", "vim", "vimdoc", "query", "markdown", "markdown_inline", -- required/docs
			"bash", "python", -- scripting
			"go", "gomod", "gosum", "gotmpl", "gowork", "templ", -- Go stack
			"javascript", "typescript", "tsx", "jsdoc", -- JS/TS stack
			"html", "css", -- web/templates
			"json", "jsonc", "yaml", "toml", "xml", -- data/config formats
			"graphql", "proto", "http", -- API schemas
			"dockerfile", "gitignore", "diff", "hcl", "terraform", -- infra/tooling
			"regex", "sql", -- text/query helpers
		},
		auto_install = true,
		highlight = { enable = true },
		indent = { enable = true },
		autotag = { enable = true },
		incremental_selection = {
			enable = true,
			keymaps = {
				init_selection = "<C-n>",
				node_incremental = "<C-n>",
				node_decremental = "<C-p>",
				scope_incremental = "<C-l>",
			},
		},
		textobjects = {
			select = {
				enable = true,
				lookahead = true,
				include_surrounding_whitespace = true,
				keymaps = {
					["af"] = "@function.outer",
					["if"] = "@function.inner",
					["ac"] = "@class.outer",
					["ic"] = "@class.inner",
					["aa"] = "@parameter.outer",
					["ia"] = "@parameter.inner",
					["aL"] = "@loop.outer",
					["iL"] = "@loop.inner",
					["ae"] = "@conditional.outer",
					["iE"] = "@conditional.inner",
					["aB"] = "@block.outer",
					["iB"] = "@block.inner",
				},
			},
			move = {
				enable = true,
				set_jumps = true,
				goto_next_start = {
					["]f"] = "@function.outer",
					["]c"] = "@class.outer",
					["]p"] = "@parameter.outer",
					["]l"] = "@loop.outer",
					["]e"] = "@conditional.outer",
					["]b"] = "@block.outer",
				},
				goto_next_end = {
					["]F"] = "@function.outer",
					["]C"] = "@class.outer",
					["]P"] = "@parameter.outer",
					["]L"] = "@loop.outer",
					["]E"] = "@conditional.outer",
					["]B"] = "@block.outer",
				},
				goto_previous_start = {
					["[f"] = "@function.outer",
					["[c"] = "@class.outer",
					["[p"] = "@parameter.outer",
					["[l"] = "@loop.outer",
					["[e"] = "@conditional.outer",
					["[b"] = "@block.outer",
				},
				goto_previous_end = {
					["[F"] = "@function.outer",
					["[C"] = "@class.outer",
					["[P"] = "@parameter.outer",
					["[L"] = "@loop.outer",
					["[E"] = "@conditional.outer",
					["[B"] = "@block.outer",
				},
			},
			swap = {
				enable = true,
				swap_next = {
					["<leader>ra"] = "@parameter.inner",
					["<leader>rf"] = "@function.outer",
					["<leader>rc"] = "@class.outer",
				},
				swap_previous = {
					["<leader>rA"] = "@parameter.inner",
					["<leader>rF"] = "@function.outer",
					["<leader>rC"] = "@class.outer",
				},
			},
			lsp_interop = {
				enable = true,
				peek_definition_code = {
					["<leader>pf"] = "@function.outer",
					["<leader>pc"] = "@class.outer",
				},
			},
		},
	})

	require("treesitter-context").setup()
	vim.api.nvim_set_hl(0, "TreesitterContext", { link = "Folded" })
	vim.api.nvim_set_hl(0, "TreesitterContextLineNumber", { link = "Folded" })
end

return {
	"nvim-treesitter/nvim-treesitter",
	build = ":TSUpdate",
	dependencies = {
		"nvim-treesitter/nvim-treesitter-textobjects",
		"nvim-treesitter/nvim-treesitter-context",
		"windwp/nvim-ts-autotag",
	},
	config = setup,
}
