-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │                                  environment                                  │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

local home = assert(os.getenv("HOME"), "HOME is not set")
local path = assert(os.getenv("PATH"), "PATH is not set")

hl.env("HYPRCURSOR_THEME", "catppuccin-macchiato-light-cursors")
hl.env("HYPRCURSOR_SIZE", "26")
hl.env("XCURSOR_THEME", "catppuccin-macchiato-light-cursors")
hl.env("XCURSOR_SIZE", "26")
hl.env("AQ_NO_MODIFIERS", "1")

hl.env("XDG_CURRENT_DESKTOP", "Hyprland")
hl.env("XDG_SESSION_TYPE", "wayland")
hl.env("XDG_SESSION_DESKTOP", "Hyprland")

hl.env("QT_QPA_PLATFORM", "wayland;xcb")
hl.env("QT_QPA_PLATFORMTHEME", "qt6ct")
hl.env("QT_WAYLAND_DISABLE_WINDOWDECORATION", "1")
hl.env("QT_SCALE_FACTOR", "1.25")

hl.env("GDK_BACKEND", "wayland,x11")
hl.env("GDK_DPI_SCALE", "1.25")
hl.env("SDL_VIDEODRIVER", "wayland")
hl.env("PATH", home .. "/.local/bin:" .. path)

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │                                   autostart                                   │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

hl.on("hyprland.start", function()
	local lane = require("machine").lane or ""
	for _, command in ipairs({
		lane == "" and "dunst" or "taskset -c " .. lane .. " dunst",
		"hyprpaper",
	}) do
		hl.exec_cmd(command)
	end
	hl.exec_cmd("hyprd init") -- imports env, starts daemons, runs boot sequence

	-- GTK theming via gsettings — Wayland reads dconf, not settings.ini.
	local gnome = "org.gnome.desktop.interface"
	hl.exec_cmd("gsettings set " .. gnome .. " gtk-theme 'vagari'")
	hl.exec_cmd("gsettings set " .. gnome .. " color-scheme 'prefer-dark'")
	hl.exec_cmd("gsettings set " .. gnome .. " icon-theme 'Papirus-Dark'")
	hl.exec_cmd("gsettings set " .. gnome .. " font-name 'sans-serif 12'")
	hl.exec_cmd("gsettings set " .. gnome .. " document-font-name 'serif 12'")
	hl.exec_cmd("gsettings set " .. gnome .. " monospace-font-name 'Vagari 12'")
	hl.exec_cmd("gsettings set " .. gnome .. " cursor-theme 'catppuccin-macchiato-light-cursors'")
	hl.exec_cmd("gsettings set " .. gnome .. " cursor-size 26")
end)

hl.monitor({
	output = "",
	mode = "preferred",
	position = "auto",
	scale = 1.00,
})
hl.monitor({
	output = "eDP-1",
	mode = "preferred",
	position = "auto",
	scale = 1.00,
})

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │                                 window rules                                  │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

local gaps = { top = 85, right = 90, bottom = 27, left = 124 } -- clearance for full-layout eww widgets
local margin = { w = 2 * math.max(gaps.left, gaps.right), h = 2 * math.max(gaps.top, gaps.bottom) }

local function relative(w, h)
	return {
		("rint(min(monitor_w*%g,monitor_w-%d))"):format(w / 100, margin.w),
		("rint(min(monitor_h*%g,monitor_h-%d))"):format(h / 100, margin.h),
	}
end

local function fit(monitor, w, h)
	local r = monitor.reserved
	local pw, ph = monitor.width, monitor.height
	if monitor.transform % 2 == 1 then
		pw, ph = ph, pw
	end
	local area = {
		x = monitor.x + r.left,
		y = monitor.y + r.top,
		w = pw / monitor.scale - r.left - r.right,
		h = ph / monitor.scale - r.top - r.bottom,
	}
	local fw = math.floor(math.min(area.w * w / 100, area.w - margin.w) + 0.5)
	local fh = math.floor(math.min(area.h * h / 100, area.h - margin.h) + 0.5)
	return fw, fh, area
end

local dialogSize = relative(43.23, 45.37)
local floatSize = relative(75.1, 86.85)
local monocle = { "(monitor_w-" .. floatSize[1] .. ")/2", "(monitor_h-" .. floatSize[2] .. ")/2+25" }

hl.window_rule({
	name = "spotify",
	match = { class = [[^([Ss]potify)$]] },
	workspace = "6 silent",
	opacity = "0.75 override 0.75 override",
})
hl.window_rule({ name = "discord-workspace", match = { class = "discord" }, workspace = "1 silent" })
hl.window_rule({ name = "slack-workspace", match = { class = "slack" }, workspace = "1 silent" })
hl.window_rule({ name = "grok-bot-workspace", match = { class = "grok-bot" }, workspace = "1 silent" })

