local status_ok, cmp = pcall(require, "cmp")
if not status_ok then
	return
end

local snip_status_ok, luasnip = pcall(require, "luasnip")
if not snip_status_ok then
	return
end

require("luasnip/loaders/from_vscode").lazy_load()

local t = function(str)
	return vim.api.nvim_replace_termcodes(str, true, true, true)
end

local check_back_space = function()
	local col = vim.fn.col(".") - 1
	if col == 0 or vim.fn.getline("."):sub(col, col):match("%s") then
		return true
	else
		return false
	end
end

_G.tab_complete = function()
	if cmp and cmp.visible() then
		cmp.select_next_item()
	elseif luasnip and luasnip.expand_or_jumpable() then
		return t("<Plug>luasnip-expand-or-jump")
	elseif check_back_space() then
		return t("<Tab>")
	else
		cmp.complete()
	end
	return ""
end
_G.s_tab_complete = function()
	if cmp and cmp.visible() then
		cmp.select_prev_item()
	elseif luasnip and luasnip.jumpable(-1) then
		return t("<Plug>luasnip-jump-prev")
	else
		return t("<S-Tab>")
	end
	return ""
end

vim.api.nvim_set_keymap("i", "<Tab>", "v:lua.tab_complete()", { expr = true })
vim.api.nvim_set_keymap("s", "<Tab>", "v:lua.tab_complete()", { expr = true })
vim.api.nvim_set_keymap("i", "<S-Tab>", "v:lua.s_tab_complete()", { expr = true })
vim.api.nvim_set_keymap("s", "<S-Tab>", "v:lua.s_tab_complete()", { expr = true })

local keymap = require("cmp.utils.keymap")
local feedkeys = require("cmp.utils.feedkeys")

local kind_icons = {
	Text = "",
	Method = "⟜",
	Function = "",
	Constructor = "",
	Field = "",
	Variable = "",
	Class = "",
	Interface = "",
	Module = "",
	Property = "",
	Unit = "",
	Value = "",
	Enum = "",
	Keyword = "",
	Snippet = "",
	Color = "",
	File = "",
	Reference = "",
	Folder = "",
	EnumMember = "",
	Constant = "",
	Struct = "",
	Event = "",
	Operator = "",
	TypeParameter = "",
}

local has_words_before = function()
	local line, col = unpack(vim.api.nvim_win_get_cursor(0))
	return col ~= 0 and vim.api.nvim_buf_get_lines(0, line - 1, line, true)[1]:sub(col, col):match("%s") == nil
end

cmp.setup({
	snippet = {
		expand = function(args)
			luasnip.lsp_expand(args.body) -- For `luasnip` users.
		end,
	},
	mapping = cmp.mapping.preset.insert({
		["<C-u>"] = cmp.mapping.scroll_docs(-2),
		["<C-d>"] = cmp.mapping.scroll_docs(2),
		["<C-Space>"] = cmp.mapping.complete(),
		["<C-e>"] = cmp.mapping.abort(),
		["<CR>"] = cmp.mapping.confirm({ select = true }),
		["<Right>"] = cmp.mapping.confirm({ select = false }),
		["<Tab>"] = cmp.mapping(function(fallback)
			if luasnip.expand_or_jumpable() then
				luasnip.expand_or_jump()
			elseif cmp.visible() then
				cmp.select_next_item()
				cmp.confirm({ select = true })
			elseif has_words_before() then
				cmp.complete()
			else
				fallback()
			end
		end, { "i", "s" }),
		["<S-Tab>"] = cmp.mapping(function(fallback)
			if luasnip.jumpable(-1) then
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
				luasnip = "",
				buffer = "﬘",
				path = "⨒",
				calc = "",
				emoji = "ﲃ",
				greek = "",
				nvim_lua = "",
				nvim_lsp = "",
			})[entry.source.name]
			return vim_item
		end,
	},
	sources = {
		{ name = "path" },
		{ name = "luasnip" },
		{ name = "nvim_lua" },
		{ name = "nvim_lsp" },
		{ name = "buffer" },
		{ name = "calc" },
		{ name = "greek" },
		{ name = "emoji" },
	},
	confirm_opts = {
		behavior = cmp.ConfirmBehavior.Replace,
		select = false,
	},
	window = {
		documentation = {
			border = { "╭", "─", "╮", "│", "╯", "─", "╰", "│" },
		},
	},
	view = {
		entires = "native",
	},
	experimental = {
		ghost_text = true,
	},
})

cmp.setup.cmdline("/", {
	mapping = cmp.mapping.preset.cmdline({
		["<Down>"] = {
			c = function()
				if cmp.visible() then
					cmp.select_next_item()
				else
					feedkeys.call(keymap.t("<Down>"), "n")
				end
			end,
		},
		["<Up>"] = {
			c = function()
				if cmp.visible() then
					cmp.select_prev_item()
				else
					feedkeys.call(keymap.t("<Up>"), "n")
				end
			end,
		},
		["<Right>"] = {
			c = function()
				if cmp.visible() then
					cmp.confirm({ select = true })
				else
					feedkeys.call(keymap.t("<Right>"), "n")
				end
			end,
		},
	}),
	sources = {
		{ name = "buffer" },
	},
})

cmp.setup.cmdline(":", {
	mapping = cmp.mapping.preset.cmdline({
		["<Down>"] = {
			c = function()
				if cmp.visible() then
					cmp.select_next_item()
				else
					feedkeys.call(keymap.t("<Down>"), "n")
				end
			end,
		},
		["<Up>"] = {
			c = function()
				if cmp.visible() then
					cmp.select_prev_item()
				else
					feedkeys.call(keymap.t("<Up>"), "n")
				end
			end,
		},
		["<Right>"] = {
			c = function()
				if cmp.visible() then
					cmp.confirm({ select = true })
				else
					feedkeys.call(keymap.t("<Right>"), "n")
				end
			end,
		},
	}),
	sources = cmp.config.sources({
		{ name = "path" },
	}, {
		{ name = "cmdline" },
	}),
})
