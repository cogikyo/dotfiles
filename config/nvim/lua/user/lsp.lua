vim.api.nvim_create_autocmd("LspAttach", {
	group = vim.api.nvim_create_augroup("lsp-attach", { clear = true }),
	callback = function(event)
		local map = function(keys, func, desc, mode)
			mode = mode or "n"
			vim.keymap.set(mode, keys, func, { buffer = event.buf, desc = "LSP: " .. desc })
		end

		map("grn", vim.lsp.buf.rename, "[R]e[n]ame")
		map("gra", vim.lsp.buf.code_action, "[G]oto Code [A]ction", { "n", "x" })
		map("grr", require("telescope.builtin").lsp_references, "[G]oto [R]eferences")
		map("gri", require("telescope.builtin").lsp_implementations, "[G]oto [I]mplementation")
		map("grd", require("telescope.builtin").lsp_definitions, "[G]oto [D]efinition")
		map("grD", vim.lsp.buf.declaration, "[G]oto [D]eclaration")
		map("gO", require("telescope.builtin").lsp_document_symbols, "Document Symbols")
		map("gW", require("telescope.builtin").lsp_dynamic_workspace_symbols, "Workspace Symbols")
		map("grt", require("telescope.builtin").lsp_type_definitions, "[G]oto [T]ype Definition")

		map("gd", vim.lsp.buf.definition, "Go to Definition")
		map("gi", vim.lsp.buf.implementation, "Go to Implementation")
		map("K", vim.lsp.buf.hover, "Hover Documentation")
		map("<C-k>", vim.lsp.buf.signature_help, "Signature Help")
		map("<leader>k", vim.diagnostic.open_float, "Diagnostic Float")
		map("<leader>rn", vim.lsp.buf.rename, "Rename")
		map("<leader>ca", vim.lsp.buf.code_action, "Code Action")
		map("<leader>gr", vim.lsp.buf.references, "References")
		map("<leader>gd", vim.lsp.buf.type_definition, "Type Definition")
		map("<leader>gg", ":LspRestart<CR>", "Restart LSP")
		map("[d", function()
			vim.diagnostic.jump({ count = -1 })
		end, "Previous Diagnostic")
		map("]d", function()
			vim.diagnostic.jump({ count = 1 })
		end, "Next Diagnostic")

		map("<leader>ci", require("telescope.builtin").lsp_incoming_calls, "Incoming Calls")
		map("<leader>co", require("telescope.builtin").lsp_outgoing_calls, "Outgoing Calls")

		local client = vim.lsp.get_client_by_id(event.data.client_id)
		if not client then
			return
		end

		if client:supports_method(vim.lsp.protocol.Methods.textDocument_documentHighlight, event.buf) then
			local highlight_augroup = vim.api.nvim_create_augroup("lsp-highlight", { clear = false })
			vim.api.nvim_create_autocmd({ "CursorHold", "CursorHoldI" }, {
				buffer = event.buf,
				group = highlight_augroup,
				callback = vim.lsp.buf.document_highlight,
			})
			vim.api.nvim_create_autocmd({ "CursorMoved", "CursorMovedI" }, {
				buffer = event.buf,
				group = highlight_augroup,
				callback = vim.lsp.buf.clear_references,
			})
			vim.api.nvim_create_autocmd("LspDetach", {
				group = vim.api.nvim_create_augroup("lsp-detach", { clear = true }),
				callback = function(event2)
					vim.lsp.buf.clear_references()
					vim.api.nvim_clear_autocmds({ group = "lsp-highlight", buffer = event2.buf })
				end,
			})
		end

		if client:supports_method(vim.lsp.protocol.Methods.textDocument_inlayHint, event.buf) then
			map("<leader>th", function()
				vim.lsp.inlay_hint.enable(not vim.lsp.inlay_hint.is_enabled({ bufnr = event.buf }))
			end, "[T]oggle Inlay [H]ints")
		end

		if client:supports_method(vim.lsp.protocol.Methods.textDocument_codeLens, event.buf) then
			map("<leader>cl", vim.lsp.codelens.run, "[C]ode [L]ens")
			vim.api.nvim_create_autocmd({ "BufEnter", "CursorHold", "InsertLeave" }, {
				buffer = event.buf,
				callback = vim.lsp.codelens.refresh,
			})
		end

		if client.name == "ts_ls" then
			map("<leader>oi", function()
				vim.lsp.buf.code_action({
					apply = true,
					context = { only = { "source.organizeImports.ts" }, diagnostics = {} },
				})
			end, "Organize Imports")
			map("<leader>ru", function()
				vim.lsp.buf.code_action({
					apply = true,
					context = { only = { "source.removeUnused.ts" }, diagnostics = {} },
				})
			end, "Remove Unused")
			map("<leader>am", function()
				vim.lsp.buf.code_action({
					apply = true,
					context = { only = { "source.addMissingImports.ts" }, diagnostics = {} },
				})
			end, "Add Missing Imports")
			map("<leader>fa", function()
				vim.lsp.buf.code_action({
					apply = true,
					context = { only = { "source.fixAll.ts" }, diagnostics = {} },
				})
			end, "Fix All")
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
		source = true,
		spacing = 2,
	},
})

