---@diagnostic disable
version = "0.20.2"
local xplr = xplr -- The globally exposed configuration to be overridden.
---@diagnostic enable

-- ### Layouts ----------------------------------------------------------------
--
-- xplr layouts define the structure of the UI, i.e. how many panel we see,
-- placement and size of the panels, how they look etc.
--
-- This is configuration exposed via the `xplr.config.layouts` API.
--
-- `xplr.config.layouts.builtin` contain some built-in panels which can be
-- overridden, but you can't add or remove panels in it.
--
-- You can add new panels in `xplr.config.layouts.custom`.
--
-- ##### Example: Defining Custom Layout
--
-- ![demo](https://s6.gifyu.com/images/layout.png)
--
-- ```lua
-- xplr.config.layouts.builtin.default = {
--   Horizontal = {
--     config = {
--       margin = 1,
--       horizontal_margin = 2,
--       vertical_margin = 3,
--       constraints = {
--         { Percentage = 50 },
--         { Percentage = 50 },
--       }
--     },
--     splits = {
--       "Table",
--       "HelpMenu",
--     }
--   }
-- }
-- ```

-- The default layout
--
-- Type: [Layout](https://xplr.dev/en/layout)
xplr.config.layouts.builtin.default = {
	Horizontal = {
		config = {
			constraints = {
				{ Percentage = 70 },
				{ Percentage = 30 },
			},
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Min = 1 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						"InputAndLogs",
					},
				},
			},
			{
				Vertical = {
					config = {
						constraints = {
							{ Percentage = 30 },
							{ Percentage = 70 },
						},
					},
					splits = {
						"Selection",
						"HelpMenu",
					},
				},
			},
		},
	},
}

-- The layout without help menu
--
-- Type: [Layout](https://xplr.dev/en/layout)
xplr.config.layouts.builtin.no_help = {
	Horizontal = {
		config = {
			constraints = {
				{ Percentage = 70 },
				{ Percentage = 30 },
			},
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Min = 1 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						"InputAndLogs",
					},
				},
			},
			"Selection",
		},
	},
}

-- The layout without selection panel
--
-- Type: [Layout](https://xplr.dev/en/layout)
xplr.config.layouts.builtin.no_selection = {
	Horizontal = {
		config = {
			constraints = {
				{ Percentage = 70 },
				{ Percentage = 30 },
			},
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Min = 1 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						"InputAndLogs",
					},
				},
			},
			"HelpMenu",
		},
	},
}

