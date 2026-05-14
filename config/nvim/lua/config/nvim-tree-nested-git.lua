local NestedGit = require("nvim-tree.api").Decorator:extend()

local cache = {}
local root_by_path = {}
local ttl_ms = 5000

local fallback_glyphs = {
	unstaged = "",
	staged = "",
	unmerged = "",
	renamed = "",
	deleted = "󰮈",
	untracked = "",
	ignored = "",
}

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

local function native_tree_root()
	local ok, core = pcall(require, "nvim-tree.core")
	local explorer = ok and core.get_explorer()
	return explorer and explorer.absolute_path and find_git_root(explorer.absolute_path) or nil
end

local function native_git_glyphs()
	local ok, core = pcall(require, "nvim-tree.core")
	local explorer = ok and core.get_explorer()
	return explorer
			and explorer.opts
			and explorer.opts.renderer
			and explorer.opts.renderer.icons
			and explorer.opts.renderer.icons.glyphs
			and explorer.opts.renderer.icons.glyphs.git
		or fallback_glyphs
end

local function build_icons_by_status(glyphs)
	return {
		staged = { str = glyphs.staged, hl = { "NvimTreeGitStagedIcon" }, ord = 1 },
		unstaged = { str = glyphs.unstaged, hl = { "NvimTreeGitDirtyIcon" }, ord = 2 },
		renamed = { str = glyphs.renamed, hl = { "NvimTreeGitRenamedIcon" }, ord = 3 },
		deleted = { str = glyphs.deleted, hl = { "NvimTreeGitDeletedIcon" }, ord = 4 },
		unmerged = { str = glyphs.unmerged, hl = { "NvimTreeGitMergeIcon" }, ord = 5 },
		untracked = { str = glyphs.untracked, hl = { "NvimTreeGitNewIcon" }, ord = 6 },
		ignored = { str = glyphs.ignored, hl = { "NvimTreeGitIgnoredIcon" }, ord = 7 },
	}
end

local function build_icons_by_xy(icons)
	return {
		["M "] = { icons.staged },
		[" M"] = { icons.unstaged },
		["C "] = { icons.staged },
		[" C"] = { icons.unstaged },
		CM = { icons.unstaged },
		[" T"] = { icons.unstaged },
		["T "] = { icons.staged },
		TM = { icons.staged, icons.unstaged },
		MM = { icons.staged, icons.unstaged },
		MD = { icons.staged },
		["A "] = { icons.staged },
		AD = { icons.staged },
		[" A"] = { icons.untracked },
		AA = { icons.unmerged, icons.untracked },
		AU = { icons.unmerged, icons.untracked },
		AM = { icons.staged, icons.unstaged },
		["??"] = { icons.untracked },
		["R "] = { icons.renamed },
		[" R"] = { icons.renamed },
		RM = { icons.unstaged, icons.renamed },
		UU = { icons.unmerged },
		UD = { icons.unmerged },
		UA = { icons.unmerged },
		[" D"] = { icons.deleted },
		["D "] = { icons.deleted },
		DA = { icons.unstaged },
		RD = { icons.deleted },
		DD = { icons.deleted },
		DU = { icons.deleted, icons.unmerged },
		["!!"] = { icons.ignored },
		dirty = { icons.unstaged },
	}
end

