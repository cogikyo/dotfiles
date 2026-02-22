local on_key = xplr.config.modes.builtin.default.key_bindings.on_key

require("ouch").setup({
	mode = "default",
	key = "o",
})

require("zoxide").setup({ key = "z" })

local ok, trash = pcall(require, "trash-cli")
if ok then
	trash.setup()
end

require("fzf").setup({
	key = "t",
	recursive = true,
	enter_dir = true,
})

require("nuke").setup({
	pager = "$PAGER",
	open = {
		run_executables = false,
		custom = {
			{ extension = "gz", command = "tar tf {} | nvimpager" },
			{ mime_regex = ".*", command = "xdg-open {}" },
		},
	},
})

local nuke_on_key = xplr.config.modes.custom.nuke.key_bindings.on_key
on_key["v"] = nuke_on_key.v
on_key["right"] = nuke_on_key.o

local preview_ok, preview = pcall(require, "preview")
if preview_ok then
	preview.setup({
		as_default = false,
		keybind = "P",
		left_pane_constraint = { Percentage = 55 },
		right_pane_constraint = { Percentage = 45 },
		style = false,
		text = {
			enable = true,
			highlight = {
				enable = false,
				method = "truecolor",
				style = nil,
			},
		},
		image = {
			enable = false,
			method = "kitty",
		},
		directory = {
			enable = true,
		},
	})

	-- Override plugin's layout with vertical (Table above, Preview below)
	xplr.config.layouts.custom.preview = {
		Horizontal = {
			config = {
				margin = 1,
				horizontal_margin = 2,
				constraints = { { Percentage = 100 } },
			},
			splits = {
				{
					Vertical = {
						config = {
							constraints = {
								{ Length = 3 },
								{ Percentage = 50 },
								{ Percentage = 50 },
								{ Length = 3 },
							},
						},
						splits = {
							"SortAndFilter",
							"Table",
							{
								CustomContent = {
									title = "  Preview",
									body = {
										DynamicParagraph = {
											render = "custom.preview.render",
										},
									},
								},
							},
							"InputAndLogs",
						},
					},
				},
			},
		},
	}

	-- Custom modes (defined after plugins so all keybindings are inherited)
	local function inherit_default_keys()
		local keys = {}
		for k, v in pairs(on_key) do
			keys[k] = v
		end
		return keys
	end

	-- Preview mode (default startup mode)
	local preview_keys = inherit_default_keys()
	preview_keys["P"] = {
		help = "close preview",
		messages = {
			{ SwitchLayoutBuiltin = "default" },
			{ SwitchModeBuiltin = "default" },
		},
	}
	preview_keys["space"] = {
		help = "toggle selection",
		messages = {
			"ToggleSelection",
			{ SwitchLayoutCustom = "selection" },
			{ SwitchModeCustom = "selection_mode" },
		},
	}

	xplr.config.modes.custom.preview_mode = {
		name = "preview",
		key_bindings = {
			on_key = preview_keys,
		},
	}

	-- Selection mode (shows selection panel instead of preview)
	local selection_keys = inherit_default_keys()
	selection_keys["P"] = {
		help = "back to preview",
		messages = {
			{ SwitchLayoutCustom = "preview" },
			{ SwitchModeCustom = "preview_mode" },
		},
	}
	selection_keys["space"] = {
		help = "toggle selection",
		messages = {
			"ToggleSelection",
			{
				BashExecSilently0 = [===[
					if [ -z "$XPLR_SELECTION" ]; then
						"$XPLR" -m 'SwitchLayoutCustom: preview' -m 'SwitchModeCustom: preview_mode'
					fi
				]===],
			},
		},
	}
	selection_keys["u"] = {
		help = "clear selection",
		messages = {
			"ClearSelection",
			{ SwitchLayoutCustom = "preview" },
			{ SwitchModeCustom = "preview_mode" },
		},
	}

	xplr.config.modes.custom.selection_mode = {
		name = "selection",
		key_bindings = {
			on_key = selection_keys,
		},
	}

	-- Start in preview mode when plugin is available
	xplr.config.general.initial_mode = "preview_mode"
	xplr.config.general.initial_layout = "preview"
end