-- The layout without help menu and selection panel
--
-- Type: [Layout](https://xplr.dev/en/layout)
xplr.config.layouts.builtin.no_help_no_selection = {
	Vertical = {
		config = {
			constraints = {
				{ Length = 3 },
				{ Min = 1 },
				{ Length = 3 },
			},
		},
		splits = {
			"SortAndFilter",
			"Table",
			"InputAndLogs",
		},
	},
}

-- This is where you can define custom layouts
--
-- Type: mapping of the following key-value pairs:
--
-- * key: string
-- * value: [Layout](https://xplr.dev/en/layout)
--
-- Example:
--
-- ```lua
-- xplr.config.layouts.custom.example = "Nothing" -- Show a blank screen
-- xplr.config.general.initial_layout = "example" -- Load the example layout
-- ```
xplr.config.layouts.custom = {}

-- ### Modes ------------------------------------------------------------------
--
-- xplr is a modal file explorer. That means the users switch between different
-- modes, each containing a different set of key bindings to avoid clashes.
-- Users can switch between these modes at run-time.
--
-- The modes can be configured using the `xplr.config.modes` Lua API.
--
-- `xplr.config.modes.builtin` contain some built-in modes which can be
-- overridden, but you can't add or remove modes in it.

-- The builtin default mode.
-- Visit the [Default Key Bindings](https://xplr.dev/en/default-key-bindings)
-- to see what each mode does.
--
-- Type: [Mode](https://xplr.dev/en/mode)

xplr.config.modes.builtin.default.key_bindings.on_key["tab"] =
	xplr.config.modes.builtin.default.key_bindings.on_key["ctrl-i"]
xplr.config.modes.builtin.default.key_bindings.on_key["v"] =
	xplr.config.modes.builtin.default.key_bindings.on_key["space"]
xplr.config.modes.builtin.default.key_bindings.on_key["V"] =
	xplr.config.modes.builtin.default.key_bindings.on_key["ctrl-a"]
xplr.config.modes.builtin.default.key_bindings.on_key["/"] =
	xplr.config.modes.builtin.default.key_bindings.on_key["ctrl-f"]
xplr.config.modes.builtin.default.key_bindings.on_key["h"] =
	xplr.config.modes.builtin.default.key_bindings.on_key["left"]
xplr.config.modes.builtin.default.key_bindings.on_key["j"] =
	xplr.config.modes.builtin.default.key_bindings.on_key["down"]
xplr.config.modes.builtin.default.key_bindings.on_key["k"] =
	xplr.config.modes.builtin.default.key_bindings.on_key["up"]
xplr.config.modes.builtin.default.key_bindings.on_key["l"] =
	xplr.config.modes.builtin.default.key_bindings.on_key["right"]

-- The builtin debug error mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.debug_error = {
	name = "debug error",
	layout = {
		Vertical = {
			config = {
				constraints = {
					{ Min = 14 },
					{ MinLessThanScreenHeight = 14 },
				},
			},
			splits = {
				{
					CustomContent = {
						title = "debug error",
						body = {
							StaticParagraph = {
								render = [[

  Some errors occurred during startup.
  If you think this is a bug, please report it at:

  https://github.com/sayanarijit/xplr/issues/new

  Press `enter` to open the logs in your $EDITOR.
  Press `escape` to ignore the errors and continue with the default config.

  To disable this mode, set `xplr.config.general.disable_debug_error_mode`
  to `true` in your config file.
                ]],
							},
						},
					},
				},
				"InputAndLogs",
			},
		},
	},
	key_bindings = {
		on_key = {
			["enter"] = {
				help = "open logs in editor",
				messages = {
					{
						BashExec = [===[
              cat "${XPLR_PIPE_LOGS_OUT:?}" | ${EDITOR:-vi} -
            ]===],
					},
				},
			},
			["q"] = {
				help = "quit",
				messages = {
					"Quit",
				},
			},
		},
		default = {
			messages = {},
		},
	},
}

-- The builtin recover mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.recover = {
	name = "recover",
	layout = {
		CustomContent = {
			title = " recover ",
			body = {
				StaticParagraph = {
					render = [[

  You pressed an invalid key and went into "recover" mode.
  This mode saves you from performing unwanted actions.

  Let's calm down, press `escape`, and try again.

  To disable this mode, set `xplr.config.general.enable_recover_mode`
  to `false` in your config file.
          ]],
				},
			},
		},
	},
	key_bindings = {
		default = {
			messages = {},
		},
	},
}

-- The builtin go to path mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.go_to_path = {
	name = "go to path",
	key_bindings = {
		on_key = {
			["enter"] = {
				help = "submit",
				messages = {
					{
						BashExecSilently0 = [===[
              PTH=${XPLR_INPUT_BUFFER}
              PTH_ESC=$(printf %q "$PTH")
              if [ -d "$PTH" ]; then
                "$XPLR" -m 'ChangeDirectory: %q' "$PTH"
              elif [ -e "$PTH" ]; then
                "$XPLR" -m 'FocusPath: %q' "$PTH"
              else
                "$XPLR" -m 'LogError: %q' "could not find $PTH_ESC"
              fi
            ]===],
					},
					"PopMode",
				},
			},
			["tab"] = {
				help = "try complete",
				messages = {
					{ CallLuaSilently = "builtin.try_complete_path" },
				},
			},
		},
		default = {
			messages = {
				"UpdateInputBufferFromKey",
			},
		},
	},
}

-- The builtin selection ops mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.selection_ops = {
	name = "selection ops",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["c"] = {
				help = "copy here",
				messages = {
					{
						BashExec0 = [===[
              (while IFS= read -r -d '' PTH; do
                PTH_ESC=$(printf %q "$PTH")
                if cp -vr -- "${PTH:?}" ./; then
                  "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC copied to ."
                else
                  "$XPLR" -m 'LogError: %q' "Failed to copy $PTH_ESC to ."
                fi
              done < "${XPLR_PIPE_SELECTION_OUT:?}")
              "$XPLR" -m ExplorePwdAsync
              "$XPLR" -m ClearSelection
              read -p "[enter to continue]"
            ]===],
					},
					"PopMode",
				},
			},
			["m"] = {
				help = "move here",
				messages = {
					{
						BashExec0 = [===[
              (while IFS= read -r -d '' PTH; do
                PTH_ESC=$(printf %q "$PTH")
                if mv -v -- "${PTH:?}" ./; then
                  "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC moved to ."
                else
                  "$XPLR" -m 'LogError: %q' "Failed to move $PTH_ESC to ."
                fi
              done < "${XPLR_PIPE_SELECTION_OUT:?}")
              "$XPLR" -m ExplorePwdAsync
              read -p "[enter to continue]"
            ]===],
					},
					"PopMode",
				},
			},
			["u"] = {
				help = "clear selection",
				messages = {
					"ClearSelection",
					"PopMode",
				},
			},
		},
	},
}

-- The builtin create mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.create = {
	name = "create",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["d"] = {
				help = "create directory",
				messages = {
					"PopMode",
					{ SwitchModeBuiltin = "create_directory" },
					{ SetInputBuffer = "" },
				},
			},
			["f"] = {
				help = "create file",
				messages = {
					"PopMode",
					{ SwitchModeBuiltin = "create_file" },
					{ SetInputBuffer = "" },
				},
			},
		},
	},
}

