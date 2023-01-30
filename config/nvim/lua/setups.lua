-- ============================================================================
-- 🛠️ Simple Setups: ⮯ {{{
require("neodev").setup()
require("Comment").setup()
require("lsp-file-operations").setup()
require("nvim-web-devicons").setup({ default = true })
require("trim").setup({
	disable = {},
	patterns = {
		-- replace multiple blank lines with a single line ⮯
		[[%s/\(\n\n\n\)\n\+/\1/]],
	},
})
require("nvim-highlight-colors").setup({
	render = "background",
	enable_named_colors = false,
	enable_tailwind = false,
})
-- }}} ⮭
-- ============================================================================

-- 🎨 Colorscheme: ⮯ {{{

require("catppuccin").setup({
	color_overrides = {
		macchiato = {
			rosewater = "#F4DBD6",
			flamingo = "#F0C6C6",
			pink = "#F5BDE6",
			mauve = "#C6A0F6",
			red = "#ED8796",
			maroon = "#EE99A0",
			peach = "#F5A97F",
			yellow = "#EED49F",
			green = "#A6DA95",
			teal = "#8BD5CA",
			sky = "#91D7E3",
			sapphire = "#7DC4E4",
			blue = "#8AADF4",
			lavender = "#B7BDF8",
			text = "#CAD3F5",
			subtext1 = "#B8C0E0",
			subtext0 = "#A5ADCB",
			overlay2 = "#939AB7",
			overlay1 = "#8087A2",
			overlay0 = "#6E738D",
			surface2 = "#5B6078",
			surface1 = "#494D64",
			surface0 = "#363A4F",

			base = "#24273A",
			mantle = "#1E2030",
			crust = "#181926",
		},
	},
})

vim.cmd([[
  colorscheme catppuccin-macchiato
  highlight ColorColumn guibg=#222536
  highlight MsgArea guifg=#4c527b
  highlight Folded guibg=#24283b guifg=#596091
]])
-- }}} #⮭

-- 🗂️ Nvim-Tree ⮯ {{{
require("nvim-tree").setup({
	disable_netrw = true,
	hijack_cursor = true,
	update_focused_file = { enable = true },
	diagnostics = { enable = true },
	modified = { enable = true },
	remove_keymaps = true,
	view = {
		number = true,
		relativenumber = true,
		signcolumn = "yes",
		mappings = {
			custom_only = true,
			list = {
				{ key = { "<CR>", "o", "<Right>" }, action = "edit" },
				{ key = "zz", action = "cd" },
				{ key = "<Up>", action = "prev_sibling" },
				{ key = "<Down>", action = "next_sibling" },
				{ key = "<Left>", action = "parent_node" },
				{ key = "<C-v>", action = "vsplit" },
				{ key = "<C-h>", action = "split" },
				{ key = "<C-t>", action = "tabnew" },
				{ key = "zc", action = "close_node" },
				{ key = "I", action = "toggle_git_ignored" },
				{ key = ".", action = "toggle_dotfiles" },
				{ key = "n", action = "create" },
				{ key = "x", action = "trash" },
				{ key = "X", action = "remove" },
				{ key = "r", action = "rename" },
				{ key = "<C-r>", action = "full_rename" },
				{ key = "R", action = "refresh" },
				{ key = "d", action = "cut" },
				{ key = "yy", action = "copy" },
				{ key = "p", action = "paste" },
				{ key = "yp", action = "copy_path" },
				{ key = "yP", action = "copy_absolute_path" },
				{ key = "[", action = "prev_git_item" },
				{ key = "]", action = "next_git_item" },
				{ key = "O", action = "system_open" },
				{ key = { "q", "<Esc>" }, action = "close" },
				{ key = "?", action = "toggle_help" },
				{ key = "zm", action = "collapse_all" },
				{ key = "zr", action = "expand_all" },
				{ key = "S", action = "search_node" },
				{ key = "<C-k>", action = "toggle_file_info" },
			},
		},
		float = {
			enable = true,
			open_win_config = function()
				local screen_w = vim.opt.columns:get()
				local screen_h = vim.opt.lines:get() - vim.opt.cmdheight:get()
				local window_w = screen_w * 0.8
				local window_h = screen_h * 0.5
				local window_w_int = math.floor(window_w)
				local window_h_int = math.floor(window_h)
				local center_x = (screen_w - window_w) / 2
				local center_y = ((vim.opt.lines:get() - window_h) / 2) - vim.opt.cmdheight:get()
				return {
					border = "rounded",
					relative = "editor",
					row = center_y,
					col = center_x,
					width = window_w_int,
					height = window_h_int,
				}
			end,
		},
	},
	renderer = {
		highlight_git = true,
		indent_markers = {
			enable = true,
			inline_arrows = true,
			icons = {
				corner = "└╾",
				edge = "│ ",
				item = "├",
				none = " ",
			},
		},
		icons = {
			glyphs = {
				default = "",
				symlink = "",
				git = {
					unstaged = "🞊",
					staged = "",
					unmerged = "",
					renamed = "",
					deleted = "",
					untracked = "",
					ignored = "",
				},
			},
		},
	},
	trash = {
		cmd = "trash-put",
		require_confirm = true,
	},
	actions = {
		open_file = {
			quit_on_open = true,
			window_picker = {
				chars = "asetniol",
			},
		},
	},
})
-- }}} ⮭

