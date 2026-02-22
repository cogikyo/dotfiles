return {
	"nvim-lualine/lualine.nvim",
	dependencies = {
		"nvim-tree/nvim-web-devicons",
	},
	config = function()
		local lualine_ok, lualine = pcall(require, "lualine")
		if not lualine_ok then
			return
		end

		local palette_ok, p = pcall(require, "vagari.palette")
		if not palette_ok then
			return
		end

		local ext_ns = vim.api.nvim_create_namespace("lualine_ext_bg")
		for _, hl in ipairs({ "StatusLine", "StatusLineNC", "lualine_c_normal", "lualine_c_inactive" }) do
			vim.api.nvim_set_hl(ext_ns, hl, { bg = p.bg, fg = p.bg })
		end
		vim.api.nvim_create_autocmd("BufWinEnter", {
			callback = function()
				local ft = vim.bo.filetype
				if ft == "NvimTree" or ft == "undotree" or ft == "diff" then
					vim.api.nvim_win_set_hl_ns(vim.api.nvim_get_current_win(), ext_ns)
				end
			end,
		})

		local hide_in_width = function()
			return vim.fn.winwidth(0) > 80
		end

		local branch = {
			"branch",
			icon = "",
		}

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
			colored = true,
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
			newfile_status = true,
			path = 1,
			shorting_target = 80,
			icon = nil,
			symbols = {
				modified = "",
				readonly = "󰍁",
				unnamed = "󱙝",
				newfile = "",
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
					[""] = p.prp_4,
				}
				return { fg = mode_color[vim.fn.mode()] }
			end,
		}

		local lsp_status = {
			"lsp_status",
			icon = "",
			symbols = {
				spinner = { "⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏" },
				done = "",
				separator = " ",
			},
			show_name = true,
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
				return " " .. last_search .. "(" .. searchcount.current .. "/" .. searchcount.total .. ")"
			end,
			color = { fg = p.orn_2 },
		}

		local function icon()
			return [[ ]]
		end

		local function make_extension(ft, label, ext_icon, opts)
			opts = opts or {}
			local active_bg = opts.active_bg or p.orn_4
			local inactive_bg = opts.inactive_bg or p.rst_2
			local inactive_fg = opts.inactive_fg or p.blk_1
			local function get_label()
				local text = type(label) == "function" and label() or label
				return ext_icon .. " " .. text
			end
			local function make_sections(label_color)
				return {
					lualine_a = { { get_label, color = label_color, separator = "" } },
					lualine_b = { { function() return "" end, color = { fg = label_color.bg, bg = p.bg }, padding = 0 } },
					lualine_c = { { function() return string.rep(" ", vim.api.nvim_win_get_width(0)) end, color = { fg = p.bg, bg = p.bg }, padding = 0 } },
					lualine_x = {},
					lualine_y = {},
					lualine_z = {},
				}
			end
			return {
				filetypes = { ft },
				sections = make_sections({ fg = p.blk_1, bg = active_bg, gui = "bold" }),
				inactive_sections = make_sections({ fg = inactive_fg, bg = inactive_bg }),
			}
		end

		local lazy = {
			require("lazy.status").updates,
			cond = require("lazy.status").has_updates,
			color = { fg = p.glc_4 },
		}

		lualine.setup({
			options = {
				theme = "vagari",
				section_separators = { left = "", right = "" },
				component_separators = { left = "⊸", right = "⟜" },
				always_divide_middle = false,
			},
			sections = {
				lualine_a = { mode },
				lualine_b = { branch, filename },
				lualine_c = { diff },
				lualine_x = {
					lazy,
					lsp_diagnostics,
					lsp_status,

					{ require("recorder").recordingStatus },
					{ require("recorder").displaySlots },
					search,
				},
				lualine_y = { filetype },
				lualine_z = { icon },
			},
			inactive_sections = {
				lualine_a = {},
				lualine_b = {
					{
						function()
							local bufname = vim.fn.expand("%:t")
							if bufname ~= "" then
								local devicons_ok, devicons = pcall(require, "nvim-web-devicons")
								local file_icon = ""
								if devicons_ok then
									file_icon = devicons.get_icon(bufname, vim.fn.expand("%:e"), { default = true })
										or " "
								end
								return file_icon .. "  " .. vim.fn.expand("%:.")
							end
							local ft = vim.bo.filetype
							if ft ~= "" then
								return ft
							end
							return "󰊠 "
						end,
					},
				},
				lualine_c = {},
				lualine_x = {
					{
						function()
							return vim.fn.line("$") .. "L"
						end,
					},
				},
				lualine_y = {
					{
						"diff",
						colored = true,
						symbols = { added = " ", modified = " ", removed = " " },
					},
				},
				lualine_z = {
					{
						function()
							if vim.bo.modified then
								return ""
							end
							return "󱣪 "
						end,
					},
				},
			},
			tabline = {},
			extensions = {
				make_extension("NvimTree", function()
					local api_ok, api = pcall(require, "nvim-tree.api")
					if api_ok then
						local node = api.tree.get_nodes()
						if node and node.absolute_path then
							local home = vim.env.HOME
							local path = node.absolute_path
							if home and path == home then
								return vim.env.USER or vim.fn.fnamemodify(home, ":t")
							end
							if home and vim.fn.fnamemodify(path, ":h") == home then
								return vim.fn.fnamemodify(path, ":t")
							end
							return "../" .. vim.fn.fnamemodify(path, ":t")
						end
					end
					return "Files"
				end, "󰙅", { active_bg = p.orn_2, inactive_bg = p.glc_1, inactive_fg = p.blu_1 }),
				make_extension("undotree", "Undotree", "󰕍"),
				make_extension("diff", "Undodiff", "󰕛"),
			},
		})
	end,
}