-- The builtin create directory mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.create_directory = {
	name = "create directory",
	prompt = "ð ❯ ",
	key_bindings = {
		on_key = {
			["tab"] = {
				help = "try complete",
				messages = {
					{ CallLuaSilently = "builtin.try_complete_path" },
				},
			},
			["enter"] = {
				help = "submit",
				messages = {
					{
						BashExecSilently0 = [===[
              PTH="$XPLR_INPUT_BUFFER"
              PTH_ESC=$(printf %q "$PTH")
              if [ "$PTH" ]; then
                mkdir -p -- "$PTH" \
                && "$XPLR" -m 'SetInputBuffer: ""' \
                && "$XPLR" -m ExplorePwd \
                && "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC created" \
                && "$XPLR" -m 'FocusPath: %q' "$PTH"
              else
                "$XPLR" -m PopMode
              fi
            ]===],
					},
				},
			},
		},
		default = {
			messages = {
				"UpdateInputBufferFromKey",
			},
		},
	},
}

-- The builtin create file mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.create_file = {
	name = "create file",
	prompt = "ƒ ❯ ",
	key_bindings = {
		on_key = {
			["tab"] = {
				help = "try complete",
				messages = {
					{ CallLuaSilently = "builtin.try_complete_path" },
				},
			},
			["enter"] = {
				help = "submit",
				messages = {
					{
						BashExecSilently0 = [===[
              PTH="$XPLR_INPUT_BUFFER"
              PTH_ESC=$(printf %q "$PTH")
              if [ "$PTH" ]; then
                mkdir -p -- "$(dirname $(realpath -m $PTH))" \
                && touch -- "$PTH" \
                && "$XPLR" -m 'SetInputBuffer: ""' \
                && "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC created" \
                && "$XPLR" -m 'ExplorePwd' \
                && "$XPLR" -m 'FocusPath: %q' "$PTH"
              else
                "$XPLR" -m PopMode
              fi
            ]===],
					},
				},
			},
		},
		default = {
			messages = {
				"UpdateInputBufferFromKey",
			},
		},
	},
}

-- The builtin number mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.number = {
	name = "number",
	prompt = ":",
	key_bindings = {
		on_key = {
			["down"] = {
				help = "to down",
				messages = {
					"FocusNextByRelativeIndexFromInput",
					"PopMode",
				},
			},
			["enter"] = {
				help = "to index",
				messages = {
					"FocusByIndexFromInput",
					"PopMode",
				},
			},
			["up"] = {
				help = "to up",
				messages = {
					"FocusPreviousByRelativeIndexFromInput",
					"PopMode",
				},
			},
		},
		on_navigation = {
			messages = {
				"UpdateInputBufferFromKey",
			},
		},
		on_number = {
			help = "input",
			messages = {
				"UpdateInputBufferFromKey",
			},
		},
	},
}

xplr.config.modes.builtin.number.key_bindings.on_key["j"] =
	xplr.config.modes.builtin.number.key_bindings.on_key["down"]
xplr.config.modes.builtin.number.key_bindings.on_key["k"] =
	xplr.config.modes.builtin.number.key_bindings.on_key["up"]

-- The builtin go to mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.go_to = {
	name = "go to",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["f"] = {
				help = "follow symlink",
				messages = {
					"FollowSymlink",
					"PopMode",
				},
			},
			["g"] = {
				help = "top",
				messages = {
					"FocusFirst",
					"PopMode",
				},
			},
			["p"] = {
				help = "path",
				messages = {
					"PopMode",
					{ SwitchModeBuiltin = "go_to_path" },
					{ SetInputBuffer = "" },
				},
			},
			["i"] = {
				help = "initial $PWD",
				messages = {
					"PopMode",
					{
						BashExecSilently0 = [===[
              "$XPLR" -m 'ChangeDirectory: %q' "${XPLR_INITIAL_PWD:?}"
            ]===],
					},
				},
			},
			["x"] = {
				help = "open in gui",
				messages = {
					{
						BashExecSilently0 = [===[
              if [ -z "$OPENER" ]; then
                if command -v xdg-open; then
                  OPENER=xdg-open
                elif command -v open; then
                  OPENER=open
                else
                  "$XPLR" -m 'LogError: "$OPENER not found"'
                  exit 1
                fi
              fi
              (while IFS= read -r -d '' PTH; do
                $OPENER "${PTH:?}" > /dev/null 2>&1
              done < "${XPLR_PIPE_RESULT_OUT:?}")
            ]===],
					},
					"ClearScreen",
					"PopMode",
				},
			},
		},
	},
}

