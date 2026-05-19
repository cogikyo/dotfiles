local modes = xplr.config.modes.builtin
local on_key = modes.default.key_bindings.on_key
local sort_on_key = modes.sort.key_bindings.on_key

local sort_modified = sort_on_key["l"]
local sort_modified_reverse = sort_on_key["L"]
local sort_mime = sort_on_key["m"]
local sort_mime_reverse = sort_on_key["M"]

sort_on_key["m"] = sort_modified
sort_on_key["t"] = sort_mime

if sort_modified_reverse then
	sort_on_key["M"] = sort_modified_reverse
end

if sort_mime_reverse then
	sort_on_key["T"] = sort_mime_reverse
end

modes.create_directory.prompt = "  (create dir)  "
modes.create_file.prompt = "  (create file)  "
modes.number.prompt = "  "
modes.rename.prompt = "  (rename)  "

modes.switch_layout = {
	name = "switch layout",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["s"] = {
				help = "selection",
				messages = {
					{ SwitchLayoutBuiltin = "default" },
					"PopMode",
				},
			},
			["n"] = {
				help = "no selection",
				messages = {
					{ SwitchLayoutBuiltin = "no_selection" },
					"PopMode",
				},
			},
		},
	},
}

on_key["m"] = {
	help = "bookmark",
	messages = {
		{
			BashExecSilently0 = [===[
        PTH="${XPLR_FOCUS_PATH:?}"
        PTH_ESC=$(printf %q "$PTH")
        if echo "${PTH:?}" >> "${XPLR_SESSION_PATH:?}/bookmarks"; then
          "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC added to bookmarks"
        else
          "$XPLR" -m 'LogError: %q' "Failed to bookmark $PTH_ESC"
        fi
      ]===],
		},
	},
}

on_key["`"] = {
	help = "go to bookmark",
	messages = {
		{
			BashExec0 = [===[
        PTH=$(cat "${XPLR_SESSION_PATH:?}/bookmarks" | fzf --no-sort)
        PTH_ESC=$(printf %q "$PTH")
        if [ "$PTH" ]; then
          "$XPLR" -m 'FocusPath: %q' "$PTH"
        fi
      ]===],
		},
	},
}

on_key["R"] = {
	help = "batch rename",
	messages = { { BashExec = [===[ renamer ]===] } },
}

-- Placeholder; fully wired after preview plugin loads in plugins/config.lua
on_key["P"] = {
	help = "toggle preview",
	messages = {
		{ SwitchLayoutCustom = "preview" },
		{ SwitchModeCustom = "preview_mode" },
	},
}
