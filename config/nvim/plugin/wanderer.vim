" Overrides {{{
let s:overrides = get(g:, "wanderer_color_overrides", {})

let s:colors = {
\ "foreground": get(s:overrides, "foreground", { "gui": "#d3d5df" }),
\ "background": get(s:overrides, "background", { "gui": "#1b1d27" }),
\
\ "black_D":    get(s:overrides, "black_D",    { "gui": "#090b18" }),
\ "black":      get(s:overrides, "black",      { "gui": "#1b1d27" }),
\
\ "grey_D":     get(s:overrides, "grey_D",     { "gui": "#1e2132" }),
\ "grey_d":     get(s:overrides, "grey_d",     { "gui": "#282b3c" }),
\ "grey":       get(s:overrides, "grey",       { "gui": "#3D3F52" }),
\ "grey_l":     get(s:overrides, "grey_l",     { "gui": "#454B68" }),
\ "grey_L":     get(s:overrides, "grey_L",     { "gui": "#6c749d" }),
\
\ "white":      get(s:overrides, "white",      { "gui": "#d3d5df" }),
\ "white_L":    get(s:overrides, "white_L",    { "gui": "#e0e3f0" }),
\
\ "red_D":      get(s:overrides, "red_D",      { "gui": "#c5595b" }),
\ "red":        get(s:overrides, "red",        { "gui": "#e67979" }),
\ "red_L":      get(s:overrides, "red_L",      { "gui": "#ff9695" }),
\
\ "orange_D":   get(s:overrides, "orange_D",   { "gui": "#c58659" }),
\ "orange":     get(s:overrides, "orange",     { "gui": "#e6a579" }),
\ "orange_L":   get(s:overrides, "orange_L",   { "gui": "#ffc295" }),
\
\ "yellow_D":   get(s:overrides, "yellow_D",   { "gui": "#c4a84a" }),
\ "yellow":     get(s:overrides, "yellow",     { "gui": "#d3bf6f" }),
\ "yellow_L":   get(s:overrides, "yellow_L",   { "gui": "#eee0a5" }),
\
\ "green_D":    get(s:overrides, "green_D",    { "gui": "#789763" }),
\ "green":      get(s:overrides, "green",      { "gui": "#94b47e" }),
\ "green_L":    get(s:overrides, "green_L",    { "gui": "#b1d29a" }),
\
\ "cyan_D":     get(s:overrides, "cyan_D",     { "gui": "#639894" }),
\ "cyan":       get(s:overrides, "cyan",       { "gui": "#7fb5b1" }),
\ "cyan_L":     get(s:overrides, "cyan_L",     { "gui": "#9cd3ce" }),
\
\ "azure_D":    get(s:overrides, "azure_D",    { "gui": "#5ba3c8" }),
\ "azure":      get(s:overrides, "azure",      { "gui": "#79c0e6" }),
\ "azure_L":    get(s:overrides, "azure_L",    { "gui": "#b4e0f8" }),
\
\ "blue_D":     get(s:overrides, "blue_D",     { "gui": "#5a86b1" }),
\ "blue":       get(s:overrides, "blue",       { "gui": "#77a2cf" }),
\ "blue_L":     get(s:overrides, "blue_L",     { "gui": "#94bfed" }),
\
\ "purple_D":   get(s:overrides, "purple_D",   { "gui": "#8c73b3" }),
\ "purple":     get(s:overrides, "purple",     { "gui": "#a98fd1" }),
\ "purple_L":   get(s:overrides, "purple_L",   { "gui": "#c7abef" }),
\
\ "pink_D":     get(s:overrides, "pink_D",     { "gui": "#c44f80" }),
\ "pink":       get(s:overrides, "pink",       { "gui": "#e46c9c" }),
\ "pink_L":     get(s:overrides, "pink_L",     { "gui": "#ff89b9" }),
\}

function! wanderer#GetColors()
  return s:colors
endfunction

" }}}

" Initialization {{{

highlight clear

if exists("syntax_on")
  syntax reset
endif

let g:colors_name="wanderer"

" Not all terminals support italics properly. If yours does, opt-in.
if !exists("g:wanderer_terminal_italics")
  let g:wanderer_terminal_italics = 1
endif

