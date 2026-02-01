local function meta(icon, color, mods)
	return {
		style = {
			fg = color,
			add_modifiers = mods or nil,
		},
		meta = { icon = icon },
	}
end

local node_types = {
	directory = meta(" ", "Blue"),
	file = meta(" ", "White"),
	symlink = meta(" ", "Cyan"),
	mime_essence = {
		audio = {
			["*"] = meta(" ", "Green"),
		},
		video = {
			["*"] = meta(" ", "Magenta"),
		},
		image = {
			["*"] = meta(" ", "Green"),
		},
		application = {
			["*"] = meta("󰶭 ", "Yellow"),
		},
		text = {
			["*"] = meta(" ", "White"),
		},
	},
	extension = {
		md = meta(" ", "White", { "Dim" }),
		toml = meta(" "),
		conf = meta(" "),
		json = meta(" "),
		list = meta(" "),
		lst = meta(" "),
		dirs = meta(" "),
		gz = meta(" ", "White"),
		zip = meta(" ", "White"),
		desktop = meta("󱕷 "),
		rules = meta(" ", "Red", { "Dim" }),
		lua = meta(" "),
		rs = meta("󱘗 "),
		py = meta(" "),
		scss = meta("󰟬 "),
		css = meta(" "),
		html = meta(" "),
	},
	special = {
		downloads = meta(" "),
		dotfiles = meta("🍙"),
		docs = meta(" "),
		books = meta(" "),
		cmd = meta(" "),
		templates = meta(" "),
		media = meta("󰈯 "),
		share = meta(" "),
		music = meta(" "),
		gifs = meta("󰤺 "),
		screenshots = meta(" "),
		images = meta("󰋯 "),
		videos = meta(" "),
		recordings = meta("󰕧 "),
		etc = meta("󱁽 "),
		bin = meta("⼡"),
		usr = meta("⼈"),
		home = meta("⾕", "Yellow"),
		cullyn = meta("⾕"),
		config = meta(" "),
		LICENSE = meta(" ", "DarkGray"),
	},
}

for key, val in pairs(node_types) do
	xplr.config.node_types[key] = val
end