require("conform").setup({
	formatters_by_ft = {
		bash = { "shellharden" },
		sh = { "shellharden" },
		zsh = { "beautysh" },
		python = { "black" },
		css = { "prettierd" },
		json = { "prettierd" },
		jsonc = { "prettierd" },
		lua = { "stylua" },
		toml = { "taplo" },
		javascript = { "prettierd" },
		javascriptreact = { "prettierd" },
		typescript = { "prettierd" },
		typescriptreact = { "prettierd" },
		html = { "prettierd" },
		markdown = { "prettierd" },
		yaml = { "prettierd" },
		go = { "goimports", "gofumpt" },
		templ = { "templ" },
		dockerfile = { "hadolint" },
	},
	format_on_save = {
		timeout_ms = 500,
	},
})

require("lint").linters_by_ft = {
	go = { "golangcilint" },
	zsh = { "zsh" },
	dockerfile = { "hadolint" },
}

vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost" }, {
	callback = function()
		require("lint").try_lint()
	end,
})

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
					fieldalignment = true,
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
				suggest = {
					completeFunctionCalls = true,
				},
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
				suggest = {
					completeFunctionCalls = true,
				},
			},
		},
	},
	eslint = {
		settings = {
			workingDirectories = { mode = "auto" },
		},
	},
	tailwindcss = {
		settings = {
			tailwindCSS = {
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
	html = {
		filetypes = { "html", "templ" },
	},
	jsonls = {
		settings = {
			json = {
				validate = { enable = true },
			},
		},
	},
	dockerls = {},
	docker_compose_language_service = {},
	emmet_ls = {
		filetypes = {
			"html",
			"css",
			"javascript",
			"javascriptreact",
			"typescript",
			"typescriptreact",
		},
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

local ensure_installed = vim.tbl_keys(servers)
vim.list_extend(ensure_installed, {
	"stylua",
	"shellharden",
	"beautysh",
	"black",
	"prettierd",
	"goimports",
	"gofumpt",
	"golangci-lint",
	"delve",
	"staticcheck",
	"gomodifytags",
	"impl",
	"gotests",
	"hadolint",
})

require("mason").setup({ ui = { border = "rounded" } })
require("mason-tool-installer").setup({ ensure_installed = ensure_installed })

local capabilities = require("blink.cmp").get_lsp_capabilities()

require("mason-lspconfig").setup({
	ensure_installed = {},
	automatic_installation = false,
	handlers = {
		function(server_name)
			local server = servers[server_name] or {}
			server.capabilities = vim.tbl_deep_extend("force", {}, capabilities, server.capabilities or {})
			require("lspconfig")[server_name].setup(server)
		end,
	},
})
