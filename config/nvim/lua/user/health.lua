local M = {}

local server_to_mason = {
	bashls = "bash-language-server",
	gopls = "gopls",
	ts_ls = "typescript-language-server",
	eslint = "eslint-lsp",
	tailwindcss = "tailwindcss-language-server",
	cssls = "css-lsp",
	html = "html-lsp",
	jsonls = "json-lsp",
	dockerls = "dockerfile-language-server",
	docker_compose_language_service = "docker-compose-language-service",
	emmet_ls = "emmet-ls",
	templ = "templ",
	marksman = "marksman",
	pyright = "pyright",
	taplo = "taplo",
	lua_ls = "lua-language-server",
}

local check_version = function()
	local verstr = tostring(vim.version())
	if not vim.version.ge then
		vim.health.error(string.format("Neovim out of date: '%s'. Upgrade to latest stable or nightly", verstr))
		return
	end
	if vim.version.ge(vim.version(), "0.10-dev") then
		vim.health.ok(string.format("Neovim version is: '%s'", verstr))
	else
		vim.health.error(string.format("Neovim out of date: '%s'. Upgrade to latest stable or nightly", verstr))
	end
end

local check_external_reqs = function()
	local required = { "git", "make", "unzip", "rg" }
	for _, exe in ipairs(required) do
		if vim.fn.executable(exe) == 1 then
			vim.health.ok(string.format("'%s' found", exe))
		else
			vim.health.error(string.format("'%s' not found - install via system package manager", exe))
		end
	end
end

local check_lang_runtimes = function()
	local runtimes = {
		{
			name = "TypeScript/JavaScript",
			deps = {
				{ "node", "ts_ls, eslint, tailwindcss, prettierd" },
				{ "npm", "package management" },
			},
			tools = { "vite", "eslint", "tsc" },
		},
		{
			name = "Go",
			deps = {
				{ "go", "gopls, goimports, gofumpt, delve" },
			},
		},
		{
			name = "Python",
			deps = {
				{ "python3", "pyright, black" },
			},
		},
	}

	for _, lang in ipairs(runtimes) do
		vim.health.info(lang.name .. ":")
		local all_found = true
		for _, dep in ipairs(lang.deps) do
			if vim.fn.executable(dep[1]) == 1 then
				vim.health.ok(string.format("'%s' (%s)", dep[1], dep[2]))
			else
				all_found = false
				vim.health.warn(string.format("'%s' not found - needed for %s", dep[1], dep[2]))
			end
		end
		if lang.tools and all_found then
			for _, tool in ipairs(lang.tools) do
				if vim.fn.executable(tool) == 1 then
					vim.health.ok(string.format("'%s' available", tool))
				end
			end
		end
	end
end

local check_formatters = function()
	local formatters = {
		{ "stylua", "lua" },
		{ "shellharden", "bash/sh" },
		{ "beautysh", "zsh" },
		{ "black", "python" },
		{ "prettierd", "js/ts/css/html/json/yaml/md" },
		{ "goimports", "go" },
		{ "gofumpt", "go" },
		{ "taplo", "toml" },
		{ "templ", "templ" },
		{ "hadolint", "dockerfile" },
	}

	local found, missing = 0, 0
	for _, fmt in ipairs(formatters) do
		if vim.fn.executable(fmt[1]) == 1 then
			found = found + 1
			vim.health.ok(string.format("'%s' (%s)", fmt[1], fmt[2]))
		else
			missing = missing + 1
			vim.health.warn(string.format("'%s' (%s) - run :MasonInstall %s", fmt[1], fmt[2], fmt[1]))
		end
	end
	vim.health.info(string.format("%d/%d formatters installed", found, found + missing))
end

local check_linters = function()
	local linters = {
		{ "golangci-lint", "go" },
		{ "hadolint", "dockerfile" },
	}

	local found, missing = 0, 0
	for _, linter in ipairs(linters) do
		if vim.fn.executable(linter[1]) == 1 then
			found = found + 1
			vim.health.ok(string.format("'%s' (%s)", linter[1], linter[2]))
		else
			missing = missing + 1
			vim.health.warn(string.format("'%s' (%s) - run :MasonInstall %s", linter[1], linter[2], linter[1]))
		end
	end
	vim.health.info(string.format("%d/%d linters installed", found, found + missing))
end

