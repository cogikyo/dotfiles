-- stylua: ignore start
local device

local svalboard = {
	list = {
		"svalboard-lightly",
		"svalboard-lightly-keyboard",
		"svalboard-lightly-system-control",
		"svalboard-lightly-consumer-control",
	},
}
local corne = { inclusive = false, list = svalboard.list }

local function describe(description)
	if device == corne then
		return "Corne: " .. (description or "")
	end
	return description
end

local function bind(keys, description, action)
	if type(action) == "string" then
		action = hl.dsp.exec_cmd(action)
	end

	hl.bind(keys, action, { description = describe(description), device = device })
end

local function super(keys, description, action)
	bind("SUPER + " .. keys, description, action)
end

local function alt(keys, description, action)
	bind("ALT + " .. keys, description, action)
end

local function keyboard(scope, binds)
	device = scope
	binds()
	device = nil
end

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ hyprd focus control                                                           │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

-- ├┤ threebody layout ├───────────────────────────────────────────────────────────┤
alt("A", "Editor",  "hyprd three-body editor")
alt("C", "Agents",  "hyprd three-body agents")
alt("R", "Browser", "hyprd three-body browser")
alt("X", "Dismiss", "dunstctl close")

-- ├┤ tabs ├───────────────────────────────────────────────────────────────────────┤
super("Y", "Tab 1", "hyprd tab 1")
super("N", "Tab 2", "hyprd tab 2")
super("I", "Tab 3", "hyprd tab 3")
super("O", "Tab 4", "hyprd tab 4")
super("L", "Tab 5", "hyprd tab 5")

super("SHIFT + N", "Toggle tab group 1", "hyprd tab 1 group")
super("SHIFT + I", "Toggle tab group 2", "hyprd tab 2 group")
super("SHIFT + O", "Toggle tab group 3", "hyprd tab 3 group")

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ window management                                                             │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

-- ├┤ core conrols ├───────────────────────────────────────────────────────────────┤
super("X", "Close active window", hl.dsp.window.close())
super("F", "Toggle floating",    "hyprd float")
super("V", "Screen share mode",   "hyprd share")
super("F11", "Toggle full screen",  hl.dsp.window.fullscreen({ mode = "fullscreen", action = "toggle" }))

hl.bind("SUPER + mouse:273", hl.dsp.window.drag(),   { mouse = true })
hl.bind("SUPER + mouse:274", hl.dsp.window.resize(), { mouse = true })

-- ├┤ move windows ├───────────────────────────────────────────────────────────────┤
super("Left",  "Move window left",    hl.dsp.window.move({ direction = "left" }))
super("Right", "Move window right",   hl.dsp.window.move({ direction = "right" }))
super("Up",    "Move window up",      hl.dsp.window.move({ direction = "up" }))
super("Down",  "Move window down",    hl.dsp.window.move({ direction = "down" }))
super("Home",  "Move workspace down", "hyprd ws down")
super("End",   "Move workspace up",   "hyprd ws up")

-- ├┤ touchpad ├───────────────────────────────────────────────────────────────────┤
local last = 6

local function swipe(step, skip_empty)
	return function()
		local active = hl.get_active_workspace()
		if not active then
			return
		end
		local id = active.id + step
		while skip_empty and id >= 1 and id <= last do
			local ws = hl.get_workspace(id)
			if ws and ws.windows > 0 then
				break
			end
			id = id + step
		end
		if id >= 1 and id <= last then
			hl.dispatch(hl.dsp.exec_cmd("hyprd ws " .. id))
		end
	end
end

hl.gesture({ fingers = 3, direction = "left",  action = swipe(1, false) })
hl.gesture({ fingers = 3, direction = "right", action = swipe(-1, false) })
hl.gesture({ fingers = 3, direction = "left",  mods = "SUPER", action = swipe(1, true) })
hl.gesture({ fingers = 3, direction = "right", mods = "SUPER", action = swipe(-1, true) })
hl.gesture({ fingers = 3, direction = "up",    action = function() hl.dispatch(hl.dsp.exec_cmd("hyprd ws 1")) end })
hl.gesture({ fingers = 3, direction = "down",  action = function() hl.dispatch(hl.dsp.exec_cmd("hyprd ws " .. last)) end })

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ launchers                                                                     │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

