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
hl.env("QT_QPA_PLATFORMTHEME", "hyprqt6engine")
hl.env("QT_WAYLAND_DISABLE_WINDOWDECORATION", "1")
hl.env("QT_SCALE_FACTOR", "1.25")
hl.env("QT_STYLE_OVERRIDE", "kvantum")

hl.env("GDK_BACKEND", "wayland,x11")
hl.env("GDK_DPI_SCALE", "1.25")
hl.env("SDL_VIDEODRIVER", "wayland")
hl.env("PATH", home .. "/.local/bin:" .. path)

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │                                   autostart                                   │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

hl.on("hyprland.start", function()
	hl.exec_cmd([[sh -c 'taskset -cp 0-6,8-14,16-1023 "$PPID" >/dev/null']])
	for _, command in ipairs({
		"taskset -c 7,15 dunst",
		"hyprpolkitagent",
		"hypridle",
		"hyprpaper",
	}) do
		hl.exec_cmd(command)
	end
	hl.exec_cmd("hyprd init") -- imports env, starts daemons, runs boot sequence

	-- GTK theming via gsettings — Wayland reads dconf, not settings.ini.
	local gnome = "org.gnome.desktop.interface"
	hl.exec_cmd("gsettings set " .. gnome .. " gtk-theme 'catppuccin-macchiato-peach-standard+default'")
	hl.exec_cmd("gsettings set " .. gnome .. " color-scheme 'prefer-dark'")
	hl.exec_cmd("gsettings set " .. gnome .. " icon-theme 'Papirus-Dark'")
	hl.exec_cmd("gsettings set " .. gnome .. " font-name 'Albert Sans 12'")
	hl.exec_cmd("gsettings set " .. gnome .. " document-font-name 'Lora 12'")
	hl.exec_cmd("gsettings set " .. gnome .. " monospace-font-name 'Vagari 12'")
	hl.exec_cmd("gsettings set " .. gnome .. " cursor-theme 'catppuccin-macchiato-light-cursors'")
	hl.exec_cmd("gsettings set " .. gnome .. " cursor-size 26")
end)

hl.monitor({
	output = "",
	mode = "preferred",
	position = "auto",
	scale = 1,
})

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │                                 window rules                                  │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

local dialogSize = "1660 980"
local floatSize = "2882 1864"

hl.window_rule({
	name = "spotify",
	match = { class = [[^([Ss]potify)$]] },
	workspace = "1 silent",
	opacity = "0.75 override 0.75 override",
})
hl.window_rule({ name = "discord-workspace", match = { class = "discord" }, workspace = "2 silent" })
hl.window_rule({ name = "slack-workspace", match = { class = "slack" }, workspace = "2 silent" })

-- Keep document placement broad while applying popup geometry only to Zathura.
hl.window_rule({ name = "libreoffice-workspace", match = { class = [[^libreoffice-.+$]] }, workspace = "3 silent" })
hl.window_rule({
	name = "zathura",
	match = { class = [[^(org\.pwmt\.zathura)$]] },
	workspace = "3 silent",
	float = true,
	size = "1000 1475",
	center = true,
})

hl.window_rule({
	name = "media-viewer",
	match = { class = [[^(imv|mpv)$]] },
	float = true,
	center = true,
})
hl.window_rule({ name = "zoom-title", match = { title = "zoom" }, float = true })

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

-- GLava — desktop audio visualizer, fully passthrough.
hl.window_rule({
	name = "glava",
	match = { title = [[^(GLava)$]] },
	border_size = 0,
	no_focus = true,
	no_shadow = true,
	float = true,
	pin = true,
	no_blur = true,
})

hl.window_rule({
	name = "dialog",
	match = { title = [[^(File|Settings|Authentication|Save As)]] },
	float = true,
	size = dialogSize,
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
	move = "479 173",
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

hl.workspace_rule({ workspace = "special:stash", gaps_out = 120 })

require("binds")

-- ╭───────────────────────────────────────────────────────────────────────────────╮
-- │                                    layout                                     │
-- ╰───────────────────────────────────────────────────────────────────────────────╯

hl.config({
	general = {
		border_size = 2,
		gaps_in = 10,
		gaps_out = { top = 85, right = 86, bottom = 30, left = 126 }, -- top, right, bottom, left — room for eww bar
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
	},
	cursor = {
		no_hardware_cursors = true,
	},
	master = {
		allow_small_split = true,
		special_scale_factor = 1,
		mfact = 0.4934,
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
})

hl.animation({ leaf = "windows", enabled = true, speed = 2, bezier = "default", style = "popin" })
hl.animation({ leaf = "windowsOut", enabled = true, speed = 2, bezier = "default" })
hl.animation({ leaf = "windowsMove", enabled = true, speed = 1.5, bezier = "default", style = "slidefade" })
hl.animation({ leaf = "workspaces", enabled = true, speed = 3, bezier = "default", style = "slide" })
hl.animation({ leaf = "fade", enabled = true, speed = 1.5, bezier = "default" })