" This function is based on one from FlatColor: https://github.com/MaxSt/FlatColor/
" Which in turn was based on one found in hemisu: https://github.com/noahfrederick/vim-hemisu/
let s:group_colors = {} " Cache of default highlight group settings, for later reference via `wanderer#extend_highlight`
function! s:h(group, style, ...)
  if (a:0 > 0)
    let s:highlight = s:group_colors[a:group]
    for style_type in ["fg", "bg", "sp"]
      if (has_key(a:style, style_type))
        let l:default_style = (has_key(s:highlight, style_type) ? copy(s:highlight[style_type]) : { "cterm16": "NONE", "cterm": "NONE", "gui": "NONE" })
        let s:highlight[style_type] = extend(l:default_style, a:style[style_type])
      endif
    endfor
    if (has_key(a:style, "gui"))
      let s:highlight.gui = a:style.gui
    endif
  else
    let s:highlight = a:style
    let s:group_colors[a:group] = s:highlight " Cache default highlight group settings
  endif

  if g:wanderer_terminal_italics == 0
    if has_key(s:highlight, "cterm") && s:highlight["cterm"] == "italic"
      unlet s:highlight.cterm
    endif
    if has_key(s:highlight, "gui") && s:highlight["gui"] == "italic"
      unlet s:highlight.gui
    endif
  endif

  execute "highlight" a:group
    \ "guifg="   (has_key(s:highlight, "fg")    ? s:highlight.fg.gui   : "NONE")
    \ "guibg="   (has_key(s:highlight, "bg")    ? s:highlight.bg.gui   : "NONE")
    \ "guisp="   (has_key(s:highlight, "sp")    ? s:highlight.sp.gui   : "NONE")
    \ "gui="     (has_key(s:highlight, "gui")   ? s:highlight.gui      : "NONE")
endfunction


function! wanderer#set_highlight(group, style)
  call s:h(a:group, a:style)
endfunction

function! wanderer#extend_highlight(group, style)
  call s:h(a:group, a:style, 1)
endfunction

" }}}

" Color variables {{{

let s:colors = wanderer#GetColors()

let s:foreground = s:colors.foreground
let s:background = s:colors.background
"
let s:black_D    = s:colors.black_D
let s:black      = s:colors.black
"
let s:grey_D     = s:colors.grey_D
let s:grey_d     = s:colors.grey_d
let s:grey       = s:colors.grey
let s:grey_l     = s:colors.grey_l
let s:grey_L     = s:colors.grey_L

"
let s:white      = s:colors.white
let s:white_L    = s:colors.white_L
"
let s:red_D      = s:colors.red_D
let s:red        = s:colors.red
let s:red_L      = s:colors.red_L
"
let s:orange_D   = s:colors.orange_D
let s:orange     = s:colors.orange
let s:orange_L   = s:colors.orange_L
"
let s:yellow_D   = s:colors.yellow_D
let s:yellow     = s:colors.yellow
let s:yellow_L   = s:colors.yellow_L
"
let s:green_D    = s:colors.green_D
let s:green      = s:colors.green
let s:green_L    = s:colors.green_L
"
let s:cyan_D     = s:colors.cyan_D
let s:cyan       = s:colors.cyan
let s:cyan_L     = s:colors.cyan_L
"
let s:azure_D    = s:colors.azure_D
let s:azure      = s:colors.azure
let s:azure_L    = s:colors.azure_L
"

let s:blue_D     = s:colors.blue_D
let s:blue       = s:colors.blue
let s:blue_L     = s:colors.blue_L
"
let s:purple_D   = s:colors.purple_D
let s:purple     = s:colors.purple
let s:purple_L   = s:colors.purple_L
"
let s:pink_D     = s:colors.pink_D
let s:pink       = s:colors.pink
let s:pink_L     = s:colors.pink_L

" }}}

" Syntax Groups (descriptions and ordering from `:h w18`) {{{

