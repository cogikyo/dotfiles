local M = {}

function M.style(color, add_mods)
	return {
		fg = color or nil,
		add_modifiers = add_mods or nil,
	}
end

function M.format(fmt, color, mods)
	return {
		format = fmt,
		style = {
			fg = color,
			add_modifiers = mods or {},
		},
	}
end

function M.panel_format(fmt, color, mods)
	return {
		title = {
			format = fmt,
			style = {
				fg = color,
				add_modifiers = mods or {},
			},
		},
		border_style = {
			fg = color,
			add_modifiers = mods or {},
		},
		style = { fg = color },
	}
end

return M