-- The builtin rename mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.rename = {
	name = "rename",
	key_bindings = {
		on_key = {
			["tab"] = {
				help = "try complete",
				messages = {
					{ CallLuaSilently = "builtin.try_complete_path" },
				},
			},
			["enter"] = {
				help = "submit",
				messages = {
					{
						BashExecSilently0 = [===[
              SRC="${XPLR_FOCUS_PATH:?}"
              SRC_ESC=$(printf %q "$SRC")
              TARGET="${XPLR_INPUT_BUFFER:?}"
              TARGET_ESC=$(printf %q "$TARGET")
              if [ -e "${TARGET:?}" ]; then
                "$XPLR" -m 'LogError: %q' "$TARGET_ESC already exists"
              else
                mv -- "${SRC:?}" "${TARGET:?}" \
                  && "$XPLR" -m ExplorePwd \
                  && "$XPLR" -m 'FocusPath: %q' "$TARGET" \
                  && "$XPLR" -m 'LogSuccess: %q' "$SRC_ESC renamed to $TARGET_ESC"
              fi
            ]===],
					},
					"PopMode",
				},
			},
		},
		default = {
			messages = {
				"UpdateInputBufferFromKey",
			},
		},
	},
}

-- The builtin duplicate as mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.duplicate_as = {
	name = "duplicate as",
	key_bindings = {
		on_key = {
			["tab"] = {
				help = "try complete",
				messages = {
					{ CallLuaSilently = "builtin.try_complete_path" },
				},
			},
			["enter"] = {
				help = "submit",
				messages = {
					{
						BashExecSilently0 = [===[
              SRC="${XPLR_FOCUS_PATH:?}"
              SRC_ESC=$(printf %q "$SRC")
              TARGET="${XPLR_INPUT_BUFFER:?}"
              TARGET_ESC=$(printf %q "$TARGET")
              if [ -e "${TARGET:?}" ]; then
                "$XPLR" -m 'LogError: %q' "$TARGET_ESC already exists"
              else
                cp -r -- "${SRC:?}" "${TARGET:?}" \
                  && "$XPLR" -m ExplorePwd \
                  && "$XPLR" -m 'FocusPath: %q' "$TARGET_ESC" \
                  && "$XPLR" -m 'LogSuccess: %q' "$SRC_ESC duplicated as $TARGET_ESC"
              fi
            ]===],
					},
					"PopMode",
				},
			},
		},
		default = {
			messages = {
				"UpdateInputBufferFromKey",
			},
		},
	},
}

-- The builtin delete mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.delete = {
	name = "delete",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["D"] = {
				help = "force delete",
				messages = {
					{
						BashExec0 = [===[
              (while IFS= read -r -d '' PTH; do
                PTH_ESC=$(printf %q "$PTH")
                if rm -rfv -- "${PTH:?}"; then
                  "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC deleted"
                else
                  "$XPLR" -m 'LogError: %q' "Failed to delete $PTH_ESC"
                fi
              done < "${XPLR_PIPE_RESULT_OUT:?}")
              "$XPLR" -m ExplorePwdAsync
              read -p "[enter to continue]"
            ]===],
					},
					"PopMode",
				},
			},
			["d"] = {
				help = "delete",
				messages = {
					{
						BashExec0 = [===[
              (while IFS= read -r -d '' PTH; do
                PTH_ESC=$(printf %q "$PTH")
                if [ -d "$PTH" ] && [ ! -L "$PTH" ]; then
                  if rmdir -v -- "${PTH:?}"; then
                    "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC deleted"
                  else
                    "$XPLR" -m 'LogError: %q' "Failed to delete $PTH_ESC"
                  fi
                else
                  if rm -v -- "${PTH:?}"; then
                    "$XPLR" -m 'LogSuccess: %q' "$PTH_ESC deleted"
                  else
                    "$XPLR" -m 'LogError: %q' "Failed to delete $PTH_ESC"
                  fi
                fi
              done < "${XPLR_PIPE_RESULT_OUT:?}")
              "$XPLR" -m ExplorePwdAsync
              read -p "[enter to continue]"
            ]===],
					},
					"PopMode",
				},
			},
		},
	},
}

-- The builtin action mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.action = {
	name = "action to",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["!"] = {
				help = "shell",
				messages = {
					"PopMode",
					{
						Call0 = {
							command = os.getenv("SHELL") or "bash",
							args = { "-i" },
						},
					},
					"ExplorePwdAsync",
				},
			},
			["c"] = {
				help = "create",
				messages = {
					"PopMode",
					{ SwitchModeBuiltin = "create" },
				},
			},
			["e"] = {
				help = "open in editor",
				messages = {
					{
						BashExec0 = [===[
              ${EDITOR:-vi} "${XPLR_FOCUS_PATH:?}"
            ]===],
					},
					"PopMode",
				},
			},
			["l"] = {
				help = "logs",
				messages = {
					{
						BashExec = [===[
              [ -z "$PAGER" ] && PAGER="less -+F"
              cat -- "${XPLR_PIPE_LOGS_OUT}" | ${PAGER:?}
            ]===],
					},
					"PopMode",
				},
			},
			["s"] = {
				help = "selection operations",
				messages = {
					"PopMode",
					{ SwitchModeBuiltin = "selection_ops" },
				},
			},
			["m"] = {
				help = "toggle mouse",
				messages = {
					"PopMode",
					"ToggleMouse",
				},
			},
			["v"] = {
				help = "vroot",
				messages = {
					"PopMode",
					{ SwitchModeBuiltin = "vroot" },
				},
			},
			["q"] = {
				help = "quit options",
				messages = {
					"PopMode",
					{ SwitchModeBuiltin = "quit" },
				},
			},
		},
		on_number = {
			help = "go to index",
			messages = {
				"PopMode",
				{ SwitchModeBuiltin = "number" },
				"BufferInputFromKey",
			},
		},
	},
}

