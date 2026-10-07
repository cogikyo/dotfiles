-- stylua: ignore start
local function bind(keys, description, action)
	if type(action) == "string" then
		action = hl.dsp.exec_cmd(action)
	end

	hl.bind(keys, action, { description = description })
end

local function super(keys, description, action)
	bind("SUPER + " .. keys, description, action)
end

local function alt(keys, description, action)
	bind("ALT + " .. keys, description, action)
end

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ hyprd focus control                                                           │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

-- ├┤ move to workspace ├──────────────────────────────────────────────────────────┤
super("B", "Workspace 1 (misc)",     "hyprd ws 1")
super("C", "Workspace 2 (chat)",     "hyprd ws 2")
super("D", "Workspace 3 (work)",     "hyprd ws 3")
super("H", "Workspace 4 (personal)", "hyprd ws 4")
super("M", "Workspace 5 (dotfiles)", "hyprd ws 5")
super("K", "Workspace 6 (music)",    "hyprd ws 6")

super("equal",     "Focus right", hl.dsp.focus({ direction = "right" }))
super("backslash", "Focus left",  hl.dsp.focus({ direction = "left" }))

-- ├┤ threebody layout ├───────────────────────────────────────────────────────────┤
alt("A", "Editor",  "hyprd three-body editor")
super("R", "Browser", "hyprd three-body browser")
alt("C", "Agents",  "hyprd three-body agents")
alt("X", "Dismiss", "dunstctl close")

super("Backspace",     "Toggle shadow",  "hyprd three-body shadow")
super("Period",        "Swap master",    "hyprd swap")
super("Return",        "Toggle monocle", "hyprd monocle")
super("Comma",         "Split narrow",   "hyprd split narrow")
super("SHIFT + Comma", "Split wide",     "hyprd split wide")

-- ├┤ editor tab focus ├───────────────────────────────────────────────────────────┤
super("A", "Editor tab 0", "hyprd tab editor:0")
super("S", "Editor tab 1", "hyprd tab editor:1")
super("E", "Editor tab 2", "hyprd tab editor:2")
super("T", "Editor tab 3", "hyprd tab editor:3")
super("G", "Editor tab 4", "hyprd tab editor:4")

-- ├┤ agents tab focus ├───────────────────────────────────────────────────────────┤
super("Y", "Agents tab 0", "hyprd tab agents:0")
super("N", "Agents tab 1", "hyprd tab agents:1")
super("I", "Agents tab 2", "hyprd tab agents:2")
super("O", "Agents tab 3", "hyprd tab agents:3")
super("L", "Agents tab 4", "hyprd tab agents:4")

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ window management                                                             │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

-- ├┤ core conrols ├───────────────────────────────────────────────────────────────┤
super("X", "Close active window", hl.dsp.window.close())
super("SHIFT + X", "Force kill window", "hyprctl kill")
super("F", "Toggle floating",    "hyprd float")
super("V", "Screen share mode",   "hyprd share")
super("F11", "Toggle full screen",  hl.dsp.window.fullscreen({ mode = "fullscreen", action = "toggle" }))

hl.bind("SUPER + mouse:273", hl.dsp.window.drag(),   { mouse = true })
hl.bind("SUPER + mouse:274", hl.dsp.window.resize(), { mouse = true })

-- ├┤ move window focus ├──────────────────────────────────────────────────────────┤
super("minus",     "Focus left",  hl.dsp.focus({ direction = "left" }))
super("slash",     "Focus right", hl.dsp.focus({ direction = "right" }))

-- ├┤ move windows ├───────────────────────────────────────────────────────────────┤
super("Left",  "Move window left",    hl.dsp.window.move({ direction = "left" }))
super("Right", "Move window right",   hl.dsp.window.move({ direction = "right" }))
super("Up",    "Move window up",      hl.dsp.window.move({ direction = "up" }))
super("Down",  "Move window down",    hl.dsp.window.move({ direction = "down" }))
super("Home",  "Move workspace down", "hyprd ws down")
super("End",   "Move workspace up",   "hyprd ws up")

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ launchers                                                                     │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

super("P", "App Launcher",    "hyprlauncher")
super("J", "Layout Launcher", "hyprd picker open")
super("U", "Keymap viewer", function()
	for _, window in ipairs(hl.get_windows()) do
		if window.class == "chrome-127.0.0.1__-Default" then
			hl.dispatch(hl.dsp.window.close({ window = "address:" .. window.address }))
			return
		end
	end
	hl.dispatch(hl.dsp.exec_cmd("chromium --app=http://127.0.0.1:42070/"))
end)

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
local display = "ddcutil --bus 8 --noverify"
local terminal = "kitty --title terminalfloat -e"

local function locked(keys, command)
	hl.bind(keys, hl.dsp.exec_cmd(command), { locked = true })
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
locked("SUPER + XF86MonBrightnessUp",    display .. " setvcp 10 85")
locked("XF86MonBrightnessUp",            display .. " setvcp 10 + 5")
locked("XF86MonBrightnessDown",          display .. " setvcp 10 - 5")
locked("SUPER + XF86MonBrightnessDown",  display .. " setvcp 10 15")

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

super("Z",         "Lock screen",    "hyprd lock full")
super("SHIFT + Z", "Wake displays", function()
	hl.dispatch(hl.dsp.dpms({ action = "off" }))
	hl.timer(function()
		hl.dispatch(hl.dsp.dpms({ action = "on" }))
	end, { timeout = 1000, type = "oneshot" })
end)

hl.define_submap("lockbarrier", function()
	hl.bind("catchall", hl.dsp.no_op())
end)
-- stylua: ignore end