-- 🧿 GitSigns: ⮯ {{{
require("gitsigns").setup({
	signs = {
		add = {
			hl = "GitSignsAdd",
			text = "╞+",
			numhl = "GitSignsAddNr",
			linehl = "GitSignsAddLn",
		},
		change = {
			hl = "GitSignsChange",
			text = "╞⚬",
			numhl = "GitSignsChangeNr",
			linehl = "GitSignsChangeLn",
		},
		untracked = {
			hl = "GitSignsChange",
			text = "╞",
			numhl = "GitSignsChangeNr",
			linehl = "GitSignsChangeLn",
		},
		delete = {
			hl = "GitSignsDelete",
			text = "╞-",
			numhl = "GitSignsDeleteNr",
			linehl = "GitSignsDeleteLn",
		},
		topdelete = {
			hl = "GitSignsDelete",
			text = "⮭",
			numhl = "GitSignsDeleteNr",
			linehl = "GitSignsDeleteLn",
		},
		changedelete = {
			hl = "GitSignsDelete",
			text = "╞±",
			numhl = "GitSignsDeleteNr",
			linehl = "GitSignsChangeLn",
		},
	},
	signcolumn = true, -- Toggle with `:Gitsigns toggle_signs`
	numhl = true, -- Toggle with `:Gitsigns toggle_numhl`
	linehl = false, -- Toggle with `:Gitsigns toggle_linehl`
	word_diff = false, -- Toggle with `:Gitsigns toggle_word_diff`
	current_line_blame = false, -- Toggle with `:Gitsigns toggle_current_line_blame`
	preview_config = {
		-- Options passed to nvim_open_win
		border = "rounded",
		style = "minimal",
		relative = "cursor",
		row = 0,
		col = 1,
	},
	yadm = {
		enable = false,
	},
	-- Keymaps
	on_attach = function(bufnr)
		local function map(mode, lhs, rhs, opts)
			opts = vim.tbl_extend("force", { noremap = true, silent = true }, opts or {})
			vim.api.nvim_buf_set_keymap(bufnr, mode, lhs, rhs, opts)
		end

		-- Navigation
		map("n", "]]", "&diff ? ']c' : '<cmd>Gitsigns next_hunk<CR>'", { expr = true })
		map("n", "[[", "&diff ? '[c' : '<cmd>Gitsigns prev_hunk<CR>'", { expr = true })

		-- Actions
		map("n", "<leader>hs", ":Gitsigns stage_hunk<CR>")
		map("n", "<leader>hr", ":Gitsigns reset_hunk<CR>")
		map("n", "<leader>hS", "<cmd>Gitsigns stage_buffer<CR>")
		map("n", "<leader>hu", "<cmd>Gitsigns undo_stage_hunk<CR>")
		map("n", "<leader>hR", "<cmd>Gitsigns reset_buffer<CR>")
		map("n", "<leader>hp", "<cmd>Gitsigns preview_hunk<CR>")
		map("n", "<leader>hb", '<cmd>lua require"gitsigns".blame_line{full=true}<CR>')
		map("n", "<leader>lb", "<cmd>Gitsigns toggle_current_line_blame<CR>")
		map("n", "<leader>hd", "<cmd>Gitsigns toggle_deleted<CR>")

		-- Text object
		map("o", "ih", ":<C-U>Gitsigns select_hunk<CR>")
		map("x", "ih", ":<C-U>Gitsigns select_hunk<CR>")
	end,
})
-- }}} ⮭