-- The builtin quit mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.quit = {
	name = "quit",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["enter"] = {
				help = "just quit",
				messages = {
					"Quit",
				},
			},
			["p"] = {
				help = "quit printing pwd",
				messages = {
					"PrintPwdAndQuit",
				},
			},
			["f"] = {
				help = "quit printing focus",
				messages = {
					"PrintFocusPathAndQuit",
				},
			},
			["s"] = {
				help = "quit printing selection",
				messages = {
					"PrintSelectionAndQuit",
				},
			},
			["r"] = {
				help = "quit printing result",
				messages = {
					"PrintResultAndQuit",
				},
			},
		},
	},
}

-- The builtin search mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.search = {
	name = "search",
	prompt = "/",
	key_bindings = {
		on_key = {
			["up"] = {
				help = "up",
				messages = {
					"FocusPrevious",
				},
			},
			["down"] = {
				help = "down",
				messages = {
					"FocusNext",
				},
			},
			["right"] = {
				help = "enter",
				messages = {
					"Enter",
					{ SetInputBuffer = "" },
				},
			},
			["left"] = {
				help = "back",
				messages = {
					"Back",
					{ SetInputBuffer = "" },
				},
			},
			["tab"] = {
				help = "toggle selection",
				messages = {
					"ToggleSelection",
					"FocusNext",
				},
			},
			["enter"] = {
				help = "submit",
				messages = {
					"AcceptSearch",
					"PopMode",
				},
			},
			["esc"] = {
				help = "cancel",
				messages = {
					"CancelSearch",
					"PopMode",
				},
			},
		},
		default = {
			messages = {
				"UpdateInputBufferFromKey",
				"SearchFuzzyFromInput",
				"ExplorePwdAsync",
			},
		},
	},
}

xplr.config.modes.builtin.search.key_bindings.on_key["ctrl-n"] =
	xplr.config.modes.builtin.search.key_bindings.on_key["down"]
xplr.config.modes.builtin.search.key_bindings.on_key["ctrl-p"] =
	xplr.config.modes.builtin.search.key_bindings.on_key["up"]

-- The builtin filter mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.filter = {
	name = "filter",
	key_bindings = {
		on_key = {
			["r"] = {
				help = "relative path does match regex",
				messages = {
					{ SwitchModeBuiltin = "relative_path_does_match_regex" },
					{ SetInputBuffer = "" },
					{ AddNodeFilterFromInput = "RelativePathDoesMatchRegex" },
					"ExplorePwdAsync",
				},
			},
			["R"] = {
				help = "relative path does not match regex",
				messages = {
					{
						SwitchModeBuiltin = "relative_path_does_not_match_regex",
					},
					{ SetInputBuffer = "" },
					{
						AddNodeFilterFromInput = "RelativePathDoesNotMatchRegex",
					},
					"ExplorePwdAsync",
				},
			},
			["backspace"] = {
				help = "remove last filter",
				messages = {
					"RemoveLastNodeFilter",
					"ExplorePwdAsync",
				},
			},
			["ctrl-r"] = {
				help = "reset filters",
				messages = {
					"ResetNodeFilters",
					"ExplorePwdAsync",
				},
			},
			["ctrl-u"] = {
				help = "clear filters",
				messages = {
					"ClearNodeFilters",
					"ExplorePwdAsync",
				},
			},
		},
	},
}

-- The builtin relative_path_does_match_regex mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.relative_path_does_match_regex = {
	name = "relative path does match regex",
	prompt = xplr.config.general.sort_and_filter_ui.filter_identifiers.RelativePathDoesMatchRegex.format,
	key_bindings = {
		on_key = {
			["enter"] = {
				help = "submit",
				messages = {
					"PopMode",
				},
			},
			["esc"] = {
				messages = {
					{
						RemoveNodeFilterFromInput = "RelativePathDoesMatchRegex",
					},
					"PopMode",
					"ExplorePwdAsync",
				},
			},
		},
		default = {
			messages = {
				{ RemoveNodeFilterFromInput = "RelativePathDoesMatchRegex" },
				"UpdateInputBufferFromKey",
				{ AddNodeFilterFromInput = "RelativePathDoesMatchRegex" },
				"ExplorePwdAsync",
			},
		},
	},
}

