local status_ok, spectre = pcall(require, "spectre")
if not status_ok then
	return
end

spectre.setup({
	header = "╰────── ﯒ Spectre ───────────────────────╮",
	open_cmd = "42 vnew",
	line_sep_start = "╭────────────────────────────────────────╮",
	result_padding = "│  ",
	line_sep = "╰────────────────────────────────────────╯",
	mapping = {
		["toggle_line"] = {
			map = "dd",
			cmd = "<cmd>lua require('spectre').toggle_line()<CR>",
			desc = "toggle current item",
		},
		["enter_file"] = {
			map = "<CR>",
			cmd = "<cmd>lua require('spectre.actions').select_entry()<CR>",
			desc = "goto current file",
		},
		["send_to_qf"] = {
			map = "L",
			cmd = "<cmd>lua require('spectre.actions').send_to_qf()<CR> | :q<CR> | :q<CR>",
			desc = "send all item to quickfix",
		},
		["go to replace text"] = {
			map = "n",
			cmd = "2jA",
			desc = "quick jump to replace text from search input",
		},
		["replace_and_quit"] = {
			map = "R",
			cmd = "<cmd>lua require('spectre.actions').run_replace()<CR> | :q<CR>",
			desc = "replace all and quit",
		},
		["run_replace"] = {
			map = "<leader>R",
			cmd = "<cmd>lua require('spectre.actions').run_replace()<CR>",
			desc = "replace all",
		},
		["change_view_mode"] = {
			map = "tv",
			cmd = "<cmd>lua require('spectre').change_view()<CR>",
			desc = "change result view mode",
		},
		["toggle_live_update"] = {
			map = "tu",
			cmd = "<cmd>lua require('spectre').toggle_live_update()<CR>",
			desc = "update change when vim write file.",
		},
		["toggle_ignore_case"] = {
			map = "tc",
			cmd = "<cmd>lua require('spectre').change_options('ignore-case')<CR>",
			desc = "toggle ignore case",
		},
		["toggle_ignore_hidden"] = {
			map = "th",
			cmd = "<cmd>lua require('spectre').change_options('hidden')<CR>",
			desc = "toggle search hidden",
		},
	},
	highlight = {
		ui = "Function",
		filename = "Keyword",
		filedirectory = "Comment",
		search = "Function",
		border = "Comment",
		replace = "Constant",
		path = "Identifier",
	},
	is_insert_mode = true,

	find_engine = {
		["rg"] = {
			options = {
				["ignore-case"] = {
					value = "--ignore-case",
					icon = "",
					desc = "ignore case",
				},
				["hidden"] = {
					value = "--hidden",
					desc = "hidden file",
					icon = "﬒",
				},
			},
		},
	},
	default = { find = { cmd = "rg", options = {} } },
})