local check_lsp_servers = function()
	local ok, registry = pcall(require, "mason-registry")
	if not ok then
		vim.health.error("mason-registry not available")
		return
	end

	local clients = vim.lsp.get_clients()
	local active_names = {}
	for _, client in ipairs(clients) do
		active_names[client.name] = true
	end

	local installed, missing = 0, 0
	for server, mason_name in pairs(server_to_mason) do
		local is_installed = registry.is_installed(mason_name)
		if is_installed then
			installed = installed + 1
			if active_names[server] then
				vim.health.ok(string.format("'%s' installed and active", server))
			else
				vim.health.ok(string.format("'%s' installed", server))
			end
		else
			missing = missing + 1
			vim.health.warn(string.format("'%s' - run :MasonInstall %s", server, mason_name))
		end
	end
	vim.health.info(string.format("%d/%d LSP servers installed", installed, installed + missing))
end

local check_plugins = function()
	local plugins = {
		{ "lazy", "lazy.nvim" },
		{ "nvim-treesitter", "nvim-treesitter" },
		{ "telescope", "telescope.nvim" },
		{ "lspconfig", "nvim-lspconfig" },
		{ "blink.cmp", "blink.cmp" },
		{ "conform", "conform.nvim" },
		{ "lint", "nvim-lint" },
		{ "mason", "mason.nvim" },
		{ "gitsigns", "gitsigns.nvim" },
		{ "nvim-tree", "nvim-tree.lua" },
		{ "lualine", "lualine.nvim" },
		{ "trouble", "trouble.nvim" },
		{ "harpoon", "harpoon" },
		{ "Comment", "Comment.nvim" },
		{ "fidget", "fidget.nvim" },
	}

	local loaded, failed = 0, 0
	for _, plugin in ipairs(plugins) do
		local ok, _ = pcall(require, plugin[1])
		if ok then
			loaded = loaded + 1
			vim.health.ok(string.format("'%s' loaded", plugin[2]))
		else
			failed = failed + 1
			vim.health.warn(string.format("'%s' not loaded", plugin[2]))
		end
	end
	vim.health.info(string.format("Plugins: %d loaded, %d failed", loaded, failed))
end

local check_treesitter = function()
	local ok, ts = pcall(require, "nvim-treesitter.parsers")
	if not ok then
		vim.health.error("nvim-treesitter not available")
		return
	end

	local parsers = {
		"lua",
		"vim",
		"vimdoc",
		"query",
		"go",
		"gomod",
		"gosum",
		"gowork",
		"javascript",
		"typescript",
		"tsx",
		"html",
		"css",
		"json",
		"yaml",
		"toml",
		"markdown",
		"markdown_inline",
		"bash",
		"python",
		"dockerfile",
		"templ",
	}

	local installed, missing = 0, 0
	for _, parser in ipairs(parsers) do
		if ts.has_parser(parser) then
			installed = installed + 1
			vim.health.ok(string.format("'%s' parser installed", parser))
		else
			missing = missing + 1
			vim.health.warn(string.format("'%s' parser not installed", parser))
		end
	end
	vim.health.info(string.format("Treesitter parsers: %d installed, %d missing", installed, missing))
end

local check_go_tools = function()
	local tools = {
		{ "delve", "dlv", "debugger" },
		{ "staticcheck", "staticcheck", "linter" },
		{ "gomodifytags", "gomodifytags", "struct tags" },
		{ "impl", "impl", "interface stubs" },
		{ "gotests", "gotests", "test generation" },
	}

	local found, missing = 0, 0
	for _, tool in ipairs(tools) do
		if vim.fn.executable(tool[2]) == 1 then
			found = found + 1
			vim.health.ok(string.format("'%s' (%s)", tool[1], tool[3]))
		else
			missing = missing + 1
			vim.health.warn(string.format("'%s' (%s) - run :MasonInstall %s", tool[1], tool[3], tool[1]))
		end
	end
	vim.health.info(string.format("%d/%d go tools installed", found, found + missing))
end

M.check = function()
	vim.health.start("user.nvim")

	local uv = vim.uv or vim.loop
	local info = uv.os_uname()
	vim.health.info(string.format("%s %s (%s)", info.sysname, info.release:match("^[^-]+"), info.machine))

	vim.health.start("Version")
	check_version()

	vim.health.start("Core Dependencies")
	check_external_reqs()

	vim.health.start("Language Runtimes")
	check_lang_runtimes()

	vim.health.start("Plugins")
	check_plugins()

	vim.health.start("Treesitter Parsers")
	check_treesitter()

	vim.health.start("LSP Servers")
	check_lsp_servers()

	vim.health.start("Formatters (conform.nvim)")
	check_formatters()

	vim.health.start("Linters (nvim-lint)")
	check_linters()

	vim.health.start("Go Tools")
	check_go_tools()
end

return M
