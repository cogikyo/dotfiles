local keymaps = require("config.keymaps")

local servers = {
	bashls = {},
	gopls = {
		settings = {
			gopls = {
				gofumpt = true,
				codelenses = {
					gc_details = true,
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
					shadow = true,
				},
				staticcheck = true,
				directoryFilters = { "-.git", "-.vscode", "-.idea", "-.venv", "-node_modules" },
				semanticTokens = true,
				usePlaceholders = true,
				completeUnimported = true,
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
	pyright = {},
	taplo = {},
	lua_ls = {
		settings = {
			Lua = {
				workspace = { checkThirdParty = false },
				telemetry = { enable = false },
				diagnostics = { globals = { "vim" } },
				format = { enable = false },
				completion = { callSnippet = "Replace" },
			},
		},
	},
}

return {
	{
		"saghen/blink.cmp",
		version = "1.*",
		dependencies = { "rafamadriz/friendly-snippets" },
		opts = {
			keymap = { preset = "default" },
			appearance = { nerd_font_variant = "mono" },
			completion = {
				accept = { auto_brackets = { enabled = true } },
				documentation = {
					auto_show = true,
					auto_show_delay_ms = 100,
					window = { border = "rounded" },
				},
				menu = {
					border = "rounded",
					draw = {
						columns = {
							{ "kind_icon" },
							{ "label", "label_description", gap = 1 },
							{ "source_name" },
						},
					},
				},
			},
			signature = {
				enabled = true,
				window = { border = "rounded" },
			},
			sources = {
				default = { "lsp", "path", "snippets", "buffer" },
			},
			fuzzy = { implementation = "prefer_rust_with_warning" },
		},
	},
	{
		"neovim/nvim-lspconfig",
		event = { "BufReadPre", "BufNewFile" },
		dependencies = { "saghen/blink.cmp" },
		config = function()
			local capabilities = require("blink.cmp").get_lsp_capabilities()

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
				virtual_text = { source = true, spacing = 2 },
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
		end,
	},
}
