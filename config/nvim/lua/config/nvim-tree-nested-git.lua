local NestedGit = require("nvim-tree.api").Decorator:extend()

local cache = {}
local ttl_ms = 5000

local function now_ms()
	return vim.uv.hrtime() / 1000000
end

local function has_git_dir(path)
	local stat = vim.uv.fs_stat(path .. "/.git")
	return stat ~= nil and (stat.type == "directory" or stat.type == "file")
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
		{ "git", "-C", path, "status", "--porcelain=v1", "--untracked-files=normal" },
		{ text = true },
		function(result)
			entry.pending = false
			entry.checked_at = now_ms()
			entry.dirty = result.code == 0 and vim.trim(result.stdout or "") ~= ""

			vim.schedule(redraw_tree)
		end
	)
end

local function dirty(path)
	local entry = cache[path]
	if not entry then
		entry = { is_git_root = has_git_dir(path), checked_at = 0, dirty = false, pending = false }
		cache[path] = entry
	end

	if not entry.is_git_root then
		return false
	end

	if now_ms() - entry.checked_at > ttl_ms then
		refresh(path, entry)
	end

	return entry.dirty
end

function NestedGit:new()
	self.enabled = true
	self.highlight_range = "all"
	self.icon_placement = "after"
	self.icon = { str = "", hl = { "NvimTreeGitDirtyIcon" } }
end

function NestedGit:highlight_group(node)
	if node.type == "directory" and node.absolute_path and dirty(node.absolute_path) then
		return "NvimTreeGitFolderDirtyHL"
	end
end

function NestedGit:icons(node)
	if node.type == "directory" and node.absolute_path and dirty(node.absolute_path) then
		return { self.icon }
	end
end

return NestedGit
