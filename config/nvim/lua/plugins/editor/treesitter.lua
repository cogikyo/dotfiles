local function setup()
	-- Install parsers (async by default)
	-- stylua: ignore
	require("nvim-treesitter").install({
		"c", "lua", "luadoc", "vim", "vimdoc", "query", "markdown", "markdown_inline", -- required/docs
		"bash", "python", -- scripting
		"go", "gomod", "gosum", "gotmpl", "gowork", "templ", -- Go stack
		"javascript", "typescript", "tsx", "jsdoc", -- JS/TS stack
		"html", "css", -- web/templates
		"json", "jsonc", "yaml", "toml", "xml", -- data/config formats
		"graphql", "proto", "http", -- API schemas
		"dockerfile", "gitignore", "diff", "hcl", "terraform", -- infra/tooling
		"regex", "sql", -- text/query helpers
	})

	-- Enable treesitter highlighting + indentation for all filetypes
	vim.api.nvim_create_autocmd("FileType", {
		group = vim.api.nvim_create_augroup("TreesitterStart", { clear = true }),
		callback = function(args)
			if pcall(vim.treesitter.start, args.buf) then
				vim.bo[args.buf].indentexpr = "v:lua.require'nvim-treesitter'.indentexpr()"
			end
		end,
	})

	-- Textobjects
	require("nvim-treesitter-textobjects").setup({
		select = {
			lookahead = true,
			include_surrounding_whitespace = true,
		},
		move = {
			set_jumps = true,
		},
	})

	-- Select keymaps
	local select = function(capture)
		return function()
			require("nvim-treesitter-textobjects.select").select_textobject(capture, "textobjects")
		end
	end
	local select_maps = {
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
	}
	for lhs, capture in pairs(select_maps) do
		vim.keymap.set({ "x", "o" }, lhs, select(capture))
	end

	-- Move keymaps
	local move = require("nvim-treesitter-textobjects.move")
	local move_maps = {
		{ "]f", move.goto_next_start, "@function.outer" },
		{ "]c", move.goto_next_start, "@class.outer" },
		{ "]p", move.goto_next_start, "@parameter.outer" },
		{ "]l", move.goto_next_start, "@loop.outer" },
		{ "]e", move.goto_next_start, "@conditional.outer" },
		{ "]b", move.goto_next_start, "@block.outer" },
		{ "]F", move.goto_next_end, "@function.outer" },
		{ "]C", move.goto_next_end, "@class.outer" },
		{ "]P", move.goto_next_end, "@parameter.outer" },
		{ "]L", move.goto_next_end, "@loop.outer" },
		{ "]E", move.goto_next_end, "@conditional.outer" },
		{ "]B", move.goto_next_end, "@block.outer" },
		{ "[f", move.goto_previous_start, "@function.outer" },
		{ "[c", move.goto_previous_start, "@class.outer" },
		{ "[p", move.goto_previous_start, "@parameter.outer" },
		{ "[l", move.goto_previous_start, "@loop.outer" },
		{ "[e", move.goto_previous_start, "@conditional.outer" },
		{ "[b", move.goto_previous_start, "@block.outer" },
		{ "[F", move.goto_previous_end, "@function.outer" },
		{ "[C", move.goto_previous_end, "@class.outer" },
		{ "[P", move.goto_previous_end, "@parameter.outer" },
		{ "[L", move.goto_previous_end, "@loop.outer" },
		{ "[E", move.goto_previous_end, "@conditional.outer" },
		{ "[B", move.goto_previous_end, "@block.outer" },
	}
	for _, m in ipairs(move_maps) do
		vim.keymap.set({ "n", "x", "o" }, m[1], function()
			m[2](m[3], "textobjects")
		end)
	end

	-- Swap keymaps
	local swap = require("nvim-treesitter-textobjects.swap")
	vim.keymap.set("n", "<leader>ra", function() swap.swap_next("@parameter.inner") end)
	vim.keymap.set("n", "<leader>rf", function() swap.swap_next("@function.outer") end)
	vim.keymap.set("n", "<leader>rc", function() swap.swap_next("@class.outer") end)
	vim.keymap.set("n", "<leader>rA", function() swap.swap_previous("@parameter.inner") end)
	vim.keymap.set("n", "<leader>rF", function() swap.swap_previous("@function.outer") end)
	vim.keymap.set("n", "<leader>rC", function() swap.swap_previous("@class.outer") end)

	-- Autotag
	require("nvim-ts-autotag").setup()

	-- Context
	require("treesitter-context").setup()
	vim.api.nvim_set_hl(0, "TreesitterContext", { link = "Folded" })
	vim.api.nvim_set_hl(0, "TreesitterContextLineNumber", { link = "Folded" })
end

return {
	"nvim-treesitter/nvim-treesitter",
	branch = "main",
	lazy = false,
	build = ":TSUpdate",
	dependencies = {
		{ "nvim-treesitter/nvim-treesitter-textobjects", branch = "main" },
		"nvim-treesitter/nvim-treesitter-context",
		"windwp/nvim-ts-autotag",
	},
	config = setup,
}