call s:h("Comment",        { "fg": s:grey_l, "gui": "italic", }) " any comment
call s:h("Constant",       { "fg": s:purple_D,  "gui": "bold", }) " any constant
call s:h("String",         { "fg": s:green }) " a string constant: "this is a string"
call s:h("Character",      { "fg": s:green_L, }) " a character constant: 'c', '\n'
call s:h("Number",         { "fg": s:pink }) " a number constant: 234, 0xff
call s:h("Boolean",        { "fg": s:pink_D, "gui": "italic", }) " a boolean constant: TRUE, false
call s:h("Float",          { "fg": s:pink_L }) " a floating point constant: 2.3e10
call s:h("Identifier",     { "fg": s:purple }) " any variable name
call s:h("Function",       { "fg": s:orange }) " function name (also: methods for classes)
call s:h("Statement",      { "fg": s:blue }) " any statement
call s:h("Conditional",    { "fg": s:cyan }) " if, then, else, endif, switch, etc.
call s:h("Repeat",         { "fg": s:cyan, "gui": "italic", }) " for, do, while, etc.
call s:h("Label",          { "fg": s:blue_L }) " case, default, etc.
call s:h("Operator",       { "fg": s:cyan_L, "gui": "bold", }) " sizeof", "+", "*", etc.
call s:h("Keyword",        { "fg": s:blue_D, "gui": "bold", }) " any other keyword
call s:h("Exception",      { "fg": s:cyan_D, "gui": "italic", }) " try, catch, throw
call s:h("PreProc",        { "fg": s:white_L, "gui": "italic", }) " generic Preprocessor
call s:h("Include",        { "fg": s:blue, "gui": "italic", }) " preprocessor #include
call s:h("Define",         { "fg": s:purple, "gui": "italic", }) " preprocessor #define
call s:h("Macro",          { "fg": s:purple, "gui": "italic", }) " same as Define
call s:h("PreCondit",      { "fg": s:white_L, "gui": "italic", }) " preprocessor #if, #else, #endif, etc.
call s:h("Type",           { "fg": s:azure }) " int, long, char, etc.
call s:h("StorageClass",   { "fg": s:azure }) " static, register, volatile, etc.
call s:h("Structure",      { "fg": s:azure_D, "gui": "italic", }) " struct, union, enum, etc.
call s:h("Typedef",        { "fg": s:azure_L }) " A typedef
call s:h("Special",        { "fg": s:yellow }) " any special symbol
call s:h("SpecialChar",    { "fg": s:green_L, "gui": "bold", }) " special character in a constant
call s:h("Tag",            { "fg": s:white_L, "gui": "italic", }) " you can use CTRL-] on this
call s:h("Delimiter",      { }) " character that needs attention
call s:h("SpecialComment", { "fg": s:grey_L }) " special things inside a comment
call s:h("Debug",          { "gui": "italic",}) " debugging statements
call s:h("Underlined",     { "gui": "underline", }) " text that stands out, HTML links
call s:h("Ignore",         { }) " left blank, hidden
call s:h("Error",          { "fg": s:red }) " any erroneous construct
call s:h("Todo",           { "fg": s:yellow_L, "gui": "italic", }) " anything that needs extra attention; mostly the keywords TODO FIXME and XXX HACK BUG NOTE Note

" }}}

" Highlighting Groups (descriptions and ordering from `:h highlight-groups`) {{{

call s:h("ColorColumn",      { "bg": s:grey_D }) " used for the columns set with 'colorcolumn'
call s:h("Conceal",          { "bg": s:grey_D }) " placeholder characters substituted for concealed text (see 'conceallevel')
call s:h("Cursor",           { "fg": s:black, "bg": s:blue }) " the character under the cursor
call s:h("CursorIM",         { }) " like Cursor, but used when in IME mode
call s:h("CursorColumn",     { "bg": s:grey_D }) " the screen column that the cursor is in when 'cursorcolumn' is set
if &diff
  " Don't change the background color in diff mode
  call s:h("CursorLine",     { "gui": "underline" }) " the screen line that the cursor is in when 'cursorline' is set
else
  call s:h("CursorLine",     { "bg": s:grey_D }) " the screen line that the cursor is in when 'cursorline' is set
endif
call s:h("Directory",        { "fg": s:blue }) " directory names (and other special names in listings)
call s:h("DiffAdd",          { "fg": s:green, }) " diff mode: Added line
call s:h("DiffChange",       { "fg": s:cyan_L,  }) " diff mode: Changed line
call s:h("DiffDelete",       { "fg": s:red, }) " diff mode: Deleted line
call s:h("DiffText",         { "fg": s:orange_L, }) " diff mode: Changed text within a changed line
if get(g:, 'wanderer_hide_endofbuffer', 1)
    " If enabled, will style end-of-buffer filler lines (~) to appear to be hidden.
    call s:h("EndOfBuffer",  { "fg": s:black }) " filler lines (~) after the last line in the buffer