-- 🧭 LuaLine: ⮯ {{{

local colors = require("tokyonight.colors").setup({})
local utils = require("tokyonight.util")
local fg_gutter_alt = "#2a2f46"

local tokyonight = {
	normal = {
		a = { bg = colors.blue, fg = colors.black },
		b = { bg = fg_gutter_alt, fg = colors.blue },
		c = { bg = colors.bg_statusline, fg = colors.blue7 },
	},
	insert = {
		a = { bg = colors.green, fg = colors.black },
		b = { bg = fg_gutter_alt, fg = colors.green },
	},
	command = {
		a = { bg = colors.yellow, fg = colors.black },
		b = { bg = fg_gutter_alt, fg = colors.yellow },
	},
	visual = {
		a = { bg = colors.magenta, fg = colors.black },
		b = { bg = fg_gutter_alt, fg = colors.magenta },
	},
	replace = {
		a = { bg = colors.red, fg = colors.black },
		b = { bg = fg_gutter_alt, fg = colors.red },
	},
	inactive = {
		a = { bg = colors.bg_statusline, fg = colors.blue },
		b = { bg = colors.bg_statusline, fg = fg_gutter_alt, gui = "bold" },
		c = { bg = colors.bg_statusline, fg = fg_gutter_alt },
	},
}

local hide_in_width = function()
	return vim.fn.winwidth(0) > 80
end

local branch = {
	"branch",
	icon = "",
}

-- see lualine.nvim/lua/lualine/utils/mode.lua for mode names
local modes = {
	["NORMAL"] = "",
	["O-PENDING"] = "",
	["INSERT"] = "",
	["VISUAL"] = "",
	["SELECT"] = "",
	["V-LINE"] = "",
	["V-BLOCK"] = "",
	["COMMAND"] = "",
	["REPLACE"] = "",
}

local mode = {
	"mode",
	fmt = function(str)
		if modes[str] then
			return modes[str]
		end
		return str
	end,
}

local lsp_diagnostics = {
	"diagnostics",

	sources = { "nvim_diagnostic" },
	sections = { "error", "warn", "hint", "info" },
	symbols = { error = " ", warn = " ", hint = " ", info = " " },
	colored = false,
	update_in_insert = false,
	padding = { left = 1, right = 1 },
	cond = hide_in_width,
}

local diff = {
	"diff",
	colored = true,
	symbols = {
		added = " ",
		modified = " ",
		removed = " ",
	},
	cond = hide_in_width,
}

local filetype = {
	"filetype",
	colored = false,
	icon_only = false,
	padding = { left = 0, right = 1 },
}

local filename = {
	"filename",
	file_status = true,
	path = 0,
	icon = "",
	symbols = {
		modified = "🞊",
		readonly = "",
		unnamed = "名前?",
	},
	color = function()
		local mode_color = {
			n = utils.lighten(colors.blue, 0.75),
			i = utils.lighten(colors.green, 0.75),
			v = utils.lighten(colors.magenta, 0.75),
			V = utils.lighten(colors.magenta, 0.75),
			c = utils.lighten(colors.orange, 0.75),
			R = utils.lighten(colors.red, 0.75),
			s = utils.lighten(colors.teal, 0.75),
			S = utils.lighten(colors.teal, 0.75),
			[""] = utils.lighten(colors.magenta, 0.75),
		}
		return { fg = mode_color[vim.fn.mode()] }
	end,
}

local lsp = {
	"lsp_progress",
	display_components = {
		"lsp_client_name",
		"spinner",
	},
	timer = {
		progress_enddelay = 100,
		spinner = 100,
		lsp_client_name_enddelay = 1000,
	},
	message = {
		commenced = "󱞇 ",
		completed = "󱞈 ",
	},
	separators = {
		component = " ",
		progress = "",
		message = { pre = "", post = " " },
		percentage = { pre = "", post = "%% " },
		title = { pre = "", post = "" },
		lsp_client_name = { pre = " ", post = "" },
		spinner = { pre = "", post = "" },
	},
	spinner_symbols = { " ", " ", " ", " ", " ", " " },
}

local buffers = {
	"buffers",
	max_length = vim.o.columns * 10 / 11,
	cond = hide_in_width,
}

local tsplayground = {
	sections = {
		lualine_b = { filename },
		lualine_z = { filetype },
	},
	inactive_sections = {
		lualine_z = { filetype },
	},
	filetypes = { "tsplayground" },
}

local search = {
	function()
		local last_search = vim.fn.getreg("/")
		if not last_search or last_search == "" then
			return ""
		end
		local searchcount = vim.fn.searchcount({ maxcount = 9999 })
		if searchcount.total == 0 then
			vim.cmd([[ :call setreg("/", [''])<CR> ]])
		end
		return " " .. last_search .. "(" .. searchcount.current .. "/" .. searchcount.total .. ")"
	end,
	color = { fg = utils.darken(colors.orange, 0.69) },
}

local function icon()
	return [[ ]]
end

require("lualine").setup({
	options = {
		icons_enabled = true,
		theme = tokyonight,
		section_separators = { left = "", right = "" },
		component_separators = { left = "⊸", right = "⟜" },
		disabled_filetypes = {},
		always_divide_middle = true,
		globalstatus = false,
	},
	sections = {
		lualine_a = { mode },
		lualine_b = { branch, filename },
		lualine_c = { diff, lsp_diagnostics },
		lualine_x = { lsp, search },
		lualine_y = { filetype },
		lualine_z = { icon },
	},
	inactive_sections = {
		lualine_a = { "" },
		lualine_b = { buffers },
		lualine_c = { "" },
		lualine_x = {},
		lualine_y = { "" },
		lualine_z = { "filetype" },
	},
	tabline = {},
	extensions = {
		"nvim-tree",
		"quickfix",
		"toggleterm",
		tsplayground,
	},
})
-- }}} ⮭