-- Keep document placement broad while applying popup geometry only to Zathura.
hl.window_rule({ name = "libreoffice-workspace", match = { class = [[^libreoffice-.+$]] }, workspace = "2 silent" })
hl.window_rule({
	name = "zathura",
	match = { class = [[^(org\.pwmt\.zathura)$]] },
	workspace = "2 silent",
	float = true,
	size = relative(26.04, 68.29),
	center = true,
})

hl.window_rule({
	name = "media-viewer",
	match = { class = [[^(imv|mpv)$]] },
	float = true,
	center = true,
})
hl.window_rule({
	name = "zoom-workspace",
	match = {
		class = [[^(zoom)$]],
		title = [[^(Zoom Workplace|Meeting)$]],
	},
	workspace = "1 silent",
})
hl.window_rule({
	name = "zoom-hidden-helper",
	match = {
		class = [[^(zoom)$]],
		title = [[^(annotate_toolbar|as_preview|as_toolbar|zoom)$]],
	},
	float = true,
	workspace = "special:zoom-hidden silent",
	no_focus = true,
	opacity = "0 override",
	no_shadow = true,
})

hl.window_rule({
	name = "thunar",
	match = { class = [[^(thunar)$]] },
	float = true,
	size = floatSize,
	center = true,
})

hl.window_rule({
	name = "firefox-opacity",
	match = { title = [[^(firefox-developer-edition)$]] },
	opacity = "1.0 override 1.0 override",
})
hl.window_rule({
	name = "firefox-dialog",
	match = {
		class = [[^(firefox-developer-edition)$]],
		title = [[^(Extension:|Sign in)]],
	},
	float = true,
})

local function isBitwarden(window)
	return window ~= nil
		and window.class == "firefox-developer-edition"
		and window.title:find("Extension: (Bitwarden Password Manager)", 1, true) == 1
end

local function floatBitwarden(window)
	local target = "address:" .. window.address
	local w, h = fit(window.monitor or hl.get_active_monitor(), 23.44, 50.93)
	hl.dispatch(hl.dsp.window.float({ action = "set", window = target }))
	hl.dispatch(hl.dsp.window.resize({ x = w, y = h, window = target }))
	hl.dispatch(hl.dsp.window.center({ window = target }))
end

hl.on("window.title", function(window)
	if isBitwarden(window) then
		hl.dispatch(hl.dsp.window.float({ action = "set", window = window }))
	end
end)

hl.on("window.fullscreen", function(window)
	if isBitwarden(window) and window.fullscreen == 0 and window.fullscreen_client == 0 then
		floatBitwarden(window)
	end
end)

-- GLava — desktop audio visualizer, fully passthrough.
hl.window_rule({
	name = "glava",
	match = { title = [[GLava( lock)?]] },
	border_size = 0,
	no_focus = true,
	no_shadow = true,
	float = true,
	pin = true,
	no_blur = true,
})

hl.window_rule({
	name = "glava-lock",
	match = { tag = "lock", title = "GLava" },
	opacity = "0.01 override 0.01 override",
})

hl.window_rule({
	name = "dialog",
	match = { title = [[^(File|Settings|Authentication|Save As)]] },
	float = true,
	size = dialogSize,
	center = true,
})
hl.window_rule({
	name = "polkit",
	match = { class = [[^(hyprpolkitagent)$]] },
	float = true,
	pin = true,
	center = true,
})
hl.window_rule({
	name = "portal-dialog",
	match = { class = [[^(xdg-desktop-portal-gtk)$]] },
	float = true,
	size = dialogSize,
	center = true,
})
hl.window_rule({
	name = "share-picker",
	match = { class = [[^(hyprland-share-picker)$]] },
	float = true,
	center = true,
})
local dropdown = { w = 50, h = 50.93, top = -20 }
local dropdownSize = relative(dropdown.w, dropdown.h)

hl.window_rule({
	name = "drop-down-terminal",
	match = { class = [[^(dropdown)$]] },
	workspace = "special:dropdown",
	float = true,
	size = dropdownSize,
	move = { "(monitor_w-" .. dropdownSize[1] .. ")/2", tostring(dropdown.top) },
	rounding = 20,
	animation = "slide top",
})

hl.on("workspace.special_active", function(ws)
	if not ws or ws.name ~= "special:dropdown" then
		return
	end
	local w, h, area = fit(ws.monitor or hl.get_active_monitor(), dropdown.w, dropdown.h)
	local x = math.floor(area.x + (area.w - w) / 2)
	for _, window in ipairs(hl.get_workspace_windows("special:dropdown")) do
		local target = "address:" .. window.address
		hl.dispatch(hl.dsp.window.resize({ x = w, y = h, window = target }))
		hl.dispatch(hl.dsp.window.move({ x = x, y = area.y + dropdown.top, window = target }))
	end
end)
hl.window_rule({
	name = "terminal-float",
	match = { title = [[^(terminalfloat)]] },
	float = true,
	size = floatSize,
	center = true,
})