endif
call s:h("ErrorMsg",         { "fg": s:red }) " error messages on the command line
call s:h("VertSplit",        { "fg": s:grey_L }) " the column separating vertically split windows
call s:h("Folded",           { "fg": s:azure_L, "gui": "italic", }) " line used for closed folds
call s:h("FoldColumn",       { }) " 'foldcolumn'
call s:h("SignColumn",       { "bg": s:grey_D }) " column where signs are displayed
call s:h("IncSearch",        { "fg": s:orange_L, "gui": "bold", }) " 'incsearch' highlighting; also used for the text replaced with ":s///c"
call s:h("LineNr",           { "fg": s:grey, "bg": s:grey_D }) " Line number for ":number" and ":#" commands, and when 'number' or 'relativenumber' option is set.
call s:h("CursorLineNr",     { "fg": s:orange, "bg": s:grey_D, "gui": "bold"}) " Like LineNr when 'cursorline' or 'relativenumber' is set for the cursor line.
call s:h("MatchParen",       { "fg": s:azure_L, "gui": "bold", }) " The character under the cursor or just before it, if it is a paired bracket, and its match.
call s:h("MsgArea",          { "fg": s:grey_L, })
call s:h("ModeMsg",          { }) " 'showmode' message (e.g., "-- INSERT --")
call s:h("MoreMsg",          { "fg": s:red, "bg": s:background,}) " more-prompt
call s:h("NonText",          { "fg": s:grey_L }) " '~' and '@' at the end of the window, characters from 'showbreak' and other characters that do not really exist in the text (e.g., ">" displayed when a double-wide character doesn't fit at the end of the line).
call s:h("Normal",           { "fg": s:foreground, "bg": s:background }) " normal text
call s:h("Pmenu",            { "fg": s:grey_L, "bg": s:grey_D }) " Popup menu: normal item.
call s:h("PmenuSel",         { "fg": s:orange, "bg": s:grey_D }) " Popup menu: selected item.
call s:h("PmenuSbar",        { "bg": s:grey_D }) " Popup menu: scrollbar.
call s:h("PmenuThumb",       { "bg": s:grey_d }) " Popup menu: Thumb of the scrollbar.
call s:h("Question",         { "fg": s:blue, "gui": "bold", }) " hit-enter prompt and yes/no questions
call s:h("QuickFixLine",     { "fg": s:black, "bg": s:yellow }) " Current quickfix item in the quickfix window.
call s:h("Search",           { "fg": s:orange_L }) " Last search pattern highlighting (see 'hlsearch'). Also used for similar items that need to stand out.
call s:h("SpecialKey",       { "fg": s:grey_L }) " Meta and special keys listed with ":map", also for text used to show unprintable characters in the text, 'listchars'. Generally: text that is displayed differently from what it really is.
call s:h("SpellBad",         { "fg": s:red, "gui": "underline", }) " Word that is not recognized by the spellchecker. This will be combined with the highlighting used otherwise.
call s:h("SpellCap",         { "fg": s:pink_D }) " Word that should start with a capital. This will be combined with the highlighting used otherwise.
call s:h("SpellLocal",       { "fg": s:pink_D }) " Word that is recognized by the spellchecker as one that is used in another region. This will be combined with the highlighting used otherwise.
call s:h("SpellRare",        { "fg": s:pink_D }) " Word that is recognized by the spellchecker as one that is hardly ever used. spell This will be combined with the highlighting used otherwise.
call s:h("StatusLine",       { "fg": s:white, "bg": s:grey_D }) " status line of current window
call s:h("StatusLineNC",     { "fg": s:grey_l }) " status lines of not-current windows Note: if this is equal to "StatusLine" Vim will use "^^^" in the status line of the current window.
call s:h("StatusLineTerm",   { "fg": s:white, "bg": s:grey_D }) " status line of current :terminal window
call s:h("StatusLineTermNC", { "fg": s:grey_l }) " status line of non-current :terminal window
call s:h("TabLine",          { }) " tab pages line, not active tab page label
call s:h("TabLineFill",      { }) " tab pages line, where there are no labels
call s:h("TabLineSel",       { "bg": s:red,}) " tab pages line, active tab page label
call s:h("Terminal",         { "fg": s:white, "bg": s:black }) " terminal window (see terminal-size-color)
call s:h("Title",            { "fg": s:azure }) " titles for output from ":set all", ":autocmd" etc.
call s:h("Visual",           { "bg": s:grey_d }) " Visual mode selection
call s:h("VisualNOS",        { "bg": s:grey_d }) " Visual mode selection when vim is "Not Owning the Selection". Only X11 Gui's gui-x11 and xterm-clipboard supports this.
call s:h("WarningMsg",       { "fg": s:red_D }) " warning messages
call s:h("WildMenu",         { "fg": s:black, "bg": s:blue }) " current match in 'wildmenu' completion

" }}}

" Language-Specific Highlighting {{{

" " CSS
" call s:h("cssAttrComma",              { "fg": s:purple })
" call s:h("cssAttributeSelector",      { "fg": s:green })
" call s:h("cssBraces",                 { "fg": s:white })
" call s:h("cssClassName",              { "fg": s:yellow_D })
" call s:h("cssClassNameDot",           { "fg": s:yellow_D })
" call s:h("cssDefinition",             { "fg": s:purple })
" call s:h("cssFontAttr",               { "fg": s:yellow_D })
" call s:h("cssFontDescriptor",         { "fg": s:purple })
" call s:h("cssFunctionName",           { "fg": s:blue })
" call s:h("cssIdentifier",             { "fg": s:blue })
" call s:h("cssImportant",              { "fg": s:purple })
" call s:h("cssInclude",                { "fg": s:white })
" call s:h("cssIncludeKeyword",         { "fg": s:purple })
" call s:h("cssMediaType",              { "fg": s:yellow_D })
" call s:h("cssProp",                   { "fg": s:white })
" call s:h("cssPseudoClassId",          { "fg": s:yellow_D })
" call s:h("cssSelectorOp",             { "fg": s:purple })
" call s:h("cssSelectorOp2",            { "fg": s:purple })
" call s:h("cssTagName",                { "fg": s:red })