-- The builtin relative_path_does_not_match_regex mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.relative_path_does_not_match_regex = {
	name = "relative path does not match regex",
	prompt = xplr.config.general.sort_and_filter_ui.filter_identifiers.RelativePathDoesNotMatchRegex.format,
	key_bindings = {
		on_key = {
			["enter"] = {
				help = "submit",
				messages = {
					"PopMode",
				},
			},
			["esc"] = {
				messages = {
					{
						RemoveNodeFilterFromInput = "RelativePathDoesNotMatchRegex",
					},
					"PopMode",
					"ExplorePwdAsync",
				},
			},
		},
		default = {
			messages = {
				{ RemoveNodeFilterFromInput = "RelativePathDoesNotMatchRegex" },
				"UpdateInputBufferFromKey",
				{ AddNodeFilterFromInput = "RelativePathDoesNotMatchRegex" },
				"ExplorePwdAsync",
			},
		},
	},
}

-- The builtin sort mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.sort = {
	name = "sort",
	key_bindings = {
		on_key = {
			["!"] = {
				help = "reverse sorters",
				messages = {
					"ReverseNodeSorters",
					"ExplorePwdAsync",
				},
			},
			["E"] = {
				help = "by canonical extension reverse",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByCanonicalExtension",
							reverse = true,
						},
					},
					"ExplorePwdAsync",
				},
			},
			["M"] = {
				help = "by canonical mime essence reverse",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByCanonicalMimeEssence",
							reverse = true,
						},
					},
					"ExplorePwdAsync",
				},
			},
			["N"] = {
				help = "by node type reverse",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByCanonicalIsDir",
							reverse = true,
						},
					},
					{
						AddNodeSorter = {
							sorter = "ByCanonicalIsFile",
							reverse = true,
						},
					},
					{
						AddNodeSorter = {
							sorter = "ByIsSymlink",
							reverse = true,
						},
					},
					"ExplorePwdAsync",
				},
			},
			["R"] = {
				help = "by relative path reverse",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByIRelativePath",
							reverse = true,
						},
					},
					"ExplorePwdAsync",
				},
			},
			["S"] = {
				help = "by size reverse",
				messages = {
					{ AddNodeSorter = { sorter = "BySize", reverse = true } },
					"ExplorePwdAsync",
				},
			},
			["backspace"] = {
				help = "remove last sorter",
				messages = {
					"RemoveLastNodeSorter",
					"ExplorePwdAsync",
				},
			},
			["ctrl-r"] = {
				help = "reset sorters",
				messages = {
					"ResetNodeSorters",
					"ExplorePwdAsync",
				},
			},
			["ctrl-u"] = {
				help = "clear sorters",
				messages = {
					"ClearNodeSorters",
					"ExplorePwdAsync",
				},
			},
			["e"] = {
				help = "by canonical extension",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByCanonicalExtension",
							reverse = false,
						},
					},
					"ExplorePwdAsync",
				},
			},
			["enter"] = {
				help = "submit",
				messages = {
					"PopMode",
				},
			},
			["m"] = {
				help = "by canonical mime essence",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByCanonicalMimeEssence",
							reverse = false,
						},
					},
					"ExplorePwdAsync",
				},
			},
			["n"] = {
				help = "by node type",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByCanonicalIsDir",
							reverse = false,
						},
					},
					{
						AddNodeSorter = {
							sorter = "ByCanonicalIsFile",
							reverse = false,
						},
					},
					{
						AddNodeSorter = {
							sorter = "ByIsSymlink",
							reverse = false,
						},
					},
					"ExplorePwdAsync",
				},
			},
			["r"] = {
				help = "by relative path",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByIRelativePath",
							reverse = false,
						},
					},
					"ExplorePwdAsync",
				},
			},
			["s"] = {
				help = "by size",
				messages = {
					{ AddNodeSorter = { sorter = "BySize", reverse = false } },
					"ExplorePwdAsync",
				},
			},

			["c"] = {
				help = "by created",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByCreated",
							reverse = false,
						},
					},
					"ExplorePwdAsync",
				},
			},

			["C"] = {
				help = "by created reverse",
				messages = {
					{
						AddNodeSorter = { sorter = "ByCreated", reverse = true },
					},
					"ExplorePwdAsync",
				},
			},

			["l"] = {
				help = "by last modified",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByLastModified",
							reverse = false,
						},
					},
					"ExplorePwdAsync",
				},
			},

			["L"] = {
				help = "by last modified reverse",
				messages = {
					{
						AddNodeSorter = {
							sorter = "ByLastModified",
							reverse = true,
						},
					},
					"ExplorePwdAsync",
				},
			},
		},
	},
}

-- The builtin switch layout mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.switch_layout = {
	name = "switch layout",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["1"] = {
				help = "default",
				messages = {
					{ SwitchLayoutBuiltin = "default" },
					"PopMode",
				},
			},
			["2"] = {
				help = "no help menu",
				messages = {
					{ SwitchLayoutBuiltin = "no_help" },
					"PopMode",
				},
			},
			["3"] = {
				help = "no selection panel",
				messages = {
					{ SwitchLayoutBuiltin = "no_selection" },
					"PopMode",
				},
			},
			["4"] = {
				help = "no help or selection",
				messages = {
					{ SwitchLayoutBuiltin = "no_help_no_selection" },
					"PopMode",
				},
			},
		},
	},
}

