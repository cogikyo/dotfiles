local h = require("lib.helpers")
local style = h.style
local format = h.format
local panel_format = h.panel_format

local general = {
	disable_debug_error_mode = false,
	enable_mouse = true,
	show_hidden = false,
	read_only = false,
	enable_recover_mode = false,
	hide_remaps_in_help_menu = false,
	enforce_bounded_index_navigation = false,
	prompt = format("  ", "Yellow"),

	logs = {
		success = format("SUCCESS", "Green"),
		warning = format("WARNING", "Yellow"),
		error = format("ERROR", "Red"),
	},

	table = {
		header = {
			cols = {
				{ format = "   󱣱 " },
				{
					format = "╭╼╾⼮path ──────────────────────────────────────────────────────────────────────────────╮",
				},
				{ format = "   " },
				{ format = "󰖡 size ─╮" },
				{
					format = "  modified ────────────╮",
				},
			},
			style = style("Yellow"),
			height = 1,
		},
		row = {
			cols = {
				{ format = "custom.fmt_col_0" },
				{ format = "custom.fmt_col_1" },
				{ format = "custom.fmt_col_2" },
				{ format = "custom.fmt_col_3" },
				{ format = "custom.fmt_col_4" },
			},
			style = {},
			height = 0,
		},
		style = {},
		tree = {
			{ format = "├", style = {} },
			{ format = "├", style = {} },
			{ format = "╰", style = {} },
		},
		col_spacing = 1,
		col_widths = {
			{ Percentage = 4 },
			{ Percentage = 57 },
			{ Percentage = 4 },
			{ Percentage = 11 },
			{ Percentage = 21 },
		},
	},

	selection = {
		item = {
			format = "custom.fmt_selection_item",
			style = {},
		},
	},

	search = {
		algorithm = "Fuzzy",
		unordered = false,
	},

	default_ui = {
		prefix = " ",
		suffix = "",
		style = {},
	},

	focus_ui = {
		prefix = "⌬ ",
		suffix = "",
		style = style("Yellow", { "Bold" }),
	},

	selection_ui = {
		prefix = "⟪─── ",
		suffix = " ─╼╾ 󱘇",
		style = style("Cyan", { "Dim" }),
	},

	focus_selection_ui = {
		prefix = "⌬ ⟪─── ",
		suffix = " ─╼╾ 󰎐",
		style = style("Yellow", { "Bold" }),
	},

	sort_and_filter_ui = {
		separator = format(" ⇨ ", "DarkGray"),
		sort_direction_identifiers = {
			forward = format("⮯", "Magenta"),
			reverse = format("⮭", "Cyan"),
		},
		sorter_identifiers = {
			ByExtension = { format = "ext", style = {} },
			ByICanonicalAbsolutePath = { format = "[ci]abs", style = {} },
			ByIRelativePath = format("⻚", "Magenta"),
			ByISymlinkAbsolutePath = { format = "[si]abs", style = {} },
			ByIsBroken = { format = "⨯", style = {} },
			ByIsDir = { format = "dir", style = {} },
			ByIsFile = { format = "file", style = {} },
			ByIsReadonly = { format = "ro", style = {} },
			ByIsSymlink = { format = "sym", style = {} },
			ByMimeEssence = { format = "mime", style = {} },
			ByRelativePath = { format = "rel", style = {} },
			BySize = { format = "size", style = {} },
			ByCreated = { format = "created", style = {} },
			ByLastModified = { format = "modified", style = {} },
			ByCanonicalAbsolutePath = { format = "[c]abs", style = {} },
			ByCanonicalExtension = { format = "[c]ext", style = {} },
			ByCanonicalIsDir = format("⽊", "Cyan"),
			ByCanonicalIsFile = { format = "[c]file", style = {} },
			ByCanonicalIsReadonly = { format = "[c]ro", style = {} },
			ByCanonicalMimeEssence = { format = "[c]mime", style = {} },
			ByCanonicalSize = { format = "[c]size", style = {} },
			ByCanonicalCreated = { format = "[c]created", style = {} },
			ByCanonicalLastModified = { format = "[c]modified", style = {} },
			BySymlinkAbsolutePath = { format = "[s]abs", style = {} },
			BySymlinkExtension = { format = "[s]ext", style = {} },
			BySymlinkIsDir = { format = "[s]dir", style = {} },
			BySymlinkIsFile = { format = "[s]file", style = {} },
			BySymlinkIsReadonly = { format = "[s]ro", style = {} },
			BySymlinkMimeEssence = { format = "[s]mime", style = {} },
			BySymlinkSize = { format = "[s]size", style = {} },
			BySymlinkCreated = { format = "[s]created", style = {} },
			BySymlinkLastModified = { format = "[s]modified", style = {} },
		},
		filter_identifiers = {
			RelativePathDoesContain = { format = "rel=~", style = {} },
			RelativePathDoesEndWith = { format = "rel=$", style = {} },
			RelativePathDoesNotContain = { format = "rel!~", style = {} },
			RelativePathDoesNotEndWith = { format = "rel!$", style = {} },
			RelativePathDoesNotStartWith = format(" 󰊠 ", "White"),
			RelativePathDoesStartWith = { format = "rel=^", style = {} },
			RelativePathIs = { format = "rel==", style = {} },
			RelativePathIsNot = { format = "rel!=", style = {} },
			RelativePathDoesMatchRegex = { format = "rel=/", style = {} },
			RelativePathDoesNotMatchRegex = { format = "rel!/", style = {} },
			IRelativePathDoesContain = { format = "[i]rel=~", style = {} },
			IRelativePathDoesEndWith = { format = "[i]rel=$", style = {} },
			IRelativePathDoesNotContain = { format = "[i]rel!~", style = {} },
			IRelativePathDoesNotEndWith = { format = "[i]rel!$", style = {} },
			IRelativePathDoesNotStartWith = { format = "[i]rel!^", style = {} },
			IRelativePathDoesStartWith = { format = "[i]rel=^", style = {} },
			IRelativePathIs = { format = "[i]rel==", style = {} },
			IRelativePathIsNot = { format = "[i]rel!=", style = {} },
			IRelativePathDoesMatchRegex = { format = "[i]rel=/", style = {} },
			IRelativePathDoesNotMatchRegex = { format = "[i]rel!/", style = {} },
			AbsolutePathDoesContain = { format = "abs=~", style = {} },
			AbsolutePathDoesEndWith = { format = "abs=$", style = {} },
			AbsolutePathDoesNotContain = { format = "abs!~", style = {} },
			AbsolutePathDoesNotEndWith = { format = "abs!$", style = {} },
			AbsolutePathDoesNotStartWith = { format = "abs!^", style = {} },
			AbsolutePathDoesStartWith = { format = "abs=^", style = {} },
			AbsolutePathIs = { format = "abs==", style = {} },
			AbsolutePathIsNot = { format = "abs!=", style = {} },
			AbsolutePathDoesMatchRegex = { format = "abs=/", style = {} },
			AbsolutePathDoesNotMatchRegex = { format = "abs!/", style = {} },
			IAbsolutePathDoesContain = { format = "[i]abs=~", style = {} },
			IAbsolutePathDoesEndWith = { format = "[i]abs=$", style = {} },
			IAbsolutePathDoesNotContain = { format = "[i]abs!~", style = {} },
			IAbsolutePathDoesNotEndWith = { format = "[i]abs!$", style = {} },
			IAbsolutePathDoesNotStartWith = { format = "[i]abs!^", style = {} },
			IAbsolutePathDoesStartWith = { format = "[i]abs=^", style = {} },
			IAbsolutePathIs = { format = "[i]abs==", style = {} },
			IAbsolutePathIsNot = { format = "[i]abs!=", style = {} },
			IAbsolutePathDoesMatchRegex = { format = "[i]abs=/", style = {} },
			IAbsolutePathDoesNotMatchRegex = { format = "[i]abs!/", style = {} },
		},
		search_identifiers = {
			Fuzzy = {
				format = " ",
				style = style("Yellow", { "Bold" }),
			},
			Regex = {
				format = " ",
				style = style("Yellow", { "Bold" }),
			},
		},
	},
	panel_ui = {
		default = {
			title = {
				format = nil,
				style = style("Blue", { "Bold" }),
			},
			style = {},
			borders = {
				"Top",
				"Right",
				"Bottom",
				"Left",
			},
			border_type = "Rounded",
			border_style = { fg = "DarkGray" },
		},
		table = panel_format(
			-- "──────────────────────────╼╾ ⽊ Directory ╼╾",
			nil,
			"Blue",
			{ "Bold" }
		),
		help_menu = panel_format("─╼╾ 何 Help ╼╾", "Magenta", { "Dim" }),
		input_and_logs = panel_format(
			"──────────────────────────────────────────────────────────────╼╾  Input ╼╾  Logs ╼╾",
			"Blue",
			{ "Dim" }
		),
		selection = panel_format("─╼╾ ⼶ Selection ╼╾", "Cyan", { "Dim" }),
		sort_and_filter = panel_format(
			"──────────────────────────────────────────────────────────────╼╾  Filter ╼╾ Sort  ╼╾",
			"Blue",
			{ "Dim" }
		),
	},

	initial_sorting = {
		{ sorter = "ByCanonicalIsDir", reverse = true },
		{ sorter = "ByIRelativePath", reverse = false },
	},

	initial_mode = "preview_mode",
	initial_layout = "preview",
	start_fifo = nil,

	global_key_bindings = {
		on_key = {
			["esc"] = {
				messages = {
					"PopMode",
				},
			},
			["ctrl-c"] = {
				messages = {
					"Terminate",
				},
			},
		},
	},
}

