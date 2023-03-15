-- =============================================================================
-- 🛠️ Simple Setups {{{
require("Comment").setup()
require("trouble").setup()
require("lsp-file-operations").setup()
require("nvim-web-devicons").setup()
require("trim").setup({
	-- disable = {},
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

require("pretty-fold").setup({
	sections = {
		left = {
			"╘╾",
			"content",
			"⮯ ",
		},
		right = {
			" ",
			"number_of_folded_lines",
			": ",
			"percentage",
			" ╼╕",
		},
	},
	fill_char = "⋅",
	process_comment_signs = "delete",
})

vim.cmd([[ colorscheme vagari ]])

-- }}}
-- =============================================================================

-- 🗂️ Nvim-Tree {{{

-- https://github.com/nvim-tree/nvim-tree.lua
require("nvim-tree").setup({
	disable_netrw = false,
	hijack_cursor = true,
	update_focused_file = { enable = true },
	diagnostics = { enable = true },
	modified = { enable = true },
	remove_keymaps = true,
	view = {
		number = true,
		cursorline = false,
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
				local window_w = screen_w * 0.66
				local window_h = screen_h * 0.33
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
-- }}}

-- 🧿 GitSigns {{{
require("gitsigns").setup({
	signs = {
		add = {
			hl = "GitSignsAdd",
			text = "",
			numhl = "GitSignsAddNr",
			linehl = "GitSignsAddLn",
		},
		change = {
			hl = "DiffChange",
			text = "🞊",
			numhl = "DiffChange",
			linehl = "DiffChange",
		},
		untracked = {
			hl = "DiffNew",
			text = "",
			numhl = "DiffNew",
			linehl = "DIffNew",
		},
		delete = {
			hl = "GitSignsDelete",
			text = "",
			numhl = "GitSignsDeleteNr",
			linehl = "GitSignsDeleteLn",
		},
		topdelete = {
			hl = "ErrorMsg",
			text = "🕱",
			numhl = "ErrorMsg",
			linehl = "GitSignsDeleteLn",
		},
		changedelete = {
			hl = "Keyword",
			text = "",
			numhl = "Keyword",
			linehl = "Keyword",
		},
	},
	signcolumn = true, -- Toggle with `:Gitsigns toggle_signs`
	numhl = false, -- Toggle with `:Gitsigns toggle_numhl`
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
-- }}}

-- 🧭 LuaLine {{{

local p = require("vagari.palette")

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
		modified = "🞊 ",
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
			n = p.blu_4,
			i = p.grn_4,
			v = p.prp_4,
			V = p.prp_4,
			c = p.orn_4,
			R = p.emr_4,
			s = p.cyn_4,
			S = p.cyn_4,
			[""] = p.prp_4,
		}
		return { fg = mode_color[vim.fn.mode()] }
	end,
}

local lsp_progress = {
	"lsp_progress",
	display_components = {
		"lsp_client_name",
		"spinner",
	},
	timer = {
		progress_enddelay = 100,
		spinner = 100,
		lsp_client_name_enddelay = 3000,
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
	color = { fg = p.orn_2 },
}

local function icon()
	return [[ ]]
end

require("lualine").setup({
	options = {
		theme = "vagari",
		section_separators = { left = "", right = "" },
		component_separators = { left = "⊸", right = "⟜" },
	},
	sections = {
		lualine_a = { mode },
		lualine_b = { branch, filename },
		lualine_c = { diff, lsp_diagnostics },
		lualine_x = { lsp_progress, search },
		lualine_y = { filetype },
		lualine_z = { icon },
	},
	inactive_sections = {},
	tabline = {},
})
-- }}}

-- 🔭 Telescope {{{

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
				["<c-t>"] = actions.select_tab,
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

local builtin = require("telescope.builtin")
local tmap = function(key, cmd)
	vim.keymap.set("n", key, cmd, {})
end

tmap("<leader>t<leader>", builtin.find_files)
tmap("<leader>g<leader>", builtin.live_grep)
tmap("<leader>ts", builtin.grep_string)
tmap("<leader>tg", builtin.git_files)
tmap("<leader>to", builtin.oldfiles)
tmap("<leader>tb", builtin.buffers)
tmap("<leader>th", builtin.help_tags)
tmap("<leader>tH", builtin.highlights)
tmap("<leader>td", builtin.diagnostics)
tmap("<leader>tp", builtin.builtin)
tmap("<leader>tc", builtin.commands)
tmap("<leader>tl", builtin.loclist)
tmap("<leader>tq", builtin.quickfix)
tmap("<leader>tm", builtin.man_pages)
tmap("<leader>tst", builtin.treesitter)

-- }}}

-- 🎄 Treesitter {{{
require("nvim-treesitter.configs").setup({
	ensure_installed = {
		"bash",
		"css",
		"help",
		"cpp",
		"html",
		"json",
		"lua",
		"markdown",
		"regex",
		"rust",
		"scss",
		"vim",
		"python",
		"toml",
		"yaml",
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
})
-- }}}

-- 🪄 Nvim-Cmp {{{
local cmp = require("cmp")
local luasnip = require("luasnip")

require("luasnip/loaders/from_vscode").lazy_load()

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
				nvim_lua = "",
				nvim_lsp = "",
				luasnip = "",
				buffer = "﬘",
				path = "⨒",
				emoji = "ﲃ",
				nerdfont = "",
			})[entry.source.name]
			return vim_item
		end,
	},
	sources = {
		{ name = "nvim_lsp" },
		{ name = "luasnip" },
		{ name = "path" },
		{ name = "nvim_lua" },
		{ name = "buffer" },
		{ name = "emoji" },
		{ name = "nerdfont" },
	},
	confirm_opts = {
		behavior = cmp.ConfirmBehavior.Replace,
		select = false,
	},
	view = {
		entries = "native",
	},
	experimental = {
		ghost_text = true,
	},
})
-- }}}

-- 📚 LSP (and more) {{{

-- critical on_attach function ⮯
local on_attach = function(client, bufnr)
	local nmap = function(keys, func)
		vim.keymap.set("n", keys, func, { buffer = bufnr })
	end

	if client.supports_method("textDocument/documentHighlight") then
		vim.api.nvim_exec(
			[[
            augroup ReferenceHighlight
                autocmd! * <buffer>
                autocmd CursorHold <buffer> lua vim.lsp.buf.document_highlight()
                autocmd CursorMoved <buffer> lua vim.lsp.buf.clear_references()
            augroup END
        ]],
			false
		)
	end

	nmap("<leader>tsr", require("telescope.builtin").lsp_references)
	nmap("<leader>tss", require("telescope.builtin").lsp_document_symbols)
	nmap("<leader>tsw", require("telescope.builtin").lsp_dynamic_workspace_symbols)
	nmap("<leader>rn", vim.lsp.buf.rename)
	nmap("<leader>ca", vim.lsp.buf.code_action)
	nmap("<leader>gr", vim.lsp.buf.references)
	nmap("gd", vim.lsp.buf.type_definition)
	nmap("<leader>gd", vim.lsp.buf.definition)
	nmap("gi", vim.lsp.buf.implementation)
	nmap("K", vim.lsp.buf.hover)
	nmap("<C-k>", vim.lsp.buf.signature_help)
	nmap("<leader>?", vim.diagnostic.open_float)
	nmap("[d", vim.diagnostic.goto_prev)
	nmap("]d", vim.diagnostic.goto_next)

	if client.supports_method("textDocument/formatting") then
		local augroup = vim.api.nvim_create_augroup("LspFormatting", {})
		vim.api.nvim_clear_autocmds({ group = augroup, buffer = bufnr })
		vim.api.nvim_create_autocmd("BufWritePre", {
			group = augroup,
			buffer = bufnr,
			callback = function()
				vim.lsp.buf.format()
			end,
		})
	end
end

-- ⛑️  Null-ls (Linting, formatting) {{{
local null_ls = require("null-ls")

-- see https://github.com/jose-elias-alvarez/null-ls.nvim/blob/main/doc/BUILTINS.md
local diagnostics = null_ls.builtins.diagnostics
local formatting = null_ls.builtins.formatting
local hover = null_ls.builtins.hover

null_ls.setup({
	sources = {
		-- 🩺 diagnostics: ⮯
		diagnostics.codespell, -- identify some common code related misspellings
		diagnostics.shellcheck, -- shell linter
		diagnostics.zsh, -- zsh -n (somewhat useful)
		diagnostics.stylelint, -- css 'n related linting
		-- 🗃️ formatting: ⮯
		formatting.shellharden, -- bash; goes well with shellcheck linting
		formatting.beautysh, -- zsh 'n more (+ alt bash)
		formatting.jq, -- json
		formatting.prettier, -- webdev 'n stuff daemon for prettier;
		formatting.rustfmt, -- rust
		formatting.stylua, -- lua
		formatting.taplo, -- toml
		-- 🏄 hover: ⮯
		hover.dictionary,
		hover.printenv,
	},
})
-- }}}

-- ♦️  Vim Diagnostic Settings {{{
local signs = {
	{ name = "DiagnosticSignError", text = "" },
	{ name = "DiagnosticSignWarn", text = "" },
	{ name = "DiagnosticSignHint", text = "" },
	{ name = "DiagnosticSignInfo", text = "" },
}

for _, sign in ipairs(signs) do
	vim.fn.sign_define(sign.name, {
		texthl = sign.name,
		text = sign.text,
		numhl = sign.name,
	})
end

local diagnostic_config = {
	underline = false,
	virtual_text = true,
	signs = true,
	update_in_insert = false,
	float = {
		focusable = false,
		style = "minimal",
		border = "rounded",
		source = "always",
		header = "",
		prefix = "",
	},
}

vim.diagnostic.config(diagnostic_config)

vim.lsp.handlers["textDocument/hover"] = vim.lsp.with(vim.lsp.handlers.hover, {
	border = "rounded",
})

vim.lsp.handlers["textDocument/signatureHelp"] = vim.lsp.with(vim.lsp.handlers.signature_help, {
	border = "rounded",
})
-- }}}

-- 🎾 Automatic Server Setup {{{
local servers = {
	bashls = {},
	marksman = {},
	pyright = {},
	prosemd_lsp = {},
	cssls = {},
	rust_analyzer = {},
	taplo = {},
	html = {},
	lua_ls = {
		Lua = {
			workspace = { checkThirdParty = false },
			telemetry = { enable = false },
			diagnostics = { globals = { "vim" } },
			format = { enable = false },
		},
	},
}
-- }}}

-- 🧱 MASON, DO THE THING! ⮯  {{{
require("mason").setup({ ui = { border = "rounded" } })

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
-- }}}

-- }}}

-- 🍐 Nvim-Autopairs {{{
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
-- }}}

-- 👋 Alpha {{{
local alpha = require("alpha")
local dashboard = require("alpha.themes.dashboard")

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
		hl = "@function.macro",
		shrink_margin = false,
		position = "center",
	},
}

local buttons = {
	type = "group",
	val = {
		{
			type = "text",
			val = "Quick Actions",
			opts = {
				hl = "Type",
				position = "center",
			},
		},
		{ type = "padding", val = 2 },
		dashboard.button("r", "  Recent Files", ":Telescope oldfiles <CR>"),
		dashboard.button("t", "  Find file", ":Telescope find_files <CR>"),
		dashboard.button("s", "  Live grep", ":Telescope live_grep <CR>"),
		{ type = "padding", val = 2 },
		dashboard.button("q", "  Quit", ":qa<CR>"),
	},
}

local opts = {
	layout = {
		{ type = "padding", val = 2 },
		header,
		{ type = "padding", val = 3 },
		buttons,
	},
	opts = {
		margin = 5,
	},
}

alpha.setup(opts)
-- }}}
