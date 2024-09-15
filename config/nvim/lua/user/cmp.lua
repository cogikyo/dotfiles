-- ============================================================================
-- 🧰 setup {{{
local cmp_ok, cmp = pcall(require, "cmp")
if not cmp_ok then
	vim.api.nvim_echo({
		{
			"Error: cmp plugin not found... skipping relevant setup()",
			"Error",
		},
	}, true, {})
	return
end

local luasnip_ok, luasnip = pcall(require, "luasnip")
if not luasnip_ok then
	vim.api.nvim_echo({
		{
			"Error: cmp uses luasnip, but luasnip is not found, skipping relevant setup()",
			"Error",
		},
	}, true, {})
	return
end

-- local copilot_ok, copilot = pcall(require, "copilot")
-- if not copilot_ok then
-- 	vim.api.nvim_echo({
-- 		{
-- 			"Error: cmp is integrated with copilot, but copilot is not found, skipping relevant setup",
-- 			"Error",
-- 		},
-- 	}, true, {})
-- 	return
-- end
--
-- local copilot_cmp_ok, copilot_cmp = pcall(require, "copilot_cmp")
-- if not copilot_cmp_ok then
-- 	vim.api.nvim_echo({
-- 		{
-- 			"Error: copilt needs copilt_cmp to work well, but copilot_cmp is not found, skipping relevant setup",
-- 			"Error",
-- 		},
-- 	}, true, {})
-- 	return
-- end

-- }}}
-- ============================================================================

require("luasnip/loaders/from_vscode").lazy_load()

local kind_icons = {
	Text = "⼠",
	Method = "⟜",
	Function = "➠",
	Constructor = "",
	Field = "",
	Variable = "λ",
	Class = "",
	Interface = "",
	Module = "",
	Property = "",
	Unit = "",
	Value = "󱁦",
	Enum = "",
	Keyword = "",
	Snippet = "",
	Color = "",
	File = "",
	Reference = "",
	Folder = "",
	EnumMember = "",
	Constant = "ℎ",
	Struct = "",
	Event = "",
	Operator = "",
	TypeParameter = "",
	Copilot = "",
}

cmp.setup({
	snippet = {
		expand = function(args)
			luasnip.lsp_expand(args.body) -- For `luasnip` users.
		end,
	},
	mapping = cmp.mapping.preset.insert({
		["<C-u>"] = cmp.mapping.scroll_docs(-4),
		["<C-d>"] = cmp.mapping.scroll_docs(4),
		["<C-Space>"] = cmp.mapping.complete(),
		["<C-e>"] = cmp.mapping.abort(),
		["<Right>"] = cmp.mapping.confirm({
			behavior = cmp.ConfirmBehavior.Replace,
			select = true,
		}),
		["<CR>"] = cmp.mapping.confirm({
			behavior = cmp.ConfirmBehavior.Replace,
			select = false,
		}),
		["<Tab>"] = cmp.mapping(function(fallback)
			if cmp.visible() then
				cmp.select_next_item()
			elseif luasnip.expandable() then
				luasnip.expand()
			elseif luasnip.expand_or_jumpable() then
				luasnip.expand_or_jump()
			else
				fallback()
			end
		end, { "i", "s" }),
		["<S-Tab>"] = cmp.mapping(function(fallback)
			if cmp.visible() then
				cmp.select_prev_item()
			elseif luasnip.jumpable(-1) then
				luasnip.jump(-1)
			else
				fallback()
			end
		end, { "i", "s" }),
	}),
	formatting = {
		fields = { "kind", "abbr", "menu" },
		format = function(entry, vim_item)
			vim_item.kind = string.format("%s ", kind_icons[vim_item.kind])
			vim_item.menu = ({
				-- copilot = " copilot",
				nvim_lua = " nvim_lua",
				nvim_lsp = " lsp",
				luasnip = " luasnip",
				buffer = "⼞buffer",
				path = "⨒ path",
				emoji = "ﲃ emoji",
				nerdfont = "󰊪 nerdfont",
			})[entry.source.name]
			return vim_item
		end,
	},
	sources = {
		{ name = "nvim_lsp", group_index = 2 },
		-- { name = "copilot", group_index = 2 },
		{ name = "luasnip", group_index = 2 },
		{ name = "path", group_index = 2 },
		{ name = "nvim_lua", group_index = 2 },
		{ name = "buffer", group_index = 2 },
		{ name = "emoji", group_index = 2 },
		{ name = "nerdfont", group_index = 2 },
	},
	sorting = {
		priority_weight = 2,
		comparators = {
			-- require("copilot_cmp.comparators").prioritize,

			-- Below is the default comparator list and order for nvim-cmp
			cmp.config.compare.offset,
			-- cmp.config.compare.scopes, --this is commented in nvim-cmp too
			cmp.config.compare.exact,
			cmp.config.compare.score,
			cmp.config.compare.recently_used,
			cmp.config.compare.locality,
			cmp.config.compare.kind,
			cmp.config.compare.sort_text,
			cmp.config.compare.length,
			cmp.config.compare.order,
		},
	},
})

-- copilot.setup({
-- 	panel = { enabled = false },
-- 	suggestion = {
-- 		enabled = true,
-- 		auto_trigger = true,
-- 		debounce = 75,
-- 		keymap = {
-- 			accept = "<C-l>",
-- 			accept_word = false,
-- 			accept_line = false,
-- 			next = "<C-y>",
-- 			prev = "<C-g>",
-- 			dismiss = "<C-e>",
-- 		},
-- 	},
-- 	filetypes = {
-- 		markdown = false,
-- 		help = false,
-- 		gitcommit = false,
-- 		gitrebase = false,
-- 		hgcommit = false,
-- 		svn = false,
-- 		cvs = false,
-- 		["."] = false,
-- 	},
-- 	copilot_node_command = "node", -- Node.js version must be > 16.x
-- 	server_opts_overrides = {},
-- })

-- local cmp_format = require("copilot_cmp.format")
-- copilot_cmp.setup({
-- 	suggestion = { enabled = true },
-- 	formatters = {
-- 		label = cmp_format.format_label_text,
-- 		insert_text = cmp_format.remove_existing,
-- 		preview = cmp_format.deindent,
-- 	},
-- })

cmp.setup.filetype({ "sql" }, {
	sources = {
		{ name = "vim-dadbod-completion" },
		-- { name = "copilot" },
		{ name = "buffer" },
	},
})
