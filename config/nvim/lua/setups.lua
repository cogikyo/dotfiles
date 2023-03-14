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
-- }}}
-- =============================================================================

-- 🎨 Colorscheme  {{
-- vim.cmd([[ colorscheme vagari ]])

-- (TEMP) palette {{{
local p = {
	bg = "#222536", -- CR: 1.00
	fg = "#aeb9f8", -- CR: 8.00

	drk_0 = "#1e2133", -- CR: 1.05
	drk_1 = "#181b2c", -- CR: 1.12
	drk_2 = "#131626", -- CR: 1.18
	drk_3 = "#0d0f1b", -- CR: 1.26
	brt_0 = "#b6c0f7", -- CR: 8.56
	brt_1 = "#bec6f8", -- CR: 9.11
	brt_2 = "#cad1fb", -- CR: 10.12
	brt_3 = "#d1d8ff", -- CR: 11.21

	blk_0 = "#1d1d21", -- CR: 1.11
	blk_1 = "#16161a", -- CR: 1.19
	blk_2 = "#101014", -- CR: 1.25
	blk_3 = "#0b0b0f", -- CR: 1.30
	wht_0 = "#c9c9e2", -- CR: 9.33
	wht_1 = "#d3d3ed", -- CR: 10.41
	wht_2 = "#ddddf6", -- CR: 11.36
	wht_3 = "#e9e9fb", -- CR: 12.63

	blu_0 = "#4a6be3", -- CR: 3.24
	blu_1 = "#6380ec", -- CR: 4.20
	blu_2 = "#7492ef", -- CR: 5.12
	blu_3 = "#8aa4f3", -- CR: 6.26
	blu_4 = "#9db2f4", -- CR: 7.29
	orn_0 = "#e56b2c", -- CR: 4.66
	orn_1 = "#ea834b", -- CR: 5.66
	orn_2 = "#eb905d", -- CR: 6.26
	orn_3 = "#f2a170", -- CR: 7.29
	orn_4 = "#f8b486", -- CR: 8.56

	grn_0 = "#5b9542", -- CR: 4.20
	grn_1 = "#73ad5a", -- CR: 5.66
	grn_2 = "#85ba6d", -- CR: 6.67
	grn_3 = "#95cb79", -- CR: 8.00
	grn_4 = "#9fd883", -- CR: 9.11
	prp_0 = "#7f61cd", -- CR: 3.24
	prp_1 = "#9376d8", -- CR: 4.20
	prp_2 = "#a188df", -- CR: 5.12
	prp_3 = "#b29ae8", -- CR: 6.26
	prp_4 = "#bdaaeb", -- CR: 7.29

	emr_0 = "#2d9a43", -- CR: 4.20
	emr_1 = "#48a95b", -- CR: 5.12
	emr_2 = "#5aba6d", -- CR: 6.26
	emr_3 = "#5fc976", -- CR: 7.29
	emr_4 = "#76d78b", -- CR: 8.56
	rby_0 = "#f34658", -- CR: 4.20
	rby_1 = "#f36978", -- CR: 5.12
	rby_2 = "#f07a88", -- CR: 5.66
	rby_3 = "#f08898", -- CR: 6.26
	rby_4 = "#f29ca9", -- CR: 7.29

	sun_0 = "#f3a338", -- CR: 7.29
	sun_1 = "#f5b855", -- CR: 8.56
	sun_2 = "#f5c069", -- CR: 9.11
	sun_3 = "#f4ce88", -- CR: 10.12
	sun_4 = "#f5d599", -- CR: 10.72
	sky_0 = "#369fd7", -- CR: 5.12
	sky_1 = "#54b0e2", -- CR: 6.26
	sky_2 = "#6bbdec", -- CR: 7.29
	sky_3 = "#7cc5ef", -- CR: 8.00
	sky_4 = "#90d1f5", -- CR: 9.11

	cyn_0 = "#2bb198", -- CR: 5.66
	cyn_1 = "#30c9b0", -- CR: 7.29
	cyn_2 = "#38d2ba", -- CR: 8.00
	cyn_3 = "#50dec8", -- CR: 9.11
	cyn_4 = "#75e6d5", -- CR: 10.12
	pnk_0 = "#d15da6", -- CR: 4.20
	pnk_1 = "#e36cb8", -- CR: 5.12
	pnk_2 = "#ea76c0", -- CR: 5.66
	pnk_3 = "#e887c3", -- CR: 6.26
	pnk_4 = "#ed9acd", -- CR: 7.29

	his_0 = "#824141", -- CR: 2.00
	his_1 = "#b15e5b", -- CR: 3.33
	his_2 = "#c67a79", -- CR: 4.66
	his_3 = "#d09490", -- CR: 6.00
	glu_0 = "#3b557c", -- CR: 2.00
	glu_1 = "#5b77a4", -- CR: 3.33
	glu_2 = "#7690b9", -- CR: 4.66
	glu_3 = "#90a4c7", -- CR: 6.00
	asn_0 = "#6f447a", -- CR: 2.00
	asn_1 = "#9961a7", -- CR: 3.33
	asn_2 = "#af7dba", -- CR: 4.66
	asn_3 = "#c193cd", -- CR: 6.00
	pro_0 = "#505831", -- CR: 2.00
	pro_1 = "#717b45", -- CR: 3.33
	pro_2 = "#8a945b", -- CR: 4.66
	pro_3 = "#9ea876", -- CR: 6.00
	tyr_0 = "#375c47", -- CR: 2.00
	tyr_1 = "#4b8163", -- CR: 3.33
	tyr_2 = "#5a9c78", -- CR: 4.66
	tyr_3 = "#72b08e", -- CR: 6.00

	gry_0 = "#373945", -- CR: 1.32
	gry_1 = "#3f414f", -- CR: 1.50
	gry_2 = "#4f5163", -- CR: 1.94
	gry_3 = "#5f6278", -- CR: 2.52
	gry_4 = "#6d7089", -- CR: 3.12
	gry_5 = "#7f8199", -- CR: 3.97
	slt_0 = "#343852", -- CR: 1.32
	slt_1 = "#3c4162", -- CR: 1.53
	slt_2 = "#484e75", -- CR: 1.89
	slt_3 = "#585f8d", -- CR: 2.47
	slt_4 = "#686ea1", -- CR: 3.12
	slt_5 = "#7b7fb0", -- CR: 3.97
	glc_0 = "#252841", -- CR: 1.05
	glc_1 = "#282b48", -- CR: 1.10
	glc_2 = "#30345a", -- CR: 1.27
	glc_3 = "#3f4578", -- CR: 1.68
	glc_4 = "#4e5492", -- CR: 2.16
	glc_5 = "#5a61aa", -- CR: 2.68
	rst_0 = "#493531", -- CR: 1.32
	rst_1 = "#563e39", -- CR: 1.54
	rst_2 = "#694b44", -- CR: 1.94
	rst_3 = "#805a52", -- CR: 2.52
	rst_4 = "#92675d", -- CR: 3.11
	rst_5 = "#a3786d", -- CR: 3.97
	plm_0 = "#453445", -- CR: 1.32
	plm_1 = "#523c52", -- CR: 1.53
	plm_2 = "#634a64", -- CR: 1.96
	plm_3 = "#765a77", -- CR: 2.52
	plm_4 = "#876888", -- CR: 3.15
	plm_5 = "#977998", -- CR: 3.97
	olv_0 = "#383a39", -- CR: 1.32
	olv_1 = "#414342", -- CR: 1.51
	olv_2 = "#505251", -- CR: 1.92
	olv_3 = "#5f6361", -- CR: 2.48
	olv_4 = "#6e7270", -- CR: 3.10
	olv_5 = "#818382", -- CR: 3.97
}
-- }}}

-- (TEMP) setup {{{
local hl = {
	syntax = {},
	builtin = {},
	treesitter = {},
	treesitter_queries = {},
	lsp_kind_icons = {},
	filetypes = {},
	plugins = {},
}
local function vim_highlights(hl_groups)
	for group_name, group_settings in pairs(hl_groups) do
		vim.api.nvim_command(
			string.format(
				"highlight %s guifg=%s guibg=%s guisp=%s gui=%s",
				group_name,
				group_settings.fg or "none",
				group_settings.bg or "none",
				group_settings.sp or "none",
				group_settings.fmt or "none"
			)
		)
	end
end

-- }}}

-- (TEMP) thalamus (link relay station) {{{
local t = {

	txt = {
		norm = { fg = p.fg, bg = p.bg },
		minor = { fg = p.slt_5 },
		bold = { fmt = "bold" },
		italic = { fg = p.blu_4, fmt = "italic" },
		underline = { fg = p.brt_0, sp = p.brt_2, fmt = "underline" },
		strike = { fg = p.slt_4, fmt = "strikethrough" },
		title = { fg = p.sky_2, fmt = "bold" },
		reverse = { fg = p.bg, bg = p.brt_1 },
		inactive = { fg = p.gry_1 },
	},

	passive = {
		comment = { fg = p.slt_2 },
		invis = { fg = p.bg, bg = p.bg },
		disown = { fg = p.blu_0, bg = p.drk_1 },
		norm = { fg = p.glc_4, bg = p.drk_0 },
		fg = { fg = p.glc_4 },
		bg = { bg = p.drk_0 },
	},

	idle = {
		norm = { fg = p.blu_2 },
		solid = { bg = p.blu_2 },
		bg = { bg = p.glc_2 },
		passive = { fg = p.blu_2, bg = p.drk_0 },
		passive_br = { fg = p.blu_2, bg = p.glc_0 },
		invis_br = { fg = p.glc_0, bg = p.glc_0 },
		bold = { fg = p.blu_2, fmt = "bold" },
		search = { fg = p.blu_0 },
		ref = { fg = p.blu_0, fmt = "bold" },
	},

	active = {
		norm = { fg = p.orn_4 },
		select = { fg = p.orn_4, bg = p.glc_2, fmt = "bold" },
		visual = { bg = p.glc_2, fmt = "bold" },
		search = { fg = p.orn_1, fmt = "bold" },
	},

	state = {
		add = { fg = p.tyr_1 },
		new = { fg = p.pro_1 },
		modified = { fg = p.glu_1 },
		delete = { fg = p.his_1 },
	},

	msg = {
		success = {
			norm = { fg = p.emr_3 },
			inverse = { fg = p.emr_2, fmt = "reverse" },
		},
		error = {
			norm = { fg = p.rby_3 },
			inverse = { fg = p.rby_2, fmt = "reverse" },
			under = { sp = p.rby_1, fmt = "undercurl" },
			virtual = { fg = p.rby_4 },
		},
		warn = {
			norm = { fg = p.sun_3 },
			alt = { fg = p.sun_2, fmt = "italic" },
			under = { sp = p.sun_1, fmt = "undercurl" },
			virtual = { fg = p.sun_4 },
		},
		hint = {
			norm = { fg = p.sky_3 },
			under = { sp = p.sky_1, fmt = "undercurl" },
			special = { fg = p.sky_1 },
			virtual = { fg = p.sky_4 },
		},
		info = {
			norm = { fg = p.cyn_3 },
			under = { sp = p.cyn_1, fmt = "undercurl" },
			custom = { sp = p.cyn_1, fmt = "undercurl" },
			rare = { sp = p.pnk_1, fmt = "undercurl" },
			virtual = { fg = p.cyn_4 },
		},
	},

	str = {
		special = { fg = p.grn_4 },
		norm = { fg = p.grn_3 },
		doc = { fg = p.grn_2 },
		regex = { fg = p.grn_1 },
		char = { fg = p.tyr_2 },
	},

	func = {
		def = { fg = p.orn_4, fmt = "bold" },
		norm = { fg = p.orn_4 },
		builtin = { fg = p.orn_2 },
		macro = { fg = p.orn_3, fmt = "italic" },
	},

	delim = {
		norm = { fg = p.glu_2 },
		bracket = { fg = p.glu_3 },
	},

	const = {
		norm = { fg = p.sun_3 },
		builtin = { fg = p.sun_2, fmt = "italic" },
		external = { fg = p.asn_3, fmt = "italic" },
	},

	var = {
		tag = { fg = p.blu_1 },
		builtin = { fg = p.blu_2 },
		norm = { fg = p.blu_3 },
		attr = { fg = p.blu_3, fmt = "italic" },
		param = { fg = p.blu_4 },
	},

	keyword = {
		external = { fg = p.asn_2, fmt = "italic" },
		externaldef = { fg = p.asn_2, fmt = "italic,bold" },
		link = { fg = p.tyr_2, fmt = "italic" },
		logic = { fg = p.prp_1, fmt = "italic" },
		norm = { fg = p.prp_2, fmt = "italic" },
		label = { fg = p.prp_3 },
		def = { fg = p.prp_2, fmt = "bold,italic" },
		flow = { fg = p.prp_1, fmt = "bold,italic" },
	},

	type = {
		struct = { fg = p.prp_0, fmt = "italic" },
		store = { fg = p.blu_0, fmt = "italic,bold" },
		builtin = { fg = p.blu_0, fmt = "italic" },
		norm = { fg = p.blu_1, fmt = "italic" },
		def = { fg = p.blu_2, fmt = "bold" },
	},

	num = { fg = p.pnk_2 },
	bool = { fg = p.cyn_2 },
	float = { fg = p.pnk_4 },
	operator = { fg = p.prp_4 },

	link = { fg = p.tyr_3 },
	specialchar = { fg = p.sky_0 },
	special = { fg = p.sky_2 },
	h1 = { fg = p.sky_2 },
}

-- }}}

-- (TEMP) builtin {{{
hl.builtin = {
	-- base:
	Normal = t.txt.norm,
	Cursor = t.txt.reverse,
	lCursor = t.txt.reverse,
	CursorIM = t.txt.reverse,
	TermCursor = t.txt.reverse,
	TermCursorNC = t.txt.reverse,
	MatchParen = t.active.norm,
	EndOfBuffer = t.passive.invis,
	Whitespace = t.passive.comment,

	-- recurring ui:
	Folded = t.passive.norm,
	FoldColumn = t.passive.norm,
	CursorLineFold = t.idle.passive,
	SignColumn = t.passive.comment,
	CursorLineSign = t.passive.norm,
	LineNrAbove = t.passive.comment,
	LineNr = t.idle.bold,
	CursorLineNr = t.idle.passive,
	LineNrBelow = t.passive.comment,
	ColorColumn = t.passive.bg,
	CursorColumn = t.passive.bg,
	CursorLine = t.passive.bg,

	-- temporary ui:
	Pmenu = t.idle.passive_br,
	PmenuSel = t.active.select,
	PmenuSbar = t.idle.bg,
	PmenuThumb = t.idle.solid,
	WildMenu = t.active.select,

	-- state change:
	DiffAdd = t.state.add,
	DiffNew = t.state.new,
	DiffChange = t.state.modified,
	DiffText = t.state.modified,
	DiffDelete = t.state.delete,

	-- visual:
	Visual = t.active.visual,
	VisualNOS = t.passive.disown,
	Search = t.idle.search,
	CurSearch = t.active.search,
	IncSearch = t.active.search,
	Substitute = t.active.search,

	-- linting:
	SpellBad = t.msg.error.under,
	SpellCap = t.msg.hint.under,
	SpellLocal = t.msg.info.custom,
	SpellRare = t.msg.info.rare,

	-- messaging:
	ErrorMsg = t.msg.error.norm,
	WarningMsg = t.msg.warn.norm,
	ModeMsg = t.idle.passive_br,
	MsgArea = t.passive.fg,
	MoreMsg = t.idle.norm,
	MsgSeparator = t.idle.norm,
	Question = t.idle.norm,

	-- windows 'n such:
	WinSeparator = t.idle.passive_br,
	NormalFloat = t.idle.passive_br,
	NormalNC = t.passive.norm,
	QuickFixLine = t.idle.passive_br,
	StatusLine = t.passive.norm,
	StatusLineNC = t.passive.norm,
	TabLine = t.passive.norm,
	TabLineFill = t.passive.bg,
	TabLineSel = t.idle.passive_br,
	WinBar = t.idle.passive,
	WinBarNC = t.passive.norm,

	-- uncertain assignments:
	Directory = t.h1,
	Title = t.h1,
	Conceal = t.idle.norm,
	NonText = t.passive.fg,
	SpecialKey = t.txt.inactive,
}

-- }}}

-- (TEMP) syntax {{{
hl.syntax = {
	Comment = t.passive.comment,
	Constant = t.const.norm,
	String = t.str.norm,
	Character = t.str.char,
	Number = t.num,
	Boolean = t.bool,
	Float = t.float,
	Identifier = t.var.norm,
	Function = t.func.norm,
	Statement = t.keyword.norm,
	Conditional = t.keyword.logic,
	Repeat = t.keyword.logic,
	Label = t.keyword.label,
	Operator = t.operator,
	Keyword = t.keyword.norm,
	Exception = t.keyword.label,
	PreProc = t.keyword.external,
	Include = t.keyword.link,
	Define = t.keyword.externaldef,
	PreCondit = t.keyword.external,
	Macro = t.func.macro,
	Type = t.type.norm,
	StorageClass = t.type.store,
	Structure = t.type.struct,
	Typedef = t.type.def,
	Special = t.special,
	SpecialChar = t.specialchar,
	Title = t.h1,
	Tag = t.var.tag,
	Delimiter = t.delim.norm,
	SpecialComment = t.msg.hint.special,
	Debug = t.msg.hint.special,
	Ignore = t.passive.fg,
	Error = t.msg.error.norm,
	Todo = t.msg.hint.special,

	Bold = t.txt.bold,
	Italic = t.txt.italic,
	Underlined = t.txt.underline,

	healthError = t.msg.error.inverse,
	healthSuccess = t.msg.success.inverse,
	healthWarning = t.msg.warn.alt,

	markdownHeadingRule = t.passive.comment,
}

-- }}}

-- (TEMP) treesitter {{{
hl.treesitter = { -- https://github.com/nvim-treesitter/nvim-treesitter/blob/master/CONTRIBUTING.md

	-- Misc
	["@comment"] = t.passive.comment, -- line and block comments
	["@comment.documentation"] = t.passive.bg, -- comments documenting code
	["@error"] = t.msg.error.norm, -- syntax/parser errors
	["@none"] = t.txt.norm, -- completely disable the highlight
	["@preproc"] = t.keyword.external, -- various preprocessor directives & shebangs
	["@define"] = t.keyword.external, -- preprocessor definition directives
	["@operator"] = t.operator, -- symbolic operators (e.g. `+` / `*`)

	-- Punctuation
	["@punctuation.delimiter"] = t.delim.norm, --` / `.` / `,`)
	["@punctuation.bracket"] = t.delim.bracket, -- brackets (e.g. `()` / `{}` / `[]`)
	["@punctuation.special"] = t.special, -- special symbols (e.g. `{}` in string interpolation)

	-- Literals
	["@string"] = t.str.norm, -- string literals
	["@string.documentation"] = t.str.doc, -- string documenting code (e.g. Python docstrings)
	["@string.regex"] = t.str.regex, -- regular expressions
	["@string.escape"] = t.str.char, -- escape sequences
	["@string.special"] = t.str.special, -- other special strings (e.g. dates)
	["@character"] = t.str.char, -- character literals
	["@character.special"] = t.str.special, -- special characters (e.g. wildcards)
	["@boolean"] = t.bool, -- boolean literals
	["@number"] = t.num, -- numeric literals
	["@float"] = t.float, -- floating-point number literals

	-- Functions
	["@function"] = t.func.norm, -- function definitions
	["@function.builtin"] = t.func.builtin, -- built-in functions
	["@function.call"] = t.func.norm, -- function calls
	["@function.macro"] = t.func.macro, -- preprocessor macros
	["@method"] = t.func.norm, -- method definitions
	["@method.call"] = t.func.norm, -- method calls
	["@constructor"] = t.typedef, -- constructor calls and definitions
	["@parameter"] = t.var.param, -- parameters of a function

	-- Keywords
	["@keyword"] = t.keyword.norm, -- various keywords
	["@keyword.coroutine"] = t.keyword.def, -- keywords related to coroutines (e.g. `go` in Go, `async/await` in Python)
	["@keyword.function"] = t.keyword.def, -- keywords that define a function (e.g. `func` in Go, `def` in Python)
	["@keyword.operator"] = t.keyword.logic, -- operators that are English words (e.g. `and` / `or`)
	["@keyword.return"] = t.keyword.flow, -- keywords like `return` and `yield`
	["@conditional"] = t.keyword.logic, -- keywords related to conditionals (e.g. `if` / `else`)
	["@conditional.ternary"] = t.keyword.flow, -- ternary operator (e.g. `?` / `:`)
	["@repeat"] = t.keyword.logic, -- keywords related to loops (e.g. `for` / `while`)
	["@debug"] = t.msg.hint.special, -- keywords related to debugging
	["@label"] = t.keyword.label, -- GOTO and other labels (e.g. `label:` in C)
	["@include"] = t.keyword.link, -- keywords for including modules (e.g. `import` / `from` in Python)
	["@exception"] = t.keyword.label, -- keywords related to exceptions (e.g. `throw` / `catch`)

	-- Types
	["@type"] = t.type.norm, -- type or class definitions and annotations
	["@type.builtin"] = t.type.builtin, -- built-in types
	["@type.definition"] = t.type.def, -- type definitions (e.g. `typedef` in C)
	["@type.qualifier"] = t.type.struct, -- type qualifiers (e.g. `const`)
	["@storageclass"] = t.type.store, -- modifiers that affect storage in memory or life-time
	["@attribute"] = t.keyword.external, -- attribute annotations (e.g. Python decorators)
	["@field"] = t.var.norm, -- object and struct fields
	["@property"] = t.var.norm, -- similar to `@field`

	-- Identifiers
	["@variable"] = t.var.norm, -- various variable names
	["@variable.builtin"] = t.var.builtin, -- built-in variable names (e.g. `this`)
	["@constant"] = t.const.norm, -- constant identifiers
	["@constant.builtin"] = t.const.builtin, -- built-in constant values
	["@constant.macro"] = t.const.external, -- constants defined by the preprocessor
	["@namespace"] = t.var.builtin, -- modules or namespaces
	["@symbol"] = t.var.builtin, -- symbols or atoms

	-- Text
	["@text"] = t.txt.norm, -- non-structured text
	["@text.strong"] = t.txt.bold, -- bold text
	["@text.emphasis"] = t.txt.italic, -- text with emphasis
	["@text.underline"] = t.txt.underline, -- underlined text
	["@text.strike"] = t.txt.strike, -- strikethrough text
	["@text.title"] = t.txt.title, -- text that is part of a title
	["@text.literal"] = t.str.norm, -- literal or verbatim text (e.g., inline code)
	["@text.quote"] = t.txt.minor, -- text quotations
	["@text.uri"] = t.link, -- URIs (e.g. hyperlinks)
	["@text.math"] = t.msg.hint.special, -- math environments (e.g. `$ ... $` in LaTeX)
	["@text.environment"] = t.type.norm, -- text environments of markup languages
	["@text.environment.name"] = t.type.def, -- text indicating the type of an environment
	["@text.reference"] = t.passive.comment, -- text references, footnotes, citations, etc.
	["@text.todo"] = t.msg.hint.special, -- todo notes
	["@text.note"] = t.msg.info.norm, -- info notes
	["@text.warning"] = t.warning, -- warning notes
	["@text.danger"] = t.msg.error.norm, -- danger/error notes
	["@text.diff.add"] = t.state.add, -- added text (for diff files)
	["@text.diff.delete"] = t.state.delete, -- deleted text (for diff files)

	-- Tags
	["@tag"] = t.var.tag, -- XML tag names
	["@tag.attribute"] = t.var.attr, -- XML tag attributes
	["@tag.delimiter"] = t.delim.norm, -- XML tag delimiters
}
-- }}}

-- (TEMP) treesitter queries {{{
hl.treesitter_queries = { -- https://github.com/nvim-treesitter/nvim-treesitter/tree/master/queries

	--help
	["@label.help"] = t.var.tag,
	["@text.reference"] = t.link,
	["@text.literal"] = t.passive.fg,

	-- toml
	["@type.toml"] = t.var.tag,
	["@property.toml"] = t.var.param,
}
-- }}}

-- (TEMP) plugins {{{

hl.plugins.lsp = {
	DiagnosticError = t.msg.error.norm,
	DiagnosticHint = t.msg.hint.norm,
	DiagnosticInfo = t.msg.info.norm,
	DiagnosticWarn = t.msg.warn.norm,

	DiagnosticVirtualTextError = t.msg.error.virtual,
	DiagnosticVirtualTextWarn = t.msg.warn.virtual,
	DiagnosticVirtualTextInfo = t.msg.info.virtual,
	DiagnosticVirtualTextHint = t.msg.hint.virtual,

	DiagnosticUnderlineError = t.msg.error.under,
	DiagnosticUnderlineHint = t.msg.hint.under,
	DiagnosticUnderlineInfo = t.msg.info.under,
	DiagnosticUnderlineWarn = t.msg.warn.under,

	LspReferenceText = t.txt.bold,
	LspReferenceWrite = t.active.search,
	LspReferenceRead = t.idle.ref,
}

hl.plugins.gitsigns = {
	GitSignsAdd = t.state.add,
	GitSignsAddLn = t.state.add,
	GitSignsAddNr = t.state.add,
	GitSignsChange = t.state.modified,
	GitSignsChangeLn = t.state.modified,
	GitSignsChangeNr = t.state.modified,
	GitSignsDelete = t.state.delete,
	GitSignsDeleteLn = t.state.delete,
	GitSignsDeleteNr = t.state.delete,
}

hl.plugins.nvim_tree = {
	NvimTreeNormal = t.idle.passive_br,
	NvimTreeEndOfBuffer = t.idle.invis_br,
	NvimTreeRootFolder = t.txt.title,
	NvimTreeGitDirty = t.state.modified,
	NvimTreeGitNew = t.state.new,
	NvimTreeGitDeleted = t.state.new,
	NvimTreeSpecialFile = t.special,
	NvimTreeFolderName = t.var.tag,
}

hl.plugins.telescope = {
	TelescopeNormal = t.idle.passive_br,
	TelescopeBorder = t.idle.passive_br,
	TelescopeTitle = t.idle.bold,
	TelescopeMatching = t.active.search,
	TelescopePromptPrefix = t.active.norm,
	TelescopeSelection = t.active.select,
	TelescopeSelectionCaret = t.active.select,
}

-- }}}

-- (TEMP) lsp_kind_icons {{{

hl.lsp_kind_icons = {
	Default = {},
	Array = {},
	Boolean = t.bool,
	Class = {},
	Color = {},
	Constant = {},
	Constructor = {},
	Enum = {},
	EnumMember = {},
	Event = {},
	Field = {},
	File = {},
	Folder = {},
	Function = {},
	Interface = {},
	Key = {},
	Keyword = {},
	Method = {},
	Module = {},
	Namespace = {},
	Null = {},
	Number = {},
	Object = {},
	Operator = {},
	Package = {},
	Property = {},
	Reference = {},
	Snippet = {},
	String = {},
	Struct = {},
	Text = {},
	TypeParameter = {},
	Unit = {},
	Value = {},
	Variable = {},
}

-- }}}

vim_highlights(hl.builtin)
vim_highlights(hl.syntax)
vim_highlights(hl.treesitter)
vim_highlights(hl.treesitter_queries)

for _, group in pairs(hl.plugins) do
	vim_highlights(group)
end

-- }}}

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

local colors = require("tokyonight.colors").setup({})
local utils = require("tokyonight.util")
local fg_gutter_alt = "#2a2f46"

local tokyonight = {
	normal = {
		a = { bg = colors.blue, fg = colors.black },
		b = { bg = fg_gutter_alt, fg = colors.blue },
		c = { bg = "#1e2133", fg = colors.blue7 },
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
		c = { bg = "#22536", fg = fg_gutter_alt },
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
		lualine_a = {},
		lualine_b = {},
		lualine_c = {},
		lualine_x = {},
		lualine_y = {},
		lualine_z = {},
	},
	tabline = {},
	extensions = {
		"nvim-tree",
		"quickfix",
		"toggleterm",
		tsplayground,
	},
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
-- }}}
