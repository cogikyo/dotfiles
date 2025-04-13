-- ============================================================================
-- 🧰 Setup {{{
local cmp_capabilities_ok, cmp = pcall(require, "cmp_nvim_lsp")
if not cmp_capabilities_ok then
	vim.api.nvim_echo({
		{
			"Error: nvim-cmp uses lsp completions, but cmp-nvim-lsp is not found... skipping lsp setup()",
			"Error",
		},
	}, true, {})
	return
end

local mason_ok, mason = pcall(require, "mason")
if not mason_ok then
	vim.api.nvim_echo({
		{
			"Error: lsp uses mason to configure various settings, but mason is not found... skipping lsp setup()",
			"Error",
		},
	}, true, {})
	return
end
-- }}}
-- ============================================================================

-- Formatting with conform.nvim
require("conform").setup({
	formatters_by_ft = {
		bash = { "shellharden" },
		sh = { "shellharden" },
		zsh = { "beautysh" },
		python = { "black" },
		css = { "stylelint", "prettier" },
		scss = { "stylelint", "prettier" },
		json = { "jq" },
		rust = { "rustfmt" },
		lua = { "stylua" },
		toml = { "taplo" },
		javascript = { "prettier" },
		typescript = { "prettier" },
		html = { "prettier" },
		go = { "gofmt", "goimports", "golines" },
	},
	format_on_save = {
		timeout_ms = 500,
		lsp_fallback = true, -- Use LSP formatting if no formatter is available
	},
})

-- Linting with nvim-lint
require("lint").linters_by_ft = {
	css = { "stylelint" },
	scss = { "stylelint" },
	go = { "gospel" },
	zsh = { "zsh" },
}

-- Run linting on write or read
vim.api.nvim_create_autocmd({ "BufWritePost", "BufReadPost" }, {
	callback = function()
		require("lint").try_lint()
	end,
})

local on_attach = function(client, bufnr)
	local nmap = function(keys, func)
		vim.keymap.set("n", keys, func, { buffer = bufnr })
	end

	if client.name == "rust_analyzer" then
		client.server_capabilities.semanticTokensProvider = nil
	end

	nmap("<leader>rn", vim.lsp.buf.rename)
	nmap("<leader>ca", vim.lsp.buf.code_action)
	nmap("<leader>gr", vim.lsp.buf.references)
	nmap("<leader>gd", vim.lsp.buf.type_definition)
	nmap("<leader>gg", ":LspRestart<CR>")
	nmap("gd", vim.lsp.buf.definition)
	nmap("gi", vim.lsp.buf.implementation)
	nmap("K", vim.lsp.buf.hover)
	nmap("<C-k>", vim.lsp.buf.signature_help)
	nmap("<leader>k", vim.diagnostic.open_float)
	nmap("[d", vim.diagnostic.goto_prev)
	nmap("]d", vim.diagnostic.goto_next)
end

-- ♦️  Vim Diagnostic Settings:
local signs = {
	{ name = "DiagnosticSignError", text = "" },
	{ name = "DiagnosticSignWarn", text = "" },
	{ name = "DiagnosticSignHint", text = "" },
	{ name = "DiagnosticSignInfo", text = "" },
}

for _, sign in ipairs(signs) do
	vim.fn.sign_define(sign.name, {
		texthl = sign.name,
		text = sign.text,
		numhl = sign.name,
	})
end

local diagnostic_config = {
	underline = false,
	virtual_text = true,
	signs = true,
	update_in_insert = false,
	float = {
		focusable = false,
		style = "minimal",
		border = "rounded",
		source = "always",
		header = "",
		prefix = "",
	},
}

vim.diagnostic.config(diagnostic_config)

vim.lsp.handlers["textDocument/hover"] = vim.lsp.with(vim.lsp.handlers.hover, {
	border = "rounded",
})

vim.lsp.handlers["textDocument/signatureHelp"] = vim.lsp.with(vim.lsp.handlers.signature_help, {
	border = "rounded",
})

-- 🎾 Automatic Server Setup:
-- https://github.com/williamboman/mason-lspconfig.nvim#default-configuration
local servers = {
	bashls = {},
	gopls = {},
	cssls = {},
	stylelint_lsp = {},
	css_variables = {},
	templ = {},
	marksman = {},
	pyright = {},
	taplo = {},
	lua_ls = {
		Lua = {
			workspace = { checkThirdParty = false },
			telemetry = { enable = false },
			diagnostics = { globals = { "vim" } },
			format = { enable = false },
		},
	},
}

-- ============================================================================
-- 🧱 MASON, DO THE THING! {{{
mason.setup({
	ui = { border = "rounded" },
})

local mason_lspconfig = require("mason-lspconfig")
local capabilities = vim.lsp.protocol.make_client_capabilities()

capabilities = cmp.default_capabilities(capabilities)

mason_lspconfig.setup({
	ensure_installed = vim.tbl_keys(servers),
})

mason_lspconfig.setup_handlers({
	function(server_name)
		require("lspconfig")[server_name].setup({
			capabilities = capabilities,
			on_attach = on_attach,
			settings = servers[server_name],
		})
	end,
})
-- }}}
-- ============================================================================
