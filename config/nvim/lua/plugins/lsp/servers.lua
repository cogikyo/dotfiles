local keymaps = require("config.keymaps")

local servers = {
	bashls = {},
	gopls = {
		settings = {
			gopls = {
				-- Use gofumpt for stricter formatting (superset of gofmt)
				gofumpt = true,

				-- Clickable actions shown above functions/types in the editor
				codelenses = {
					gc_details = false, -- Show garbage collector optimization decisions (noisy)
					generate = true, -- Run `go generate` for //go:generate directives
					regenerate_cgo = true, -- Regenerate cgo definitions
					run_govulncheck = true, -- Scan dependencies for known vulnerabilities
					test = true, -- Run tests for this package
					tidy = true, -- Run `go mod tidy` to clean up go.mod
					upgrade_dependency = true, -- Upgrade a dependency to latest version
					vendor = true, -- Run `go mod vendor` to vendor dependencies
				},

				-- Inlay hints: inline type/value annotations shown in the editor
				hints = {
					assignVariableTypes = true, -- Show types for `x := foo()` → `x int := foo()`
					compositeLiteralFields = true, -- Show field names in struct literals
					compositeLiteralTypes = true, -- Show types in composite literals
					constantValues = true, -- Show computed values of constants
					functionTypeParameters = true, -- Show type params in generic function calls
					parameterNames = true, -- Show parameter names at call sites
					rangeVariableTypes = true, -- Show types for range loop variables
				},

				-- Static analyzers that run on your code
				analyses = {
					nilness = true, -- Detect nil pointer dereferences
					unusedparams = true, -- Warn about unused function parameters
					unusedwrite = true, -- Detect writes to variables that are never read
					useany = true, -- Suggest `any` instead of `interface{}`
					shadow = false, -- Variable shadowing (disabled: too noisy for `err`)
					ST1003 = false, -- Naming conventions (disabled: allow SCREAMING_SNAKE_CASE)
				},

				-- Enable staticcheck: additional linters beyond go vet
				staticcheck = true,

				-- Directories to exclude from workspace scanning (prefix `-` to exclude)
				directoryFilters = { "-.git", "-.vscode", "-.idea", "-.venv", "-node_modules" },

				-- Enhanced syntax highlighting using LSP semantic tokens
				semanticTokens = true,

				-- Insert placeholders for function parameters in completions
				usePlaceholders = true,

				-- Auto-import: suggest completions from packages not yet imported
				completeUnimported = true,

				-- Postfix completions: `err.ifnotnil` → `if err != nil { }`
				experimentalPostfixCompletions = true,
			},
		},
	},
	ts_ls = {
		settings = {
			typescript = {
				inlayHints = {
					includeInlayParameterNameHints = "all",
					includeInlayParameterNameHintsWhenArgumentMatchesName = false,
					includeInlayFunctionParameterTypeHints = true,
					includeInlayVariableTypeHints = true,
					includeInlayVariableTypeHintsWhenTypeMatchesName = false,
					includeInlayPropertyDeclarationTypeHints = true,
					includeInlayFunctionLikeReturnTypeHints = true,
					includeInlayEnumMemberValueHints = true,
				},
				suggest = { completeFunctionCalls = true },
			},
			javascript = {
				inlayHints = {
					includeInlayParameterNameHints = "all",
					includeInlayParameterNameHintsWhenArgumentMatchesName = false,
					includeInlayFunctionParameterTypeHints = true,
					includeInlayVariableTypeHints = true,
					includeInlayVariableTypeHintsWhenTypeMatchesName = false,
					includeInlayPropertyDeclarationTypeHints = true,
					includeInlayFunctionLikeReturnTypeHints = true,
					includeInlayEnumMemberValueHints = true,
				},
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
	basedpyright = {
		settings = {
			basedpyright = {
				typeCheckingMode = "standard",
				analysis = {
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

return {
	{
		"williamboman/mason.nvim",
		cmd = "Mason",
		build = ":MasonUpdate",
		opts = {},
	},
	{
		"williamboman/mason-lspconfig.nvim",
		opts = {
			ensure_installed = vim.tbl_keys(servers),
			handlers = {},
		},
	},
	{
		"neovim/nvim-lspconfig",
		event = { "BufReadPre", "BufNewFile" },
		dependencies = {
			"williamboman/mason.nvim",
			"williamboman/mason-lspconfig.nvim",
		},
		config = function()
			local capabilities = vim.lsp.protocol.make_client_capabilities()

			vim.api.nvim_create_autocmd("LspAttach", {
				group = vim.api.nvim_create_augroup("lsp-attach", { clear = true }),
				callback = function(event)
					keymaps.on_attach(event)
					local client = vim.lsp.get_client_by_id(event.data.client_id)
					if client and client.name == "ts_ls" then
						keymaps.ts_actions(event)
					end
				end,
			})

			vim.diagnostic.config({
				severity_sort = true,
				float = { border = "rounded", source = true },
				underline = { severity = vim.diagnostic.severity.ERROR },
				signs = {
					text = {
						[vim.diagnostic.severity.ERROR] = "󰅚 ",
						[vim.diagnostic.severity.WARN] = "󰀪 ",
						[vim.diagnostic.severity.INFO] = "󰋽 ",
						[vim.diagnostic.severity.HINT] = "󰌶 ",
					},
				},
				virtual_text = {
					source = false,
					spacing = 2,
					severity = { min = vim.diagnostic.severity.HINT },
				},
			})

			local hover_method = vim.lsp.protocol.Methods.textDocument_hover
			local hover_border = "rounded"

			vim.lsp.handlers[hover_method] = vim.lsp.with(vim.lsp.handlers.hover, { border = hover_border })

			local open_floating_preview = vim.lsp.util.open_floating_preview
			vim.lsp.util.open_floating_preview = function(contents, syntax, opts, ...)
				opts = opts or {}
				if opts.focus_id == hover_method then
					local padded = { "" }
					for _, line in ipairs(contents) do
						padded[#padded + 1] = " " .. line .. " "
					end
					padded[#padded + 1] = ""
					contents = padded
					opts.border = opts.border or hover_border
				end
				return open_floating_preview(contents, syntax, opts, ...)
			end

			for name, config in pairs(servers) do
				config.capabilities = vim.tbl_deep_extend("force", {}, capabilities, config.capabilities or {})
				vim.lsp.config(name, config)
				vim.lsp.enable(name)
			end

			vim.lsp.enable("stylua", false)
		end,
	},
}
