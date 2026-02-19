return {
	filetypes = { "html", "css", "javascript", "javascriptreact", "typescript", "typescriptreact", "templ" },
	init_options = { userLanguages = { templ = "html" } },
	settings = {
		tailwindCSS = {
			includeLanguages = { templ = "html" },
			experimental = {
				classRegex = {
					{ "cva\\(([^)]*)\\)", "[\"'`]([^\"'`]*).*?[\"'`]" },
					{ "cx\\(([^)]*)\\)", "(?:'|\"|`)([^']*)(?:'|\"|`)" },
					{ "cn\\(([^)]*)\\)", "(?:'|\"|`)([^']*)(?:'|\"|`)" },
					{ "clsx\\(([^)]*)\\)", "(?:'|\"|`)([^']*)(?:'|\"|`)" },
				},
			},
		},
	},
}
