-- LSP server definitions (data-only module)

local ts_inlay_hints = {
	includeInlayParameterNameHints = "all",
	includeInlayParameterNameHintsWhenArgumentMatchesName = false,
	includeInlayFunctionParameterTypeHints = true,
	includeInlayVariableTypeHints = true,
	includeInlayVariableTypeHintsWhenTypeMatchesName = false,
	includeInlayPropertyDeclarationTypeHints = true,
	includeInlayFunctionLikeReturnTypeHints = true,
	includeInlayEnumMemberValueHints = true,
}

return {
	bashls = {},
	gopls = {
		settings = {
			gopls = {
				gofumpt = true,
				codelenses = {
					gc_details = false,
					generate = true,
					regenerate_cgo = true,
					run_govulncheck = true,
					test = true,
					tidy = true,
					upgrade_dependency = true,
					vendor = true,
				},
				hints = {
					assignVariableTypes = true,
					compositeLiteralFields = true,
					compositeLiteralTypes = true,
					constantValues = true,
					functionTypeParameters = true,
					parameterNames = true,
					rangeVariableTypes = true,
				},
				analyses = {
					nilness = true,
					unusedparams = true,
					unusedwrite = true,
					useany = true,
					shadow = false,
					ST1003 = false,
				},
				staticcheck = true,
				directoryFilters = { "-.git", "-.vscode", "-.idea", "-.venv", "-node_modules" },
				semanticTokens = false,
				usePlaceholders = true,
				completeUnimported = true,
				experimentalPostfixCompletions = true,
			},
		},
	},
	ts_ls = {
		settings = {
			typescript = {
				inlayHints = ts_inlay_hints,
				suggest = { completeFunctionCalls = true },
			},
			javascript = {
				inlayHints = ts_inlay_hints,
				suggest = { completeFunctionCalls = true },
			},
		},
	},
	eslint = {
		settings = { workingDirectories = { mode = "auto" } },
	},
	tailwindcss = {
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
	},
	cssls = {},
	html = { filetypes = { "html", "templ" } },
	jsonls = {
		settings = { json = { validate = { enable = true } } },
	},
	dockerls = {},
	docker_compose_language_service = {},
	emmet_ls = {
		filetypes = { "html", "css", "javascript", "javascriptreact", "typescript", "typescriptreact", "templ" },
	},
	templ = {},
	marksman = {},
	pyright = {
		settings = {
			python = {
				analysis = {
					typeCheckingMode = "standard",
					autoSearchPaths = true,
					useLibraryCodeForTypes = true,
					diagnosticSeverityOverrides = {
						reportUnknownMemberType = "none",
						reportUnknownArgumentType = "none",
						reportUnknownVariableType = "none",
						reportMissingTypeStubs = "none",
					},
				},
			},
		},
	},
	ruff = {},
	taplo = {},
	lua_ls = {
		filetypes = { "lua" },
		root_markers = { ".luarc.json", ".luarc.jsonc", ".stylua.toml", "stylua.toml", ".git" },
		settings = {
			Lua = {
				workspace = { checkThirdParty = false },
				telemetry = { enable = false },
				diagnostics = { globals = { "vim" } },
				format = { enable = false },
			},
		},
	},
}
