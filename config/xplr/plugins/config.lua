local on_key = xplr.config.modes.builtin.default.key_bindings.on_key

require("lib.archive").setup()
require("lib.preview").setup()

on_key["o"] = {
	help = "archive",
	messages = {
		{ SwitchModeCustom = "archive" },
	},
}

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
	smart_view = {
		custom = {
			{ extension = "zip", command = "ouch list {} | nvimpager" },
		},
	},
	open = {
		run_executables = false,
		custom = {
			{ extension = "gz", command = "tar tf {} | nvimpager" },
			{ mime_regex = "text/.*", command = "${VISUAL:-${EDITOR:-nvim}} {}" },
			{ mime_regex = "application/(json|x-sh|x-python|x-shellscript|xml|yaml)", command = "${VISUAL:-${EDITOR:-nvim}} {}" },
			{ mime_regex = ".*", command = "xdg-open {}" },
		},
	},
})

local nuke_on_key = xplr.config.modes.custom.nuke.key_bindings.on_key
on_key["v"] = nuke_on_key.v
on_key["S"] = nuke_on_key.s
on_key["right"] = {
	help = "open/archive",
	messages = {
		{ CallLua = "custom.archive_open" },
	},
}
on_key["enter"] = {
	help = "submit/archive",
	messages = {
		{ CallLua = "custom.archive_enter" },
	},
}
on_key["I"] = {
	help = "draw image preview",
	messages = {
		{ CallLuaSilently = "custom.preview.show_image" },
	},
}
on_key["alt-f"] = {
	help = "copy full path",
	messages = {
		{
			BashExecSilently0 = [===[
				path=${XPLR_FOCUS_PATH:?}
				if printf '%s' "$path" | wl-copy; then
					"$XPLR" -m 'LogSuccess: %q' "copied $path"
				else
					"$XPLR" -m 'LogError: %q' "failed to copy $path"
				fi
			]===],
		},
	},
}
on_key["alt-o"] = {
	help = "open folder gui",
	messages = {
		{ BashExecSilently0 = "thunar \"$PWD\" >/dev/null 2>&1 &" },
	},
}
on_key["ctrl-o"] = {
	help = "open folder gui",
	messages = {
		{ BashExecSilently0 = "thunar \"$PWD\" >/dev/null 2>&1 &" },
	},
}
on_key["alt-i"] = {
	help = "open image viewer",
	messages = {
		{
			BashExecSilently0 = [===[
				mime=$(file --brief --mime-type -- "${XPLR_FOCUS_PATH:?}") || exit 1
				case "$mime" in
					image/*) imv "${XPLR_FOCUS_PATH:?}" >/dev/null 2>&1 & ;;
					video/*) mpv "${XPLR_FOCUS_PATH:?}" >/dev/null 2>&1 & ;;
					*) "$XPLR" -m 'LogError: %q' "not an image or video: $mime" ;;
				esac
			]===],
		},
	},
}

-- Vertical layout: Table above, Preview below
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

-- Modes defined after plugins so all keybindings are inherited
local function inherit_default_keys()
	local keys = {}
	for k, v in pairs(on_key) do
		keys[k] = v
	end
	return keys
end

local preview_keys = inherit_default_keys()
preview_keys["P"] = {
	help = "close preview",
	messages = {
		{ CallLua = "custom.preview.clear_image" },
		{ SwitchLayoutBuiltin = "default" },
		{ SwitchModeBuiltin = "default" },
		"ClearScreen",
	},
}
preview_keys["space"] = {
	help = "toggle selection",
	messages = {
		{ CallLua = "custom.preview.clear_image" },
		"ToggleSelection",
		{ SwitchLayoutCustom = "selection" },
		{ SwitchModeCustom = "selection_mode" },
		"ClearScreen",
	},
}

xplr.config.modes.custom.preview_mode = {
	name = "preview",
	key_bindings = {
		on_key = preview_keys,
	},
}

local selection_keys = inherit_default_keys()
selection_keys["enter"] = {
	help = "quit with result",
	messages = { "PrintResultAndQuit" },
}
selection_keys["P"] = {
	help = "back to preview",
	messages = {
		{ SwitchLayoutCustom = "preview" },
		{ SwitchModeCustom = "preview_mode" },
		"ClearScreen",
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
		"ClearScreen",
	},
}

xplr.config.modes.custom.selection_mode = {
	name = "selection",
	key_bindings = {
		on_key = selection_keys,
	},
}
