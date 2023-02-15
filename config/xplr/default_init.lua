---@diagnostic disable
version = "0.20.2"
local xplr = xplr -- The globally exposed configuration to be overridden.
---@diagnostic enable

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
