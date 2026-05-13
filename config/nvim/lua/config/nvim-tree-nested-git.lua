local NestedGit = require("nvim-tree.api").Decorator:extend()

local cache = {}
local root_by_path = {}
local ttl_ms = 5000

local function now_ms()
	return vim.uv.hrtime() / 1000000
end

local function has_git_dir(path)
	local stat = vim.uv.fs_stat(path .. "/.git")
	return stat ~= nil and (stat.type == "directory" or stat.type == "file")
end

local function dirname(path)
	return vim.fn.fnamemodify(path, ":h")
end

local function find_git_root(path)
	local cached = root_by_path[path]
	if cached ~= nil then
		return cached or nil
	end

	local stat = vim.uv.fs_stat(path)
	local dir = stat and stat.type == "directory" and path or dirname(path)
	while dir and dir ~= "" do
		if has_git_dir(dir) then
			root_by_path[path] = dir
			return dir
		end

		local parent = dirname(dir)
		if parent == dir then
			break
		end
		dir = parent
	end

	root_by_path[path] = false
	return nil
end

local function add_status(statuses, path, status)
	if not path or not status or status == "" then
		return
	end

	statuses[path] = statuses[path] or {}
	statuses[path][status] = true
end

local function parse_status(root, output)
	local files = {}
	local dirs = { direct = {}, indirect = {} }
	local skip_next = false

	for item in (output or ""):gmatch("[^%z]+") do
		if skip_next then
			skip_next = false
		else
			local status = item:sub(1, 2)
			local relative = item:sub(4):gsub("/$", "")
			local path = root .. "/" .. relative

			files[path] = status
			add_status(dirs.direct, dirname(path), status)

			local parent = dirname(path)
			while parent ~= root do
				parent = dirname(parent)
				add_status(dirs.indirect, parent, status)
			end

			if status:find("R", 1, true) then
				skip_next = true
			end
		end
	end

	return files, dirs
end

local function redraw_tree()
	local ok, core = pcall(require, "nvim-tree.core")
	local explorer = ok and core.get_explorer()
	if explorer and explorer.renderer then
		explorer.renderer:draw()
	end
end

local function refresh(path, entry)
	if entry.pending then
		return
	end

	entry.pending = true
	vim.system(
		{ "git", "-C", path, "status", "--porcelain=v1", "-z", "--untracked-files=normal" },
		{ text = true },
		function(result)
			entry.pending = false
			entry.checked_at = now_ms()

			if result.code == 0 then
				entry.files, entry.dirs = parse_status(path, result.stdout)
				entry.dirty = (result.stdout or "") ~= ""
			else
				entry.files, entry.dirs, entry.dirty = {}, { direct = {}, indirect = {} }, false
			end

			vim.schedule(redraw_tree)
		end
	)
end

local function root_entry(root)
	local entry = cache[root]
	if not entry then
		entry = { checked_at = 0, dirty = false, pending = false, files = {}, dirs = { direct = {}, indirect = {} } }
		cache[root] = entry
	end

	if now_ms() - entry.checked_at > ttl_ms then
		refresh(root, entry)
	end

	return entry
end

local function has_native_git_status(node)
	if type(node.get_git_xy) ~= "function" then
		return false
	end

	local ok, status = pcall(node.get_git_xy, node)
	return ok and status ~= nil and #status > 0
end

local function dirty(node)
	if not node.absolute_path or has_native_git_status(node) then
		return false
	end

	local root = find_git_root(node.absolute_path)
	if not root then
		return false
	end

	local entry = root_entry(root)
	if node.type == "file" then
		return entry.files[node.absolute_path] ~= nil
	end

	if node.type == "directory" then
		return entry.files[node.absolute_path] ~= nil
			or entry.dirs.direct[node.absolute_path] ~= nil
			or entry.dirs.indirect[node.absolute_path] ~= nil
			or (node.absolute_path == root and entry.dirty)
	end

	return false
end

vim.api.nvim_create_autocmd("BufWritePost", {
	group = vim.api.nvim_create_augroup("NvimTreeNestedGit", { clear = true }),
	callback = function(args)
		local path = vim.api.nvim_buf_get_name(args.buf)
		local root = path ~= "" and find_git_root(path) or nil
		local entry = root and cache[root]
		if entry then
			entry.checked_at = 0
			refresh(root, entry)
		end
	end,
})

function NestedGit:new()
	self.enabled = true
	self.highlight_range = "all"
	self.icon_placement = "after"
	self.icon = { str = "", hl = { "NvimTreeGitDirtyIcon" } }
end

function NestedGit:highlight_group(node)
	if dirty(node) then
		return node.type == "directory" and "NvimTreeGitFolderDirtyHL" or "NvimTreeGitFileDirtyHL"
	end
end

function NestedGit:icons(node)
	if dirty(node) then
		return { self.icon }
	end
end

NestedGit.is_dirty = dirty

return NestedGit