-- Satty — screenshot annotation popup, float centered at monocle size.
hl.window_rule({
	name = "satty",
	match = { class = [[^(com\.gabm\.satty)$]] },
	float = true,
	size = floatSize,
	move = monocle,
})
hl.window_rule({
	name = "keymap",
	match = { class = [[^(chrome-127\.0\.0\.1__-Default)$]] },
	float = true,
	size = floatSize,
	move = monocle,
})

hl.window_rule({
	name = "guvcview",
	match = { title = "guvcview" },
	float = true,
	-- The Lua rule API caps per-window rounding at 20.
	rounding = 20,
	opacity = "0.8",
	no_focus = true,
})

hl.layer_rule({ match = { namespace = [[^(hyprpaper)$]] }, order = 1 })
hl.layer_rule({ match = { namespace = [[^(mpvpaper)$]] }, order = 0 })
hl.layer_rule({
	name = "launcher",
	match = { namespace = [[^(rofi)$]] },
	animation = "slide bottom",
	blur = true,
	ignore_alpha = 0.5,
	dim_around = true,
})

hl.workspace_rule({ workspace = "special:stash", gaps_out = 120 })
hl.workspace_rule({ workspace = "special:dropdown", on_created_empty = "kitty --class dropdown -o tab_bar_min_tabs=2 -o background_opacity=0.92 -o 'window_padding_width=24 9 9 9'" })

require("binds")

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │                                    layout                                     │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

hl.config({
	general = {
		border_size = 2,
		gaps_in = 10,
		gaps_out = gaps,
		col = {
			active_border = "rgb(f2a170)",
			inactive_border = "rgb(7492ef)",
		},
		layout = "master",
		resize_on_border = false,
	},
	decoration = {
		rounding = 4,
		active_opacity = 1,
		inactive_opacity = 0.98,
		fullscreen_opacity = 1,
		dim_special = 0.4,
		shadow = {
			enabled = true,
			range = 24,
			render_power = 4,
			color = "rgba(e56b2c08)",
			color_inactive = "rgba(4a6be305)",
			offset = { 0, 0 },
		},
	},
	animations = {
		enabled = true,
	},
	input = {
		repeat_rate = 40,
		repeat_delay = 500,
		follow_mouse = 1,
		sensitivity = 1,
		float_switch_override_focus = 0,
		touchpad = {
			clickfinger_behavior = true,
		},
	},
	cursor = {
		no_hardware_cursors = true,
	},
	master = {
		allow_small_split = true,
		special_scale_factor = 1,
		mfact = 0.4749,
		new_status = "slave",
		new_on_top = false,
		new_on_active = "none",
		orientation = "left",
		smart_resizing = false,
		drop_at_cursor = true,
		always_keep_position = false,
	},
	dwindle = {
		force_split = 0,
		preserve_split = false,
		smart_split = false,
		smart_resizing = true,
		permanent_direction_override = false,
		special_scale_factor = 1,
		split_width_multiplier = 1.0,
		use_active_for_splits = true,
		default_split_ratio = 1.0,
		split_bias = 0,
		precise_mouse_move = false,
	},
	misc = {
		allow_session_lock_restore = true,
		session_lock_xray = true,
	},
})

hl.device({ name = "ven_04f3:00-04f3:32aa-touchpad", sensitivity = 0.69 })

hl.animation({ leaf = "windows", enabled = true, speed = 2, bezier = "default", style = "popin" })
hl.animation({ leaf = "windowsOut", enabled = true, speed = 2, bezier = "default" })
hl.animation({ leaf = "windowsMove", enabled = true, speed = 1.5, bezier = "default", style = "slidefade" })
hl.animation({ leaf = "workspaces", enabled = true, speed = 3, bezier = "default", style = "slide" })
hl.animation({ leaf = "fade", enabled = true, speed = 1.5, bezier = "default" })

hl.curve("easeOutQuint", { type = "bezier", points = { { 0.23, 1 }, { 0.32, 1 } } })
hl.curve("easeInCubic", { type = "bezier", points = { { 0.32, 0 }, { 0.67, 0 } } })
hl.animation({ leaf = "layersIn", enabled = true, speed = 2.5, bezier = "easeOutQuint" })
hl.animation({ leaf = "layersOut", enabled = true, speed = 2.2, bezier = "easeInCubic" })
hl.animation({ leaf = "fadeLayersOut", enabled = true, speed = 2.6, bezier = "easeInCubic" })
hl.animation({ leaf = "specialWorkspaceIn", enabled = true, speed = 3, bezier = "easeOutQuint", style = "slide top" })
hl.animation({ leaf = "specialWorkspaceOut", enabled = true, speed = 2.2, bezier = "easeInCubic", style = "slide bottom" })