-- 🔭 Telescope: ⮯ {{{

local actions = require("telescope.actions")

require("telescope").setup({
	defaults = {
		prompt_prefix = "   ",
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
				["<gt>"] = actions.select_tab,
				["<Tab>"] = actions.toggle_selection + actions.move_selection_worse,
				["<S-Tab>"] = actions.toggle_selection + actions.move_selection_better,
				["<C-f>"] = actions.send_to_qflist + actions.open_qflist,
				["<C-q>"] = actions.send_selected_to_qflist + actions.open_qflist,
				["<C-l>"] = actions.complete_tag,
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

require("telescope").load_extension("zf-native")

local map = vim.keymap.set
map("n", "<leader>t<leader>", require("telescope.builtin").find_files)
map("n", "<leader>tt", require("telescope.builtin").live_grep)
map("n", "<leader>TT", require("telescope.builtin").grep_string)
map("n", "<leader>tg", require("telescope.builtin").git_files)
map("n", "<leader>to", require("telescope.builtin").oldfiles)
map("n", "<leader>tb", require("telescope.builtin").buffers)
map("n", "<leader>th", require("telescope.builtin").help_tags)
map("n", "<leader>td", require("telescope.builtin").diagnostics)
map("n", "[d", vim.diagnostic.goto_prev)
map("n", "]d", vim.diagnostic.goto_next)
map("n", "<leader>tp", require("telescope.builtin").builtin)
map("n", "<leader>tc", require("telescope.builtin").commands)
map("n", "<leader>tl", require("telescope.builtin").loclist)
map("n", "<leader>tq", require("telescope.builtin").quickfix)
map("n", "<leader>tm", require("telescope.builtin").man_pages)
map("n", "<leader>tst", require("telescope.builtin").treesitter)
-- }}} ⮭

-- 🎄 Treesitter: ⮯ {{{
require("nvim-treesitter.configs").setup({
	ensure_installed = {
		"lua",
		"help",
		"vim",
		"go",
		"scss",
		"yaml",
		"css",
		"html",
		"python",
		"json",
		"rust",
		"bash",
		"markdown",
		"regex",
		"solidity",
	},
	sync_install = true,
	auto_install = true,
	highlight = {
		enable = true,
		disable = function(lang, buf)
			local max_filesize = 100 * 1024 -- 100 KB
			local ok, stats = pcall(vim.loop.fs_stat, vim.api.nvim_buf_get_name(buf))
			if ok and stats and stats.size > max_filesize then
				return true
			end
		end,
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
})
-- }}} ⮭

-- 🪄 Nvim-Cmp: ⮯ {{{
local cmp = require("cmp")
local luasnip = require("luasnip")

require("luasnip/loaders/from_vscode").lazy_load()

local check_backspace = function()
	local col = vim.fn.col(".") - 1
	return col == 0 or vim.fn.getline("."):sub(col, col):match("%s")
end

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

cmp.setup({
	snippet = {
		expand = function(args)
			luasnip.lsp_expand(args.body) -- For `luasnip` users.
		end,
	},
	mapping = cmp.mapping.preset.insert({
		["<C-u>"] = cmp.mapping.scroll_docs(-1),
		["<C-d>"] = cmp.mapping.scroll_docs(1),
		["<C-Space>"] = cmp.mapping.complete(),
		["<C-e>"] = cmp.mapping.abort(),
		["<CR>"] = cmp.mapping.confirm({ select = true }),
		["<Right>"] = cmp.mapping.confirm({ select = false }),
		["<Tab>"] = cmp.mapping(function(fallback)
			if cmp.visible() then
				cmp.select_next_item()
			elseif luasnip.expandable() then
				luasnip.expand()
			elseif luasnip.expand_or_jumpable() then
				luasnip.expand_or_jump()
			elseif check_backspace() then
				fallback()
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
		{ name = "emoji" },
		{ name = "calc" },
		{ name = "greek" },
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
		entries = "native",
	},
	experimental = {
		ghost_text = true,
	},
})
-- }}} ⮭

-- 📚 LSP Settings: ⮯ {{{
-- ============================================================================
-- ⛑️  NULL-LS, PREP THE THING! ⮯ {{{
local null_ls = require("null-ls")

-- see https://github.com/jose-elias-alvarez/null-ls.nvim/blob/main/doc/BUILTINS.md
local diagnostics = null_ls.builtins.diagnostics
local formatting = null_ls.builtins.formatting
local hover = null_ls.builtins.hover

null_ls.setup({
	sources = {
		-- 🩺 diagnostics: ⮯
		diagnostics.codespell, -- identify some common code related misspellings
		diagnostics.ruff, -- python linter
		diagnostics.shellcheck, -- shell linter
		diagnostics.stylelint, -- css 'n related linting
		diagnostics.tidy, -- html/xml linting
		-- 🗃️ formatting: ⮯
		formatting.shellharden, -- bash; goes well with shellcheck linting
		formatting.black, -- python
		formatting.jq, -- json
		formatting.prettier, -- webdev 'n stuff daemon for prettier;
		formatting.rustfmt, -- rust
		formatting.stylua, -- lua
		-- 🏄 hover: ⮯
		hover.dictionary,
		hover.printenv,
	},
})

local lsp_formatting = function(bufnr)
	vim.lsp.buf.format({
		filter = function(client)
			-- apply whatever logic you want (in this example, we'll only use null-ls)
			return client.name == "null-ls"
		end,
		bufnr = bufnr,
	})
end

-- if you want to set up formatting on save, you can use this as a callback
local augroup = vim.api.nvim_create_augroup("LspFormatting", {})
-- }}} ⮭
-- ============================================================================

-- critical on_attach function, applied by mason ⮯
local on_attach = function(client, bufnr)
	-- only apply keymaps if lsp exist
	local nmap = function(keys, func)
		vim.keymap.set("n", keys, func, { buffer = bufnr })
	end
	nmap("<leader>tsr", require("telescope.builtin").lsp_references)
	nmap("<leader>tss", require("telescope.builtin").lsp_document_symbols)
	nmap("<leader>tsw", require("telescope.builtin").lsp_dynamic_workspace_symbols)
	nmap("<leader>rn", vim.lsp.buf.rename)
	nmap("<leader>ca", vim.lsp.buf.code_action)
	nmap("gd", vim.lsp.buf.definition)
	nmap("<leader>gd", vim.lsp.buf.type_definition)
	nmap("gi", vim.lsp.buf.implementation)
	nmap("K", vim.lsp.buf.hover)
	nmap("<C-k>", vim.lsp.buf.signature_help)

	-- apply null-ls auto format on save ⮯
	if client.supports_method("textDocument/formatting") then
		vim.api.nvim_clear_autocmds({ group = augroup, buffer = bufnr })
		vim.api.nvim_create_autocmd("BufWritePre", {
			group = augroup,
			buffer = bufnr,
			callback = function()
				lsp_formatting(bufnr)
			end,
		})
	end
end

-- ensure installed servers, and adjust settings if desired  ⮯
local servers = {

	sumneko_lua = {
		Lua = {
			workspace = { checkThirdParty = false },
			telemetry = { enable = false },
			diagnostics = { globals = { "vim" } },
		},
	},
}

-- ============================================================================
-- 🧱 MASON, DO THE THING! ⮯ {{{
require("mason").setup({ ui = { border = "rounded" } })

-- Ensure the servers above are installed
local mason_lspconfig = require("mason-lspconfig")
local capabilities = vim.lsp.protocol.make_client_capabilities()
capabilities = require("cmp_nvim_lsp").default_capabilities(capabilities)

mason_lspconfig.setup({
	ensure_installed = vim.tbl_keys(servers),
})

mason_lspconfig.setup_handlers({
	function(server_name)
		require("lspconfig")[server_name].setup({
			capabilities = capabilities,
			on_attach = on_attach,
			settings = servers[server_name],
		})
	end,
})
-- }}} ⮭
-- ============================================================================
-- }}} ⮭

-- 🍐 Nvim-Autopairs: ⮯ {{{
local npairs = require("nvim-autopairs")
local ts_conds = require("nvim-autopairs.ts-conds")
local Rule = require("nvim-autopairs.rule")
local cmp_autopairs = require("nvim-autopairs.completion.cmp")

npairs.setup({
	enable_check_bracket_line = false,
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
		javascript = { "template_string" },
		java = false, -- don't check treesitter on java
	},
})

-- press % => %% only while inside a comment or string
npairs.add_rules({
	Rule("%", "%", "lua"):with_pair(ts_conds.is_ts_node({ "string", "comment" })),
	Rule("$", "$", "lua"):with_pair(ts_conds.is_not_ts_node({ "function" })),
})

cmp.event:on(
	"confirm_done",
	cmp_autopairs.on_confirm_done({
		map_char = { tex = "" },
	})
)
-- }}} ⮭

-- 👋 Alpha: ⮯ {{{
local alpha = require("alpha")
local dashboard = require("alpha.themes.dashboard")
local plenary_path = require("plenary.path")
local cdir = vim.fn.fnamemodify(vim.fn.getcwd(), ":~")
local if_nil = vim.F.if_nil

local nvim_web_devicons = {
	enabled = true,
	highlight = true,
}

local function get_extension(fn)
	local match = fn:match("^.+(%..+)$")
	local ext = ""
	if match ~= nil then
		ext = match:sub(2)
	end
	return ext
end

local function alpha_icon(fn)
	local nwd = require("nvim-web-devicons")
	local ext = get_extension(fn)
	return nwd.get_icon(fn, ext, { default = true })
end

local function file_button(fn, sc, short_fn)
	short_fn = short_fn or fn
	local ico_txt
	local fb_hl = {}

	if nvim_web_devicons.enabled then
		local ico, hl = alpha_icon(fn)
		local hl_option_type = type(nvim_web_devicons.highlight)
		if hl_option_type == "boolean" then
			if hl and nvim_web_devicons.highlight then
				table.insert(fb_hl, { hl, 0, 3 })
			end
		end
		if hl_option_type == "string" then
			table.insert(fb_hl, { nvim_web_devicons.highlight, 0, 3 })
		end
		ico_txt = ico .. "  "
	else
		ico_txt = ""
	end
	local file_button_el = dashboard.button(sc, ico_txt .. short_fn, "<cmd>e " .. fn .. " <CR>")
	local fn_start = short_fn:match(".*[/\\]")
	if fn_start ~= nil then
		table.insert(fb_hl, { "Comment", #ico_txt - 2, #fn_start + #ico_txt })
	end
	file_button_el.opts.hl = fb_hl
	return file_button_el
end

local default_mru_ignore = { "gitcommit" }

local mru_opts = {
	ignore = function(path, ext)
		return (string.find(path, "COMMIT_EDITMSG")) or (vim.tbl_contains(default_mru_ignore, ext))
	end,
}

local function mru(start, cwd, items_number, opts)
	opts = opts or mru_opts
	items_number = if_nil(items_number, 10)

	local oldfiles = {}
	for _, v in pairs(vim.v.oldfiles) do
		if #oldfiles == items_number then
			break
		end
		local cwd_cond
		if not cwd then
			cwd_cond = true
		else
			cwd_cond = vim.startswith(v, cwd)
		end
		local ignore = (opts.ignore and opts.ignore(v, get_extension(v))) or false
		if (vim.fn.filereadable(v) == 1) and cwd_cond and not ignore then
			oldfiles[#oldfiles + 1] = v
		end
	end
	local target_width = 35

	local tbl = {}
	for i, fn in ipairs(oldfiles) do
		local short_fn
		if cwd then
			short_fn = vim.fn.fnamemodify(fn, ":.")
		else
			short_fn = vim.fn.fnamemodify(fn, ":~")
		end

		if #short_fn > target_width then
			short_fn = plenary_path.new(short_fn):shorten(1, { -2, -1 })
			if #short_fn > target_width then
				short_fn = plenary_path.new(short_fn):shorten(1, { -1 })
			end
		end

		local shortcut = tostring(i + start - 1)

		local file_button_el = file_button(fn, shortcut, short_fn)
		tbl[i] = file_button_el
	end
	return {
		type = "group",
		val = tbl,
		opts = {},
	}
end

local function mru_title()
	return "MRU פּ " .. cdir
end

local header = {
	type = "text",
	val = {
		[[                                __                 ]],
		[[   ___     ___    ___   __  __ /\_\    ___ ___     ]],
		[[  / _ `\  / __`\ / __`\/\ \/\ \\/\ \  / __` __`\   ]],
		[[ /\ \/\ \/\  __//\ \_\ \ \ \_/ |\ \ \/\ \/\ \/\ \  ]],
		[[ \ \_\ \_\ \____\ \____/\ \___/  \ \_\ \_\ \_\ \_\ ]],
		[[  \/_/\/_/\/____/\/___/  \/__/    \/_/\/_/\/_/\/_/ ]],
		[[                                                   ]],
		[[  _.--'"`'--._    _.--'"`'--._    _.--'"`'--._     ]],
		[[ :`.'|`|"':-.  '-:`.'|`|"':-.  '-:`.'|`|"':-.  '-  ]],
		[[ '.  | |  | |'.  '.  | |  | |'.  '.  | |  | |'.    ]],
		[[ . '.| |  | |  '.  '.| |  | |  '.  '.| |  | |  '.  ]],
		[[  `. `.:_ | :_.' '.  `.:_ | :_.' '.  `.:_ | :_.'   ]],
		[[    `-..,..-'       `-..,..-'       `-..,..-'      ]],
	},
	opts = {
		hl = "Constant",
		shrink_margin = false,
		position = "center",
	},
}

local section_mru = {
	type = "group",
	val = {
		{
			type = "text",
			val = mru_title(),
			opts = {
				hl = "Function",
				shrink_margin = false,
				position = "center",
			},
		},
		{ type = "padding", val = 1 },
		{
			type = "group",
			val = function()
				return { mru(1, cdir, 9) }
			end,
			opts = { shrink_margin = false },
		},
	},
}

local buttons = {
	type = "group",
	val = {
		{ type = "text", val = "Quick Actions", opts = { hl = "Function", position = "center" } },
		{ type = "padding", val = 1 },
		dashboard.button("t", "  Find file", ":Telescope find_files <CR>"),
		dashboard.button("/", "  Live grep", ":Telescope live_grep <CR>"),
		dashboard.button("r", "  Recently used files", ":Telescope oldfiles <CR>"),
		dashboard.button("q", "  Quit neovim", ":qa<CR>"),
	},
}

local opts = {
	layout = {
		{ type = "padding", val = 2 },
		header,
		{ type = "padding", val = 2 },
		section_mru,
		{ type = "padding", val = 2 },
		buttons,
	},
	opts = {
		margin = 5,
	},
}

alpha.setup(opts)
-- }}} ⮭
