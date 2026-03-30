return {
	"catgoose/nvim-colorizer.lua",
	opts = {
		user_default_options = {
			css = true,
			tailwind = false,
			names = false,
			mode = "background",
		},
		filetypes = {
			"*",
			css = { tailwind = true },
			scss = { tailwind = true },
		},
	},
}