for key, val in pairs(general) do
	xplr.config.general[key] = val
end

-- Custom formatters: style entire row based on selection/focus state
local selection_style = { fg = "Cyan", add_modifiers = { "Dim" } }
local focus_selection_style = { fg = "Cyan", add_modifiers = { "Bold" } }

local function strip_ansi(str)
	return str:gsub("\027%[[%d;]*m", "")
end

local function wrap_with_style(ctx, content)
	if ctx.is_selected and ctx.is_focused then
		return xplr.util.paint(strip_ansi(content), focus_selection_style)
	elseif ctx.is_selected then
		return xplr.util.paint(strip_ansi(content), selection_style)
	end
	return content
end

xplr.fn.custom.fmt_col_0 = function(ctx)
	local content = xplr.fn.builtin.fmt_general_table_row_cols_0(ctx)
	return wrap_with_style(ctx, content)
end

xplr.fn.custom.fmt_col_1 = function(ctx)
	local content = xplr.fn.builtin.fmt_general_table_row_cols_1(ctx)

	local clean = strip_ansi(content)
	local s, e = clean:find("-> ", 1, true)
	if s then
		local before = clean:sub(1, s - 1)
		local after = clean:sub(e + 1)
		local link_style = { fg = "Blue", add_modifiers = { "Italic", "Dim" } }
		content = xplr.util.paint(before, link_style)
			.. xplr.util.paint("-> ", link_style)
			.. xplr.util.paint(after, link_style)
	end

	return wrap_with_style(ctx, content)
end

xplr.fn.custom.fmt_col_2 = function(ctx)
	local content = xplr.fn.builtin.fmt_general_table_row_cols_2(ctx)
	return wrap_with_style(ctx, content)
end

xplr.fn.custom.fmt_col_3 = function(ctx)
	local content = xplr.fn.builtin.fmt_general_table_row_cols_3(ctx)
	return wrap_with_style(ctx, content)
end

xplr.fn.custom.fmt_col_4 = function(ctx)
	local content = xplr.fn.builtin.fmt_general_table_row_cols_4(ctx)
	return wrap_with_style(ctx, content)
end

xplr.fn.custom.fmt_selection_item = function(ctx)
	local content = xplr.fn.builtin.fmt_general_selection_item(ctx)
	local clean = strip_ansi(content)
	return xplr.util.paint(clean, { fg = "Cyan", add_modifiers = { "Bold" } })
end