" " Go
" call s:h("goDeclaration",             { "fg": s:purple })
" call s:h("goBuiltins",                { "fg": s:cyan })
" call s:h("goFunctionCall",            { "fg": s:blue })
" call s:h("goVarDefs",                 { "fg": s:red })
" call s:h("goVarAssign",               { "fg": s:red })
" call s:h("goVar",                     { "fg": s:purple })
" call s:h("goConst",                   { "fg": s:purple })
" call s:h("goType",                    { "fg": s:yellow })
" call s:h("goTypeName",                { "fg": s:yellow })
" call s:h("goDeclType",                { "fg": s:cyan })
" call s:h("goTypeDecl",                { "fg": s:purple })

" " Python

" " Solditiy

" ╔═╗┬ ┬┌─┐┬  ┬
" ╚═╗├─┤├┤ │  │
" ╚═╝┴ ┴└─┘┴─┘┴─┘
call s:h("bashTSNone", { "fg": s:purple })
call s:h("bashTSParameter", { "fg": s:white })
call s:h("shDerefSimple", { "fg": s:purple })

" " HTML (keep consistent with Markdown, below)
" call s:h("htmlArg",                   { "fg": s:yellow_D })
" call s:h("htmlBold",                  { "fg": s:yellow_D, "gui": "bold", "cterm": "bold" })
" call s:h("htmlBoldItalic",            { "fg": s:green, "gui": "bold,italic", "cterm": "bold,italic" })
" call s:h("htmlEndTag",                { "fg": s:white })
" call s:h("htmlH1",                    { "fg": s:red })
" call s:h("htmlH2",                    { "fg": s:red })
" call s:h("htmlH3",                    { "fg": s:red })
" call s:h("htmlH4",                    { "fg": s:red })
" call s:h("htmlH5",                    { "fg": s:red })
" call s:h("htmlH6",                    { "fg": s:red })
" call s:h("htmlItalic",                { "fg": s:purple, "gui": "italic", "cterm": "italic" })
" call s:h("htmlLink",                  { "fg": s:cyan, "gui": "underline", "cterm": "underline" })
" call s:h("htmlSpecialChar",           { "fg": s:yellow_D })
" call s:h("htmlSpecialTagName",        { "fg": s:red })
" call s:h("htmlTag",                   { "fg": s:white })
" call s:h("htmlTagN",                  { "fg": s:red })
" call s:h("htmlTagName",               { "fg": s:red })
" call s:h("htmlTitle",                 { "fg": s:white })

" " Markdown (keep consistent with HTML, above)
" call s:h("markdownBlockquote",        { "fg": s:grey_l })
" call s:h("markdownBold",              { "fg": s:yellow_D, "gui": "bold", "cterm": "bold" })
" call s:h("markdownBoldItalic",        { "fg": s:green, "gui": "bold,italic", "cterm": "bold,italic" })
" call s:h("markdownCode",              { "fg": s:green })
" call s:h("markdownCodeBlock",         { "fg": s:green })
" call s:h("markdownCodeDelimiter",     { "fg": s:green })
" call s:h("markdownH1",                { "fg": s:red })
" call s:h("markdownH2",                { "fg": s:red })
" call s:h("markdownH3",                { "fg": s:red })
" call s:h("markdownH4",                { "fg": s:red })
" call s:h("markdownH5",                { "fg": s:red })
" call s:h("markdownH6",                { "fg": s:red })
" call s:h("markdownHeadingDelimiter",  { "fg": s:red })
" call s:h("markdownHeadingRule",       { "fg": s:grey_l })
" call s:h("markdownId",                { "fg": s:purple })
" call s:h("markdownIdDeclaration",     { "fg": s:blue })
" call s:h("markdownIdDelimiter",       { "fg": s:purple })
" call s:h("markdownItalic",            { "fg": s:purple, "gui": "italic", "cterm": "italic" })
" call s:h("markdownLinkDelimiter",     { "fg": s:purple })
" call s:h("markdownLinkText",          { "fg": s:blue })
" call s:h("markdownListMarker",        { "fg": s:red })
" call s:h("markdownOrderedListMarker", { "fg": s:red })
" call s:h("markdownRule",              { "fg": s:grey_l })
" call s:h("markdownUrl",               { "fg": s:cyan, "gui": "underline", "cterm": "underline" })