local file_hl_by_xy = {
	["M "] = "NvimTreeGitFileStagedHL",
	["C "] = "NvimTreeGitFileStagedHL",
	AA = "NvimTreeGitFileStagedHL",
	AD = "NvimTreeGitFileStagedHL",
	MD = "NvimTreeGitFileStagedHL",
	["T "] = "NvimTreeGitFileStagedHL",
	TT = "NvimTreeGitFileStagedHL",
	[" M"] = "NvimTreeGitFileDirtyHL",
	CM = "NvimTreeGitFileDirtyHL",
	[" C"] = "NvimTreeGitFileDirtyHL",
	[" T"] = "NvimTreeGitFileDirtyHL",
	MM = "NvimTreeGitFileDirtyHL",
	AM = "NvimTreeGitFileDirtyHL",
	dirty = "NvimTreeGitFileDirtyHL",
	["A "] = "NvimTreeGitFileStagedHL",
	["??"] = "NvimTreeGitFileNewHL",
	AU = "NvimTreeGitFileMergeHL",
	UU = "NvimTreeGitFileMergeHL",
	UD = "NvimTreeGitFileMergeHL",
	DU = "NvimTreeGitFileMergeHL",
	UA = "NvimTreeGitFileMergeHL",
	[" D"] = "NvimTreeGitFileDeletedHL",
	DD = "NvimTreeGitFileDeletedHL",
	RD = "NvimTreeGitFileDeletedHL",
	["D "] = "NvimTreeGitFileDeletedHL",
	["R "] = "NvimTreeGitFileRenamedHL",
	RM = "NvimTreeGitFileRenamedHL",
	[" R"] = "NvimTreeGitFileRenamedHL",
	["!!"] = "NvimTreeGitFileIgnoredHL",
	[" A"] = "NvimTreeGitFileNewHL",
}

local function add_statuses(statuses, source)
	if type(source) ~= "table" then
		return
	end

	for status in pairs(source) do
		table.insert(statuses, status)
	end
end

local function has_native_git_status(node)
	if type(node.get_git_xy) ~= "function" then
		local status = type(node.git_status) == "table" and node.git_status or nil
		if not status then
			return false
		end

		if type(status.file) == "string" and status.file ~= "" then
			return true
		end

		local dir = type(status.dir) == "table" and status.dir or nil
		if not dir then
			return false
		end

		for _, statuses in ipairs({ dir.direct, dir.indirect }) do
			if type(statuses) == "table" then
				for _, value in pairs(statuses) do
					if type(value) == "string" and value ~= "" then
						return true
					end
				end
			end
		end

		return false
	end

	local ok, status = pcall(node.get_git_xy, node)
	return ok and status ~= nil and #status > 0
end

local function statuses(node)
	if not node.absolute_path or has_native_git_status(node) then
		return nil
	end

	local root = find_git_root(node.absolute_path)
	if not root or root == native_tree_root() then
		return nil
	end

	local entry = root_entry(root)
	if node.type == "file" then
		local status = entry.files[node.absolute_path]
		return status and { status } or nil
	end

	if node.type == "directory" then
		local result = {}
		if entry.files[node.absolute_path] then
			table.insert(result, entry.files[node.absolute_path])
		end
		add_statuses(result, entry.dirs.direct[node.absolute_path])
		add_statuses(result, entry.dirs.indirect[node.absolute_path])

		if #result > 0 then
			return result
		end
	end

	return nil
end

local function best_status(statuses_, icons_by_xy)
	local best
	local best_ord = math.huge
	for _, status in ipairs(statuses_) do
		for _, icon in ipairs(icons_by_xy[status] or icons_by_xy.dirty) do
			if icon.ord < best_ord then
				best = status
				best_ord = icon.ord
			end
		end
	end

	return best or "dirty"
end

local function dirty(node)
	return statuses(node) ~= nil
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
	self.icons_by_xy = build_icons_by_xy(build_icons_by_status(native_git_glyphs()))
end

function NestedGit:highlight_group(node)
	local statuses_ = statuses(node)
	if statuses_ then
		local hl = file_hl_by_xy[best_status(statuses_, self.icons_by_xy)] or file_hl_by_xy.dirty
		return node.type == "directory" and hl:gsub("File", "Folder") or hl
	end
end

function NestedGit:icons(node)
	local statuses_ = statuses(node)
	if not statuses_ then
		return nil
	end

	local inserted = {}
	local result = {}
	for _, status in ipairs(statuses_) do
		for _, icon in ipairs(self.icons_by_xy[status] or self.icons_by_xy.dirty) do
			if #icon.str > 0 and not inserted[icon] then
				table.insert(result, icon)
				inserted[icon] = true
			end
		end
	end

	table.sort(result, function(a, b)
		return a.ord < b.ord
	end)

	return result
end

NestedGit.is_dirty = dirty

return NestedGit