-- The builtin vroot mode.
--
-- Type: [Mode](https://xplr.dev/en/mode)
xplr.config.modes.builtin.vroot = {
	name = "vroot",
	layout = "HelpMenu",
	key_bindings = {
		on_key = {
			["v"] = {
				help = "toggle vroot",
				messages = {
					"PopMode",
					"ToggleVroot",
				},
			},
			["."] = {
				help = "vroot $PWD",
				messages = {
					"PopMode",
					{
						BashExecSilently0 = [===[
              "$XPLR" -m 'SetVroot: %q' "${PWD:?}"
            ]===],
					},
				},
			},
			["/"] = {
				help = "vroot /",
				messages = {
					"PopMode",
					{ SetVroot = "/" },
				},
			},
			["~"] = {
				help = "vroot $HOME",
				messages = {
					"PopMode",
					{
						BashExecSilently0 = [===[
              "$XPLR" -m 'SetVroot: %q' "${HOME:?}"
            ]===],
					},
				},
			},
			["ctrl-r"] = {
				help = "reset vroot",
				messages = {
					"PopMode",
					"ResetVroot",
				},
			},
			["ctrl-u"] = {
				help = "unset vroot",
				messages = {
					"PopMode",
					"UnsetVroot",
				},
			},
		},
	},
}

-- This is where you define custom modes.
--
-- Type: mapping of the following key-value pairs:
--
-- * key: string
-- * value: [Mode](https://xplr.dev/en/mode)
--
-- Example:
--
-- ```lua
-- xplr.config.modes.custom.example = {
--   name = "example",
--   key_bindings = {
--     on_key = {
--       enter = {
--         help = "default mode",
--         messages = {
--           "PopMode",
--           { SwitchModeBuiltin = "default" },
--         },
--       },
--     },
--   },
-- }
--
-- xplr.config.general.initial_mode = "example"
-- ```
xplr.config.modes.custom = {}

-- ## Function ----------------------------------------------------------------
--
-- While `xplr.config` defines all the static parts of the configuration,
-- `xplr.fn` defines all the dynamic parts using functions.
--
-- See: [Lua Function Calls](https://xplr.dev/en/lua-function-calls)
--
-- As always, `xplr.fn.builtin` is where the built-in functions are defined
-- that can be overwritten.

-- Tries to auto complete the path in the input buffer
xplr.fn.builtin.try_complete_path = function(m)
	if not m.input_buffer then
		return
	end

	local function matches_all(str, paths)
		for _, path in ipairs(paths) do
			if string.sub(path, 1, #str) ~= str then
				return false
			end
		end
		return true
	end

	local path = m.input_buffer
	local explorer_config = {
		filters = {
			{
				filter = "RelativePathDoesStartWith",
				input = xplr.util.basename(path),
			},
		},
	}
	local parent = xplr.util.dirname(path)
	if not parent or parent == "" then
		parent = "./"
	elseif parent ~= "/" then
		parent = parent .. "/"
	end

	local nodes = xplr.util.explore(parent, explorer_config)
	local found = {}
	for _, node in ipairs(nodes) do
		table.insert(found, parent .. node.relative_path)
	end
	local count = #found

	if count == 0 then
		return
	elseif count == 1 then
		return {
			{ SetInputBuffer = found[1] },
		}
	else
		local first = found[1]
		while #first > #path and matches_all(path, found) do
			path = string.sub(first, 1, #path + 1)
		end

		if matches_all(path, found) then
			return {
				{ SetInputBuffer = path },
			}
		end

		return {
			{ SetInputBuffer = string.sub(path, 1, #path - 1) },
		}
	end
end

-- Renders the first column in the table
xplr.fn.builtin.fmt_general_table_row_cols_0 = function(m)
	local r = ""
	if m.is_before_focus then
		r = r .. " -"
	else
		r = r .. "  "
	end

	r = r .. m.relative_index .. "│" .. m.index

	return r
end

-- Renders the second column in the table
xplr.fn.builtin.fmt_general_table_row_cols_1 = function(m)
	local r = m.tree .. m.prefix

	local function path_escape(path)
		return string.gsub(string.gsub(path, "\\", "\\\\"), "\n", "\\n")
	end

	if m.meta.icon == nil then
		r = r .. ""
	else
		r = r .. m.meta.icon .. " "
	end

	r = r .. path_escape(m.relative_path)

	if m.is_dir then
		r = r .. "/"
	end

	r = r .. m.suffix .. " "

	if m.is_symlink then
		r = r .. "-> "

		if m.is_broken then
			r = r .. "×"
		else
			r = r .. path_escape(m.symlink.absolute_path)

			if m.symlink.is_dir then
				r = r .. "/"
			end
		end
	end

	return r
end

-- Renders the third column in the table
xplr.fn.builtin.fmt_general_table_row_cols_2 = function(m)
	local no_color = os.getenv("NO_COLOR")

	local function green(x)
		if no_color == nil then
			return "\x1b[32m" .. x .. "\x1b[0m"
		else
			return x
		end
	end

	local function yellow(x)
		if no_color == nil then
			return "\x1b[33m" .. x .. "\x1b[0m"
		else
			return x
		end
	end

	local function red(x)
		if no_color == nil then
			return "\x1b[31m" .. x .. "\x1b[0m"
		else
			return x
		end
	end

	local function bit(x, color, cond)
		if cond then
			return color(x)
		else
			return color("-")
		end
	end

	local p = m.permissions

	local r = ""

	r = r .. bit("r", green, p.user_read)
	r = r .. bit("w", yellow, p.user_write)

	if p.user_execute == false and p.setuid == false then
		r = r .. bit("-", red, p.user_execute)
	elseif p.user_execute == true and p.setuid == false then
		r = r .. bit("x", red, p.user_execute)
	elseif p.user_execute == false and p.setuid == true then
		r = r .. bit("S", red, p.user_execute)
	else
		r = r .. bit("s", red, p.user_execute)
	end

	r = r .. bit("r", green, p.group_read)
	r = r .. bit("w", yellow, p.group_write)

	if p.group_execute == false and p.setuid == false then
		r = r .. bit("-", red, p.group_execute)
	elseif p.group_execute == true and p.setuid == false then
		r = r .. bit("x", red, p.group_execute)
	elseif p.group_execute == false and p.setuid == true then
		r = r .. bit("S", red, p.group_execute)
	else
		r = r .. bit("s", red, p.group_execute)
	end

	r = r .. bit("r", green, p.other_read)
	r = r .. bit("w", yellow, p.other_write)

	if p.other_execute == false and p.setuid == false then
		r = r .. bit("-", red, p.other_execute)
	elseif p.other_execute == true and p.setuid == false then
		r = r .. bit("x", red, p.other_execute)
	elseif p.other_execute == false and p.setuid == true then
		r = r .. bit("T", red, p.other_execute)
	else
		r = r .. bit("t", red, p.other_execute)
	end

	return r
end

-- Renders the fourth column in the table
xplr.fn.builtin.fmt_general_table_row_cols_3 = function(m)
	if not m.is_dir then
		return m.human_size
	else
		return ""
	end
end

-- Renders the fifth column in the table
xplr.fn.builtin.fmt_general_table_row_cols_4 = function(m)
	return tostring(
		os.date("%a %b %d %H:%M:%S %Y", m.last_modified / 1000000000)
	)
end

-- This is where the custom functions can be added.
--
-- There is currently no restriction on what kind of functions can be defined
-- in `xplr.fn.custom`.
--
-- You can also use nested tables such as
-- `xplr.fn.custom.my_plugin.my_function` to define custom functions.
xplr.fn.custom = {}

-- ## Hooks -------------------------------------------------------------------
--
-- This section of the configuration cannot be overwritten by another config
-- file or plugin, since this is an optional lua return statement specific to
-- each config file. It can be used to define things that should be explicit
-- for reasons like performance concerns, such as hooks.
--
-- Plugins should expose the hooks, and require users to subscribe to them
-- explicitly.
--
-- Example:
--
-- ```lua
-- return {
--   -- Add messages to send when the xplr loads.
--   -- This is similar to the `--on-load` command-line option.
--   --
--   -- Type: list of [Message](https://xplr.dev/en/message#message)s
--   on_load = {
--     { LogSuccess = "Configuration successfully loaded!" },
--     { CallLuaSilently = "custom.some_plugin_with_hooks.on_load" },
--   },
--
--   -- Add messages to send when the directory changes.
--   --
--   -- Type: list of [Message](https://xplr.dev/en/message#message)s
--   on_directory_change = {
--     { LogSuccess = "Changed directory" },
--     { CallLuaSilently = "custom.some_plugin_with_hooks.on_directory_change" },
--   },
--
--   -- Add messages to send when the focus changes.
--   --
--   -- Type: list of [Message](https://xplr.dev/en/message#message)s
--   on_focus_change = {
--     { LogSuccess = "Changed focus" },
--     { CallLuaSilently = "custom.some_plugin_with_hooks.on_focus_change" },
--   }
--
--   -- Add messages to send when the mode is switched.
--   --
--   -- Type: list of [Message](https://xplr.dev/en/message#message)s
--   on_mode_switch = {
--     { LogSuccess = "Switched mode" },
--     { CallLuaSilently = "custom.some_plugin_with_hooks.on_mode_switch" },
--   }
--
--   -- Add messages to send when the layout is switched
--   --
--   -- Type: list of [Message](https://xplr.dev/en/message#message)s
--   on_layout_switch = {
--     { LogSuccess = "Switched layout" },
--     { CallLuaSilently = "custom.some_plugin_with_hooks.on_layout_switch" },
--   }
-- }
-- ```

return {
	on_load = {},
	on_directory_change = {},
	on_focus_change = {},
	on_mode_switch = {},
	on_layout_switch = {},
}
