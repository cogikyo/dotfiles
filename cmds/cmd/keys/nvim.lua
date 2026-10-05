local out = assert(os.getenv("KEYS_OUT"))
local cwd = vim.uv.cwd()

local function write(result)
	local file = assert(io.open(out, "w"))
	file:write(vim.json.encode(result))
	file:close()
end

local function finish(result)
	write(result)
	vim.cmd("noautocmd qall!")
end

local lazypath = vim.fn.stdpath("data") .. "/lazy/lazy.nvim"
if not vim.uv.fs_stat(lazypath) then
	write({ error = "lazy.nvim is not installed at " .. lazypath })
	os.exit(1)
end

local function patch(name, fn)
	package.preload[name] = function()
		package.preload[name] = nil
		for i = 2, #package.loaders do
			local loader = package.loaders[i](name)
			if type(loader) == "function" then
				local mod = loader(name)
				fn(mod)
				package.loaded[name] = mod
				return mod
			end
		end
		error("keys: cannot load " .. name)
	end
end

local function noop() end

patch("lazy", function(lazy)
	local setup = lazy.setup
	lazy.setup = function(spec, opts)
		local o = (type(spec) == "table" and spec.spec) and spec or (opts or {})
		o.install = vim.tbl_extend("force", o.install or {}, { missing = false })
		o.checker = { enabled = false }
		o.change_detection = { enabled = false }
		o.rocks = vim.tbl_extend("force", o.rocks or {}, { enabled = false })
		if o == spec then
			return setup(o)
		end
		return setup(spec, o)
	end
end)

patch("nvim-treesitter", function(ts)
	ts.install = noop
	ts.update = noop
end)

patch("mason-tool-installer", function(mti)
	mti.run_on_start = noop
	mti.check_install = noop
	mti.clean = noop
end)

patch("mason-lspconfig", function(mlsp)
	local setup = mlsp.setup
	mlsp.setup = function(opts)
		opts = vim.tbl_extend("force", opts or {}, { ensure_installed = {} })
		return setup(opts)
	end
end)

local function dump()
	local runtime = vim.env.VIMRUNTIME
	local scripts = {}
	for _, info in ipairs(vim.fn.getscriptinfo()) do
		scripts[info.sid] = info.name
	end

	local keymap = vim.api.nvim_get_keymap("n")
	local targets = {}
	for _, m in ipairs(keymap) do
		targets[vim.fn.keytrans(m.lhsraw or m.lhs)] = m.desc
	end

	local maps = {}
	for _, m in ipairs(keymap) do
		local path = scripts[m.sid] or ""
		local rhs = m.rhs or ""
		maps[#maps + 1] = {
			lhs = vim.fn.keytrans(m.lhsraw or m.lhs),
			rhs = rhs,
			desc = m.desc or (vim.startswith(rhs, "<Plug>") and targets[vim.fn.keytrans(vim.keycode(rhs))]) or "",
			builtin = vim.startswith(path, runtime) or vim.startswith(path, cwd .. "/vim/"),
		}
	end

	local groups = {}
	local keymaps = package.loaded["config.keymaps"]
	local specs = type(keymaps) == "table" and keymaps.groups
	for _, spec in ipairs(type(specs) == "table" and specs or {}) do
		local mode = spec.mode or "n"
		if type(mode) == "string" then
			mode = { mode }
		end
		if spec.group and vim.tbl_contains(mode, "n") then
			groups[#groups + 1] = { lhs = vim.fn.keytrans(vim.keycode(spec[1])), label = spec.group }
		end
	end

	finish({
		maps = maps,
		groups = groups,
		groupsLoaded = type(specs) == "table",
		leader = vim.fn.keytrans(vim.g.mapleader or "\\"),
		localleader = vim.fn.keytrans(vim.g.maplocalleader or "\\"),
	})
end

vim.api.nvim_create_autocmd("VimEnter", {
	once = true,
	callback = function()
		vim.schedule(function()
			local ok, err = pcall(dump)
			if not ok then
				finish({ error = tostring(err) })
			end
		end)
	end,
})
