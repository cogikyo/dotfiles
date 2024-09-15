-- 🧰 setup {{{
local cusor_ok, cursor = pcall(require, "smoothcursor")
if not cusor_ok then
	vim.api.nvim_echo({
		{
			"Error: cursor plugin not found... skipping relevant setup()",
			"Error",
		},
	}, true, {})
	return
end

local additional_colors_ok, p = pcall(require, "vagari.palette")
if not additional_colors_ok then
	vim.api.nvim_echo({
		{
			"Error: cursor config has impored vagari.nvim that are not found, skipping setup",
			"Error",
		},
	}, true, {})
	return
end
-- }}}

local autocmd = vim.api.nvim_create_autocmd

autocmd({ "ModeChanged" }, {
	callback = function()
		local current_mode = vim.fn.mode()
		if current_mode == "n" then
			vim.api.nvim_set_hl(0, "SmoothCursor", { fg = p.blu_3 })
		elseif current_mode == "c" then
			vim.api.nvim_set_hl(0, "SmoothCursor", { fg = p.orn_3 })
		elseif current_mode == "v" then
			vim.api.nvim_set_hl(0, "SmoothCursor", { fg = p.prp_3 })
		elseif current_mode == "V" then
			vim.api.nvim_set_hl(0, "SmoothCursor", { fg = p.prp_3 })
		elseif current_mode == "�" then
			vim.api.nvim_set_hl(0, "SmoothCursor", { fg = p.prp_3 })
		elseif current_mode == "i" then
			vim.api.nvim_set_hl(0, "SmoothCursor", { fg = p.grn_3 })
		elseif current_mode == "R" then
			vim.api.nvim_set_hl(0, "SmoothCursor", { fg = p.emr_3 })
		end
	end,
})

local chars = function()
-- stylua: ignore
return {
    'ｱ', 'ｲ', 'ｳ', 'ｴ', 'ｵ',
    'ｶ', 'ｷ', 'ｸ', 'ｹ', 'ｺ',
    'ｻ', 'ｼ', 'ｽ', 'ｾ', 'ｿ',
    'ﾀ', 'ﾁ', 'ﾂ', 'ﾃ', 'ﾄ',
    'ﾅ', 'ﾆ', 'ﾇ', 'ﾈ', 'ﾉ',
    'ﾊ', 'ﾋ', 'ﾌ', 'ﾍ', 'ﾎ',
    'ﾏ', 'ﾐ', 'ﾑ', 'ﾒ', 'ﾓ',
    'ﾔ', 'ﾕ', 'ﾖ',
    'ﾗ', 'ﾘ', 'ﾙ', 'ﾚ', 'ﾛ',
    'ﾜ', 'ｦ', 'ﾝ', '-',
    'Γ', 'Δ', 'Λ', 'Ξ', 'Π',
    'Σ', 'Φ', 'Χ', 'Ψ', 'Ω',
    'α', 'β', 'γ', 'δ', 'ε',
    'ζ', 'η', 'θ', 'ι', 'κ',
    'λ', 'μ', 'ν', 'ξ', 'ο',
    'π', 'ρ', 'σ', 'τ', 'υ',
    'φ', 'ψ', 'ω'
}
end

cursor.setup({
	type = "matrix",
	cursor = "",
	texthl = "SmoothCursor",
	linehl = nil,

	fancy = {
		enable = false, -- enable fancy mode
		head = { cursor = "▷", texthl = "SmoothCursor", linehl = nil }, -- false to disable fancy head
		body = {
			{ cursor = "󰝥", texthl = "SmoothCursorRed" },
			{ cursor = "󰝥", texthl = "SmoothCursorOrange" },
			{ cursor = "●", texthl = "SmoothCursorYellow" },
			{ cursor = "●", texthl = "SmoothCursorGreen" },
			{ cursor = "•", texthl = "SmoothCursorAqua" },
			{ cursor = ".", texthl = "SmoothCursorBlue" },
			{ cursor = ".", texthl = "SmoothCursorPurple" },
		},
		tail = { cursor = nil, texthl = "SmoothCursor" }, -- false to disable fancy tail
	},

	matrix = {
		head = {
			cursor = { "λ" },
			texthl = { "SmoothCursor" },
			linehl = nil,
		},
		body = {
			length = 9,
			cursor = chars(),
			texthl = { "Comment" },
		},
		tail = {
			cursor = chars(),
			texthl = { "SmoothCursor" },
		},
		unstop = false, -- Determines if the cursor should stop or not (false means it will stop)
	},

	autostart = true, -- Automatically start SmoothCursor
	always_redraw = true, -- Redraw the screen on each update
	flyin_effect = nil, -- Choose "bottom" or "top" for flying effect
	speed = 25, -- Max speed is 100 to stick with your current position
	intervals = 35, -- Update intervals in milliseconds
	priority = 10, -- Set marker priority
	timeout = 3000, -- Timeout for animations in milliseconds
	threshold = 3, -- Animate only if cursor moves more than this many lines
	max_threshold = 1000, -- If you move more than this many lines, don't animate (if `nil`, deactivate check)
	disable_float_win = false, -- Disable in floating windows
	enabled_filetypes = nil, -- Enable only for specific file types, e.g., { "lua", "vim" }
	disabled_filetypes = nil, -- Disable for these file types, ignored if enabled_filetypes is set. e.g., { "TelescopePrompt", "NvimTree" }
	-- Show the position of the latest input mode positions.
	-- A value of "enter" means the position will be updated when entering the mode.
	-- A value of "leave" means the position will be updated when leaving the mode.
	-- `nil` = disabled
	show_last_positions = "leave",
})