" " TypeScript
" call s:h("typescriptReserved",        { "fg": s:purple })
" call s:h("typescriptEndColons",       { "fg": s:white })
" call s:h("typescriptBraces",          { "fg": s:white })

" " JavaScript
" call s:h("javaScriptBraces",          { "fg": s:white })
" call s:h("javaScriptFunction",        { "fg": s:purple })
" call s:h("javaScriptIdentifier",      { "fg": s:purple })
" call s:h("javaScriptNull",            { "fg": s:yellow_D })
" call s:h("javaScriptNumber",          { "fg": s:yellow_D })
" call s:h("javaScriptRequire",         { "fg": s:cyan })
" call s:h("javaScriptReserved",        { "fg": s:purple })
" call s:h("jsArrowFunction",           { "fg": s:purple })
" call s:h("jsClassKeyword",            { "fg": s:purple })
" call s:h("jsClassMethodType",         { "fg": s:purple })
" call s:h("jsDocParam",                { "fg": s:blue })
" call s:h("jsDocTags",                 { "fg": s:purple })
" call s:h("jsExport",                  { "fg": s:purple })
" call s:h("jsExportDefault",           { "fg": s:purple })
" call s:h("jsExtendsKeyword",          { "fg": s:purple })
" call s:h("jsFrom",                    { "fg": s:purple })
" call s:h("jsFuncCall",                { "fg": s:blue })
" call s:h("jsFunction",                { "fg": s:purple })
" call s:h("jsGenerator",               { "fg": s:yellow })
" call s:h("jsGlobalObjects",           { "fg": s:yellow })
" call s:h("jsImport",                  { "fg": s:purple })
" call s:h("jsModuleAs",                { "fg": s:purple })
" call s:h("jsModuleWords",             { "fg": s:purple })
" call s:h("jsModules",                 { "fg": s:purple })
" call s:h("jsNull",                    { "fg": s:yellow_D })
" call s:h("jsOperator",                { "fg": s:purple })
" call s:h("jsStorageClass",            { "fg": s:purple })
" call s:h("jsSuper",                   { "fg": s:red })
" call s:h("jsTemplateBraces",          { "fg": s:red_D })
" call s:h("jsTemplateVar",             { "fg": s:green })
" call s:h("jsThis",                    { "fg": s:red })
" call s:h("jsUndefined",               { "fg": s:yellow_D })
" call s:h("javascriptArrowFunc",       { "fg": s:purple })
" call s:h("javascriptClassExtends",    { "fg": s:purple })
" call s:h("javascriptClassKeyword",    { "fg": s:purple })
" call s:h("javascriptDocNotation",     { "fg": s:purple })
" call s:h("javascriptDocParamName",    { "fg": s:blue })
" call s:h("javascriptDocTags",         { "fg": s:purple })
" call s:h("javascriptEndColons",       { "fg": s:white })
" call s:h("javascriptExport",          { "fg": s:purple })
" call s:h("javascriptFuncArg",         { "fg": s:white })
" call s:h("javascriptFuncKeyword",     { "fg": s:purple })
" call s:h("javascriptIdentifier",      { "fg": s:red })
" call s:h("javascriptImport",          { "fg": s:purple })
" call s:h("javascriptMethodName",      { "fg": s:white })
" call s:h("javascriptObjectLabel",     { "fg": s:white })
" call s:h("javascriptOpSymbol",        { "fg": s:cyan })
" call s:h("javascriptOpSymbols",       { "fg": s:cyan })
" call s:h("javascriptPropertyName",    { "fg": s:green })
" call s:h("javascriptTemplateSB",      { "fg": s:red_D })
" call s:h("javascriptVariable",        { "fg": s:purple })

" " TeX
" call s:h("texStatement",              { "fg": s:purple })
" call s:h("texSubscripts",             { "fg": s:yellow_D })
" call s:h("texSuperscripts",           { "fg": s:yellow_D })
" call s:h("texTodo",                   { "fg": s:red_D })
" call s:h("texBeginEnd",               { "fg": s:purple })
" call s:h("texBeginEndName",           { "fg": s:blue })
" call s:h("texMathMatcher",            { "fg": s:blue })
" call s:h("texMathDelim",              { "fg": s:blue })
" call s:h("texDelimiter",              { "fg": s:yellow_D })
" call s:h("texSpecialChar",            { "fg": s:yellow_D })
" call s:h("texCite",                   { "fg": s:blue })
" call s:h("texRefZone",                { "fg": s:blue })

