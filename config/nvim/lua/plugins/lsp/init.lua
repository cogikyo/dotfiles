-- Core LSP configuration: nvim-lspconfig, diagnostics, LspAttach
return {
	"neovim/nvim-lspconfig",
	event = { "BufReadPre", "BufNewFile" },
	dependencies = {
		"williamboman/mason.nvim",
		"williamboman/mason-lspconfig.nvim",
	},
	config = function()
		local servers = require("config.servers").servers
		local keymaps = require("config.keymaps")
		local capabilities = require("blink.cmp").get_lsp_capabilities()

		-- Diagnostics
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
			virtual_text = { source = false, spacing = 2 },
		})

		-- Hover border
		vim.lsp.handlers["textDocument/hover"] = vim.lsp.with(vim.lsp.handlers.hover, { border = "rounded" })

		-- LspAttach
		vim.api.nvim_create_autocmd("LspAttach", {
			group = vim.api.nvim_create_augroup("lsp-attach", { clear = true }),
			callback = function(event)
				keymaps.on_attach(event)

				local client = vim.lsp.get_client_by_id(event.data.client_id)
				if not client then
					return
				end

				-- TypeScript-specific actions
				if client.name == "ts_ls" then
					keymaps.ts_actions(event)
				end

				-- Auto-enable inlay hints for Go/TS
				if client.name == "gopls" or client.name == "ts_ls" then
					vim.lsp.inlay_hint.enable(true, { bufnr = event.buf })
				end

					-- Codelens refresh
				if client:supports_method(vim.lsp.protocol.Methods.textDocument_codeLens, event.buf) then
					vim.api.nvim_create_autocmd({ "BufEnter", "InsertLeave" }, {
						buffer = event.buf,
						callback = vim.lsp.codelens.refresh,
					})
				end
			end,
		})

		-- Enable servers
		for name, config in pairs(servers) do
			config.capabilities = vim.tbl_deep_extend("force", {}, capabilities, config.capabilities or {})
			vim.lsp.config(name, config)
			vim.lsp.enable(name)
		end
	end,
}
