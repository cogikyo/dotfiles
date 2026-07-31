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

alt("A", "Focus tree", "hyprd tab nvimtree")
alt("S", "Focus nvim", "hyprd tab nvim")
alt("E", "Terminal",   "hyprd tab term")
alt("T", "Build",      "hyprd tab build")
alt("G", "Focus git",  "hyprd tab git")

super("Y", "Focus scout", "hyprd tab agents:scout")
super("N", "Focus build", "hyprd tab agents:build")
super("I", "Focus drive", "hyprd tab agents:drive")
super("O", "Focus plan",  "hyprd tab agents:plan")
super("L", "Focus learn", "hyprd tab agents:learn")

alt("R", "Editor",  "hyprd three-body editor")
alt("D", "Browser", "hyprd three-body browser")
alt("C", "Agents",  "hyprd three-body agents")
alt("X", "Dismiss", "dunstctl close")

super("Backspace",  "Toggle shadow",  "hyprd three-body shadow")
super("Escape",     "Toggle shadow",  "hyprd three-body shadow")
super("apostrophe", "Toggle monocle", "hyprd monocle")
super("Comma",      "Cycle split",    "hyprd split")
super("Period",     "Cycle split",    "hyprd swap")

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │ window management                                                             │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

super("X", "Close active window", hl.dsp.window.close())
super("K", "Force kill window",  "hyprctl kill")
super("F", "Toggle floating",    "hyprd float")
super("V",   "Screen share mode",   "hyprd share")
super("F11", "Toggle full screen",  hl.dsp.window.fullscreen({ mode = "fullscreen", action = "toggle" }))

hl.bind("SUPER + mouse:273", hl.dsp.window.drag(),   { mouse = true })
hl.bind("SUPER + mouse:274", hl.dsp.window.resize(), { mouse = true })

-- ├┤ move to workspace ├──────────────────────────────────────────────────────────┤
super("g", "Workspace 1 (music)",    "hyprd ws 1")
super("s", "Workspace 2 (chat)",     "hyprd ws 2")
super("e", "Workspace 3 (misc)",     "hyprd ws 3")
super("t", "Workspace 4 (primary)",  "hyprd ws 4")
super("d", "Workspace 5 (settings)", "hyprd ws 5")

-- ├┤ move window focus ├──────────────────────────────────────────────────────────┤
super("R",         "Focus left",  hl.dsp.focus({ direction = "left" }))
super("minus",     "Focus left",  hl.dsp.focus({ direction = "left" }))
super("equal",     "Focus right", hl.dsp.focus({ direction = "right" }))
super("slash",     "Focus right", hl.dsp.focus({ direction = "right" }))
super("backslash", "Focus up",    hl.dsp.focus({ direction = "up" }))
super("C",         "Focus down",  hl.dsp.focus({ direction = "down" }))

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
super("H", "Layout Launcher", "hyprd picker open")

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
super("SHIFT + Z", "Wake displays",  [[sh -c 'hyprctl dispatch dpms off; sleep 1; hyprctl dispatch dpms on']])
super("Q",         "pseudo-lock",    "hyprd lock")

hl.define_submap("pseudolock", function()
	super("Q", nil, "hyprd lock unlock && " .. player .. " play")
	hl.bind("catchall", hl.dsp.no_op())
end)
-- stylua: ignore end