" }}}

" Plugin Highlighting {{{
"

" airblade/vim-gitgutter
call s:h("SignifySignAdd",          { "bg": s:grey_D, "fg": s:green })
call s:h("SignifySignChange",       { "bg": s:grey_D, "fg": s:cyan })
call s:h("SignifySignDelete",       { "bg": s:grey_D, "fg": s:red })

" mhinz/vim-startify
call s:h("StartifyFile",            { "fg": s:orange, })
call s:h("StartifySection",         { "fg": s:azure, })
call s:h("StartifyBracket",         { "fg": s:grey_l, })

" vim-syntastic/syntastic
call s:h("SyntasticError",          { "fg": s:red, "bg": s:grey_D,"gui": "underline", })
call s:h("SyntasticWarning",        { "fg": s:yellow, "bg": s:grey_D,"gui": "underline",})
call s:h("SyntasticErrorSign",      { "fg": s:red, "bg": s:grey_D,"gui": "bold",})
call s:h("SyntasticWarningSign",    { "fg": s:yellow, "bg": s:grey_D,"gui": "bold",})

" easymotion/vim-easymotion
call s:h("EasyMotionTarget",        { "fg": s:orange, "gui": "bold", })
call s:h("EasyMotionTarget2First",  { "fg": s:orange_D, "gui": "bold", })
call s:h("EasyMotionTarget2Second", { "fg": s:orange_L, "gui": "bold", })
call s:h("EasyMotionShade",         { "fg": s:grey_d })

" prabirshrestha/vim-lsp
call s:h("LspError",                { "fg": s:red })
call s:h("LspWarning",              { "fg": s:yellow })
call s:h("LspInformation",          { "fg": s:blue })
call s:h("LspHint",                 { "fg": s:cyan, "gui": "italic", })

" tpope/vim-fugitive
call s:h("diffAdded",               { "fg": s:green })
call s:h("diffRemoved",             { "fg": s:red })

" nvim-telescope/telescope.nvim
call s:h("TelescopeTitle",          { "fg": s:azure_L, "gui": "bold", })
call s:h("TelescopePromptPrefix",   { "fg": s:azure_L, "gui": "bold", })
call s:h("TelescopeMatching",       { "fg": s:orange_L, "gui": "bold", })
call s:h("TelescopeSelection",      { "bg": s:grey_D, })

" }}}

" Git Highlighting {{{

call s:h("gitcommitComment",       { "fg": s:grey_l })
call s:h("gitcommitUnmerged",      { "fg": s:green })
call s:h("gitcommitOnBranch",      { })
call s:h("gitcommitBranch",        { "fg": s:cyan })
call s:h("gitcommitDiscardedType", { "fg": s:red })
call s:h("gitcommitSelectedType",  { "fg": s:green })
call s:h("gitcommitHeader",        { })
call s:h("gitcommitUntrackedFile", { "fg": s:yellow })
call s:h("gitcommitDiscardedFile", { "fg": s:red })
call s:h("gitcommitSelectedFile",  { "fg": s:azure })
call s:h("gitcommitUnmergedFile",  { "fg": s:yellow_D })
call s:h("gitcommitFile",          { "fg": s:orange })
call s:h("gitcommitSummary",       { "fg": s:white })
call s:h("gitcommitOverflow",      { "fg": s:red })
hi link gitcommitNoBranch gitcommitBranch
hi link gitcommitUntracked gitcommitComment
hi link gitcommitDiscarded gitcommitComment
hi link gitcommitSelected gitcommitComment
hi link gitcommitDiscardedArrow gitcommitDiscardedFile
hi link gitcommitSelectedArrow gitcommitSelectedFile
hi link gitcommitUnmergedArrow gitcommitUnmergedFile

" }}}

" Neovim-Specific Highlighting {{{

if has("nvim")
  call s:h("DiagnosticError", { "fg": s:red })
  call s:h("DiagnosticWarn", { "fg": s:yellow })
  call s:h("DiagnosticInfo", { "fg": s:blue })
  call s:h("DiagnosticHint", { "fg": s:cyan, "gui": "italic", })
  call s:h("DiagnosticUnderlineError", { "fg": s:red, "gui": "underline", })
  call s:h("DiagnosticUnderlineWarn", { "fg": s:yellow, "gui": "underline", })
  call s:h("DiagnosticUnderlineInfo", { "fg": s:blue, "gui": "underline", })
  call s:h("DiagnosticUnderlineHint", { "fg": s:cyan, "gui": "underline", })
endif

" }}}

set background=dark
lua require 'colorizer'.setup()