super("P",     "App Launcher",       "pkill -x rofi || rofi -show drun")
super("Space", "Drop-down terminal", hl.dsp.workspace.toggle_special("dropdown"))

hl.define_submap("picker", function()
	bind("Left",   "Picker layout previous",  "hyprd picker left")
	bind("H",      "Picker layout previous",  "hyprd picker left")
	bind("Right",  "Picker layout next",      "hyprd picker right")
	bind("L",      "Picker layout next",      "hyprd picker right")
	bind("Up",     "Picker workspace up",     "hyprd picker up")
	bind("K",      "Picker workspace up",     "hyprd picker up")
	bind("Down",   "Picker workspace down",   "hyprd picker down")
	bind("J",      "Picker workspace down",   "hyprd picker down")
	bind("Return", "Picker confirm",          "hyprd picker confirm")
	bind("Escape", "Picker close",            "hyprd picker close")
	hl.bind("catchall", hl.dsp.no_op())
end)

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │                                     media                                     │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

local player = "playerctl --player=spotify"
local audio = "wpctl"
local backlight = require("machine").backlight == "yes"
local terminal = "kitty --title terminalfloat -e"

local function display(vcp, percent)
	if backlight then
		return "brightnessctl --class=backlight set " .. percent
	end
	return [[sh -c 'for c in /sys/class/drm/card*-*; do read -r s < "$c/status" && [ "$s" = connected ] && b=$(readlink "$c/ddc") && { ddcutil --bus "${b##*-}" --noverify setvcp 10 ]]
		.. vcp .. [[ & }; done; wait']]
end

local function locked(keys, command)
	hl.bind(keys, hl.dsp.exec_cmd(command), { locked = true, description = describe(), device = device })
end

locked("XF86AudioPlay",                  player .. " play-pause")
locked("XF86AudioPause",                 player .. " play-pause")
locked("XF86AudioStop",                  player .. " stop")
locked("XF86AudioNext",                  player .. " next")
locked("XF86AudioPrev",                  player .. " previous")
locked("XF86AudioMute",                  audio .. " set-mute @DEFAULT_AUDIO_SINK@ toggle")
locked("SUPER + XF86AudioMute",          audio .. " set-mute @DEFAULT_AUDIO_SOURCE@ toggle")
locked("XF86AudioRaiseVolume",           audio .. " set-volume -l 1.5 @DEFAULT_AUDIO_SINK@ 5%+")
locked("XF86AudioLowerVolume",           audio .. " set-volume @DEFAULT_AUDIO_SINK@ 5%-")
locked("SUPER + XF86AudioRaiseVolume",   player .. " volume 0.05+")
locked("SUPER + XF86AudioLowerVolume",   player .. " volume 0.05-")
locked("SUPER + XF86MonBrightnessUp",    display("85", "85%"))
locked("XF86MonBrightnessUp",            display("+ 5", "5%+"))
locked("XF86MonBrightnessDown",          display("- 5", "5%-"))
locked("SUPER + XF86MonBrightnessDown",  display("15", "15%"))

bind("XF86Calculator", nil, terminal .. " calc")
bind("XF86Explorer",   nil, terminal .. [[ zsh -c 'cd "$(xplr --print-pwd-as-result)" 2>/dev/null; exec zsh -l']])

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ screenshare                                                                   │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

bind("Print",  "Screenshot to clipboard", "hyprd screenshot")
super("Print", "Screenshot + annotate",   "hyprd screenshot annotate")

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ lock                                                                          │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

super("Q",         "Lock screen",    "hyprd lock full")
super("SHIFT + Q", "Wake displays", function()
	hl.dispatch(hl.dsp.dpms({ action = "off" }))
	hl.timer(function()
		hl.dispatch(hl.dsp.dpms({ action = "on" }))
	end, { timeout = 1000, type = "oneshot" })
end)

hl.define_submap("lockbarrier", function()
	hl.bind("catchall", hl.dsp.no_op())
end)

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ lid                                                                           │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

local panel = "eDP-1"

hl.bind("switch:on:Lid Switch", function()
	for _, monitor in ipairs(hl.get_monitors()) do
		if monitor.name ~= panel then
			hl.monitor({ output = panel, disabled = true })
			return
		end
	end
end, { locked = true })
hl.bind("switch:off:Lid Switch", function()
	hl.monitor({ output = panel, disabled = false })
end, { locked = true })
hl.on("monitor.removed", function(removed)
	if removed.name == panel then
		return
	end
	for _, monitor in ipairs(hl.get_monitors()) do
		if monitor.name ~= removed.name and monitor.name ~= "FALLBACK" then
			return
		end
	end
	hl.monitor({ output = panel, disabled = false })
end)

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ svalboard                                                                     │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

keyboard(svalboard, function()
	super("S", "Workspace 1 (chat)",     "hyprd ws 1")
	super("E", "Workspace 2 (misc)",     "hyprd ws 2")
	super("T", "Workspace 3 (work)",     "hyprd ws 3")
	super("C", "Workspace 4 (personal)", "hyprd ws 4")
	super("D", "Workspace 5 (dotfiles)", "hyprd ws 5")
	super("G", "Workspace 6 (music)",    "hyprd ws 6")

	super("equal",     "Focus right", "hyprd cycle right")
	super("backslash", "Focus left",  "hyprd cycle left")
	super("minus",     "Focus left",  "hyprd cycle left")
	super("slash",     "Focus right", "hyprd cycle right")

	super("Backspace",     "Toggle shadow",  "hyprd shadow")
	super("M",             "Swap master",    "hyprd swap")
	super("B",             "Toggle monocle", "hyprd monocle")
	super("K",             "Split narrow",   "hyprd split narrow")
	super("U",             "Split wide",     "hyprd split wide")

	for _, key in ipairs({ "A", "S", "E", "T", "G" }) do
		bind("CTRL + SHIFT + " .. key, nil, function()
			local window = hl.get_active_window()
			if window and window.class:find("^kitty") then
				return
			end
			hl.dispatch(hl.dsp.send_shortcut({ mods = "CTRL SHIFT", key = key:lower() }))
		end)
	end

	super("SHIFT + X", "Force kill window", "hyprctl kill")
	super("J",         "Layout Launcher",   "hyprd picker open")
	super("H",         "Keymap viewer", function()
		for _, window in ipairs(hl.get_windows()) do
			if window.class == "chrome-127.0.0.1__-Default" then
				hl.dispatch(hl.dsp.window.close({ window = "address:" .. window.address }))
				return
			end
		end
		hl.dispatch(hl.dsp.exec_cmd("chromium --app=http://127.0.0.1:42070/"))
	end)
end)

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ corne                                                                         │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

keyboard(corne, function()
	super("A", "Workspace 1 (chat)",     "hyprd ws 1")
	super("S", "Workspace 2 (misc)",     "hyprd ws 2")
	super("E", "Workspace 3 (work)",     "hyprd ws 3")
	super("T", "Workspace 4 (personal)", "hyprd ws 4")
	super("D", "Workspace 5 (dotfiles)", "hyprd ws 5")
	super("G", "Workspace 6 (music)",    "hyprd ws 6")

	alt("S", "Focus left",  "hyprd cycle left")
	alt("T", "Focus right", "hyprd cycle right")

	alt("Backspace",              "Toggle shadow",  "hyprd shadow")
	alt("Z",                      "Swap master",    "hyprd swap")
	alt("SHIFT + Z",              "Split wide",     "hyprd split wide")
	super("Comma",                "Split narrow",   "hyprd split narrow")
	bind("CTRL + SHIFT + Escape", "Toggle monocle", "hyprd monocle")

	super("K", "Force kill window", "hyprctl kill")
	super("H", "Layout Launcher",   "hyprd picker open")

	super("SHIFT + S", "Screenshot to clipboard", "hyprd screenshot")
	super("SHIFT + B", "Screenshot + annotate",   "hyprd screenshot annotate")
	super("SHIFT + P", "Screen share mode",       "hyprd share")

	locked("SUPER + SHIFT + H",      player .. " play-pause")
	locked("SUPER + SHIFT + Period", player .. " previous")
	locked("SUPER + SHIFT + Comma",  player .. " next")
end)
-- stylua: ignore end
