local npairs = require("nvim-autopairs")

npairs.setup({
	enable_check_bracket_line = false,
	ignored_next_char = "[%w%.]", -- will ignore alphanumeric and `.` symbol
	fast_wrap = {
		map = "<C-e>",
		chars = { "{", "[", "(", '"', "'", "`", "<" },
		pattern = string.gsub([[ [%'%"%)%>%]%)%}%,] ]], "%s+", ""),
		offset = -1,
		end_key = "$",
		keys = "asetniol",
		check_comma = true,
		highlight = "Search",
		highlight_grey = "Comment",
	},
	check_ts = true,
	ts_config = {
		lua = { "string" }, -- it will not add a pair on that treesitter node
	},
})

-- https://github.com/windwp/nvim-autopairs/wiki/Custom-rules

local cond = require("nvim-autopairs.conds")
local ts_conds = require("nvim-autopairs.ts-conds")
local Rule = require("nvim-autopairs.rule")

npairs.add_rules({
	Rule("%", "%", "lua"):with_pair(ts_conds.is_ts_node({ "string", "comment" })),
	Rule("$", "$", "lua"):with_pair(ts_conds.is_not_ts_node({ "function" })),
})

local brackets = { { "(", ")" }, { "[", "]" }, { "{", "}" } }
npairs.add_rules({
	Rule(" ", " ")
		:with_pair(function(opts)
			local pair = opts.line:sub(opts.col - 1, opts.col)
			return vim.tbl_contains({
				brackets[1][1] .. brackets[1][2],
				brackets[2][1] .. brackets[2][2],
				brackets[3][1] .. brackets[3][2],
			}, pair)
		end)
		:with_move(cond.none())
		:with_cr(cond.none())
		:with_del(function(opts)
			local col = vim.api.nvim_win_get_cursor(0)[2]
			local context = opts.line:sub(col - 1, col + 2)
			return vim.tbl_contains({
				brackets[1][1] .. "  " .. brackets[1][2],
				brackets[2][1] .. "  " .. brackets[2][2],
				brackets[3][1] .. "  " .. brackets[3][2],
			}, context)
		end),
})

for _, bracket in pairs(brackets) do
	Rule("", " " .. bracket[2])
		:with_pair(cond.none())
		:with_move(function(opts)
			return opts.char == bracket[2]
		end)
		:with_cr(cond.none())
		:with_del(cond.none())
		:use_key(bracket[2])
end

for _, punct in pairs({ ",", ";" }) do
	npairs.add_rules({
		Rule("", punct)
			:with_move(function(opts)
				return opts.char == punct
			end)
			:with_pair(function()
				return false
			end)
			:with_del(function()
				return false
			end)
			:with_cr(function()
				return false
			end)
			:use_key(punct),
	})
end

Rule("%(.*%)%s*%=>$", " {  }", { "typescript", "typescriptreact", "javascript" }):use_regex(true):set_end_pair_length(2)
