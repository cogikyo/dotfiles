xplr.config.layouts.builtin.default = {
	Horizontal = {
		config = {
			margin = 1,
			horizontal_margin = 2,
			constraints = { { Percentage = 100 } },
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Min = 10 },
							{ Max = 10 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						"Selection",
						"InputAndLogs",
					},
				},
			},
		},
	},
}

xplr.config.layouts.builtin.no_selection = {
	Horizontal = {
		config = {
			margin = 1,
			horizontal_margin = 2,
			constraints = { { Percentage = 100 } },
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Min = 5 },
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
		},
	},
}

xplr.config.layouts.custom.selection = {
	Horizontal = {
		config = {
			margin = 1,
			horizontal_margin = 2,
			constraints = { { Percentage = 100 } },
		},
		splits = {
			{
				Vertical = {
					config = {
						constraints = {
							{ Length = 3 },
							{ Percentage = 50 },
							{ Percentage = 50 },
							{ Length = 3 },
						},
					},
					splits = {
						"SortAndFilter",
						"Table",
						"Selection",
						"InputAndLogs",
					},
				},
			},
		},
	},
}