let s:_white   = '#d3d5df'
let s:_black_D = '#090b18'
let s:_black   = '#1b1d27'
let s:_grey_D  = '#1e2132'
let s:_grey_d  = '#282b3c'
let s:_grey    = '#3d3f52'
let s:_grey_l  = '#454B68'
let s:_grey_L  = '#6c749d'
let s:_red     = '#e67979'
let s:_orange  = '#e6a579'
let s:_yellow  = '#d3bf6f'
let s:_green   = '#94b47e'
let s:_cyan    = '#7fb5b1'
let s:_azure   = '#b4e0f8'
let s:_blue    = '#77a2cf'
let s:_purple  = '#a98fd1'
let s:_pink    = '#e46c9c'

let s:p = {'normal': {}, 'inactive': {}, 'insert': {}, 'replace': {}, 'visual': {}, 'tabline': {}}

let s:p.normal.left     = [ [s:_black, s:_blue, 'bold'   ], [s:_blue, s:_grey_d   ] ]
let s:p.insert.left     = [ [s:_black, s:_azure, 'bold'  ], [s:_azure, s:_grey_d  ] ]
let s:p.visual.left     = [ [s:_black, s:_green, 'bold'  ], [s:_green,  s:_grey_d ] ]
let s:p.replace.left    = [ [s:_black, s:_purple, 'bold' ], [s:_purple, s:_grey_d ] ]
let s:p.inactive.left   = [ [s:_black, s:_pink, 'bold'   ], [s:_pink, s:_grey_d   ] ]

let s:p.normal.middle   = [ [s:_white, s:_grey_D ] ]

let s:p.normal.right    = [ [s:_black, s:_blue   ], [s:_blue,  s:_grey_d  ] ]
let s:p.insert.right    = [ [s:_black, s:_azure  ], [s:_azure, s:_grey_d ] ]
let s:p.visual.right    = [ [s:_black, s:_green  ], [s:_green, s:_grey_d ] ]
let s:p.replace.right   = [ [s:_black, s:_purple ], [s:_green, s:_grey_d ] ]
let s:p.inactive.right  = [ [s:_black, s:_pink   ], [s:_pink, s:_grey_d  ] ]

let s:p.normal.error    = [ [s:_red, s:_grey_D    ] ]
let s:p.normal.warning  = [ [s:_yellow, s:_grey_D ] ]

let g:lightline#colorscheme#wanderer#palette = lightline#colorscheme#fill(s:p)

" ⚓ lightline
let g:lightline = {
\ 'separator': { 'left': '', 'right': '' },
\ 'subseparator': { 'left': '⊸', 'right': ''},
\ 'colorscheme': 'wanderer',
\ 'active': {
\   'left': [ [ 'mode', ],
\             [ 'fugitive', 'filename', 'modified',] ],
\   'right': [ [ 'filetype' ],
\              [ 'readonly'],
\              [ 'syntastic'] ]
\ },
\ 'inactive': {
\   'left': [ [ 'relativepath' ],
\             [ 'modified' ] ],
\   'right': [ [ 'filetype' ],
\              [ 'readonly'] ]
\ },
\ 'component_function': {
\   'fugitive': 'LightlineFugitive',
\   'filename': 'LightlineFilename',
\   'readonly': 'LightlineReadonly',
\   'modified': 'LightlineModified',
\ },
\ 'component_expand': {
\   'syntastic': 'SyntasticStatuslineFlag',
\   'filename': 'LightlineFilename',
\ },
\ 'component_type': {
\   'syntastic': 'error',
\ },
\ }

let g:lightline.mode_map = {
\ 'n' : '⟂',
\ 'i' : '⊢',
\ 'R' : '﯒',
\ 'v' : '∋',
\ 'V' : 'λ',
\ "\<C-v>": 'β',
\ 'c' : '',
\ }

fun! LightlineFugitive()
    try
	    if expand('%:t') !~? 'Tagbar\|Gundo\|NERD' && &ft !~? 'vimfiler' && exists('*FugitiveHead')
	        let mark = ' '   " edit here for cool mark
            let branch = FugitiveHead()
	        return branch !=# '' ? mark.branch : ''
	    endif
    catch
    endtry
    return ''
endfun

fun! LightlineFilename()
	return expand('%:t') !=# '' ? expand('%:t') : '名前?'
endfun

fun! LightlineReadonly()
	return &readonly ? '' : ''
endfun

fun! LightlineModified()
   return &modifiable && &modified ? '' : ''
endfun

fun! SyntasticCheckHook(errors)
    call lightline#update()
endfun

lua << EOF
require'nvim-web-devicons'.setup {
 default = true;
}
EOF
