return {
	"3rd/image.nvim",
	ft = { "markdown", "norg", "oil" },
	opts = {
		backend = "kitty",
		processor = "magick_cli",
		integrations = {
			markdown = {
				enabled = true,
				clear_in_insert_mode = true,
				only_render_image_at_cursor = true,
			},
		},
		max_width = 100,
		max_height = 30,
		max_height_window_percentage = 50,
		max_width_window_percentage = 50,
		window_overlap_clear_enabled = true,
	},
}
