return {
	"gen740/smoothcursor.nvim",
	config = function()
		local cursor_ok, cursor = pcall(require, "smoothcursor")
		if not cursor_ok then
			return
		end

		local palette_ok, p = pcall(require, "vagari.palette")
		if not palette_ok then
			return
		end

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
				elseif current_mode == "\22" then
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
				unstop = false,
			},
			autostart = true,
			always_redraw = true,
			flyin_effect = nil,
			speed = 25,
			intervals = 35,
			priority = 10,
			timeout = 3000,
			threshold = 3,
			max_threshold = 1000,
			disable_float_win = false,
			enabled_filetypes = nil,
			disabled_filetypes = nil,
			show_last_positions = "leave",
		})
	end,
}
