local M = {}

local api, fn = vim.api, vim.fn
local ns = api.nvim_create_namespace("gitsigns_commit")
local augroup = api.nvim_create_augroup("GitsignsCommit", { clear = true })
local diff_opts = { result_type = "indices", indent_heuristic = true }
local dismiss = { ["\27"] = true, ["\3"] = true }
local state = nil
local serial = 0

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ git: history reads for the blamed commit                                    │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

function M.git(root, args, stdin)
	local result = vim.system(vim.list_extend({ "git", "-C", root }, args), { stdin = stdin }):wait()
	if result.code ~= 0 then
		local err = vim.trim(result.stderr or "")
		return nil, err ~= "" and vim.split(err, "\n")[1] or ("git exited " .. result.code)
	end
	return result.stdout
end

local git = M.git

local function buf_path(buf)
	local name = api.nvim_buf_get_name(buf or 0)
	return name ~= "" and vim.uv.fs_realpath(name) or name
end

local function cursor_line() return api.nvim_win_get_cursor(0)[1] end
local function echo(text) api.nvim_echo({ { text } }, false, {}) end

local function strip_cr(lines)
	for index, line in ipairs(lines) do
		lines[index] = line:gsub("\r$", "")
	end
	return lines
end

local function text_lines(text)
	local lines = vim.split((text:gsub("^\239\187\191", "")), "\n", { plain = true })
	if lines[#lines] == "" then
		table.remove(lines)
	end
	return strip_cr(lines)
end

local function buf_lines(buf) return strip_cr(api.nvim_buf_get_lines(buf, 0, -1, false)) end
local function join_lines(lines) return #lines > 0 and table.concat(lines, "\n") .. "\n" or "" end

local function buf_contents(buf)
	local bo = vim.bo[buf]
	local eol = ({ dos = "\r\n", mac = "\r" })[bo.fileformat] or "\n"
	local text = table.concat(api.nvim_buf_get_lines(buf, 0, -1, false), eol)
	return (bo.bomb and "\239\187\191" or "") .. text .. (bo.eol and eol or "")
end

local function blame(root)
	local path = buf_path()
	local rel = vim.fs.relpath(root, path)
	if not rel then
		return nil, "Not in a git repository"
	end

	local line = cursor_line()
	local args = { "-c", "core.quotePath=false", "blame", "--porcelain", "-L", line .. "," .. line }
	local out, err = git(root, vim.list_extend(args, { "--contents", "-", "--", rel }), buf_contents(0))
	if not out then
		return nil, err
	end

	local sha = out:match("^%x+")
	if sha:match("^0+$") then
		return nil, "Not committed yet"
	end

	return {
		sha = sha,
		subject = out:match("\nsummary ([^\n]*)") or "",
		filename = out:match("\nfilename ([^\n]*)"),
		path = path,
		boundary = out:find("\nboundary\n", 1, true) ~= nil,
	}
end

local function diff_base(root, blamed)
	if not blamed.boundary then
		return blamed.sha .. "^"
	end

	local raw, err = git(root, { "cat-file", "commit", blamed.sha })
	if not raw then
		return nil, err
	end
	if (raw:match("^(.-)\n\n") or raw):find("\nparent ", 1, true) then
		return nil, "History before " .. blamed.sha:sub(1, 7) .. " is missing (shallow clone?)"
	end
	return "--root"
end

local function changed_files(root, base, sha)
	local out, err = git(root, { "diff-tree", "-r", "-M", "-z", "--no-commit-id", "--diff-filter=d", base, sha })
	if not out then
		return nil, err
	end

	local fields = vim.split(out, "\0", { plain = true })
	local changes = {}
	local index = 1
	while fields[index] and fields[index] ~= "" do
		local old_mode, new_mode, old_id, new_id, status = fields[index]:match("^:(%d+) (%d+) (%x+) (%x+) (%u)")
		local change = { old = fields[index + 1], new = fields[index + 1], old_id = old_id, new_id = new_id }
		index = index + 2
		if status == "R" then
			change.new = fields[index]
			index = index + 1
		elseif status == "A" then
			change.old, change.old_id = nil, nil
		end
		if old_mode ~= "160000" and new_mode ~= "160000" then
			table.insert(changes, change)
		end
	end
	return changes
end

local function read_blobs(root, ids)
	if #ids == 0 then
		return {}
	end

	local out, err = git(root, { "cat-file", "--batch" }, table.concat(ids, "\n") .. "\n")
	if not out then
		return nil, err
	end

	local blobs, pos = {}, 1
	for _, id in ipairs(ids) do
		local header_end = out:find("\n", pos, true)
		local header = header_end and out:sub(pos, header_end - 1) or id .. " truncated"
		local kind, size = header:match("^%x+ (%a+) (%d+)$")
		if not size then
			return nil, "git cat-file: " .. header
		end

		pos = header_end + 1
		blobs[id] = kind == "blob" and out:sub(pos, pos + tonumber(size) - 1)
		pos = pos + tonumber(size) + 1
	end
	return blobs
end

local function current_names(root, sha, changes)
	local missing = vim.iter(changes):any(function(change) return not vim.uv.fs_stat(root .. "/" .. change.new) end)
	if not missing then
		return {}
	end

	local out = git(root, { "diff-tree", "-r", "-M", "-z", "--name-status", "--diff-filter=R", sha, "HEAD" }) or ""
	local fields = vim.split(out, "\0", { plain = true })
	local names = {}
	for index = 1, #fields - 1, 3 do
		names[fields[index + 1]] = fields[index + 2]
	end
	return names
end

local function disk_lines(path)
	local file = io.open(path, "rb")
	if not file then
		return nil
	end
	local text = file:read("*a")
	file:close()
	return text and text_lines(text)
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ mapping: the commit's hunks onto the current text                           │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

local function line_map(from, to)
	local map = {}
	local a, b = 1, 1
	for _, hunk in ipairs(vim.text.diff(join_lines(from), join_lines(to), diff_opts)) do
		local last = hunk[2] > 0 and hunk[1] - 1 or hunk[1]
		while a <= last do
			map[a], a, b = b, a + 1, b + 1
		end
		a, b = a + hunk[2], b + hunk[4]
	end
	while a <= #from do
		map[a], a, b = b, a + 1, b + 1
	end
	return map
end

local function surviving(map, from, to, step)
	for line = from, to, step do
		if map[line] then
			return map[line]
		end
	end
	return nil
end

local function file_hunks(file, old, new)
	local map = line_map(new, file.lines)
	local hunks = {}

	for _, indices in ipairs(vim.text.diff(join_lines(old), join_lines(new), diff_opts)) do
		local old_start, old_count, new_start, new_count = unpack(indices)
		local hunk = {
			file = file,
			old_start = old_start,
			new_start = new_start,
			removed = vim.list_slice(old, old_start, old_start + old_count - 1),
			added = vim.list_slice(new, new_start, new_start + new_count - 1),
			rows = {},
			above = true,
		}

		for line = new_start, new_start + new_count - 1 do
			if map[line] then
				table.insert(hunk.rows, map[line])
			end
		end

		if new_count == 0 then
			-- new_start is the line above the deletion (0 at BOF); below anchors virt_lines under it.
			local after = surviving(map, new_start + 1, #new, 1)
			local below = map[new_start] or (not after and surviving(map, new_start - 1, 1, -1)) or nil
			hunk.rows = { below or after or 1 }
			hunk.above = not below
		end

		if #hunk.rows > 0 then
			table.insert(hunks, hunk)
		end
	end

	return hunks
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ overlay: drawing, invalidation, and dismissal                               │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

local function draw(buf, file)
	api.nvim_buf_clear_namespace(buf, ns, 0, -1)
	local pad = string.rep(" ", vim.o.columns)

	for ordinal = file.first, file.last do
		local hunk = state.hunks[ordinal]
		if #hunk.added > 0 then
			local group = #hunk.removed == 0 and "GitSignsAddLn" or "GitSignsChangeLn"
			for _, row in ipairs(hunk.rows) do
				api.nvim_buf_set_extmark(buf, ns, row - 1, 0, { line_hl_group = group, strict = false })
			end
		end

		if #hunk.removed > 0 then
			local virt_lines = {}
			for _, text in ipairs(hunk.removed) do
				table.insert(virt_lines, { { text .. pad, "GitSignsDeleteVirtLn" } })
			end
			api.nvim_buf_set_extmark(buf, ns, hunk.rows[1] - 1, 0, {
				virt_lines = virt_lines,
				virt_lines_above = hunk.above,
				strict = false,
			})
		end
	end
end

function M.clear()
	if not state then
		return
	end

	state = nil
	vim.on_key(nil, ns)
	api.nvim_clear_autocmds({ group = augroup })
	for _, buf in ipairs(api.nvim_list_bufs()) do
		api.nvim_buf_clear_namespace(buf, ns, 0, -1)
	end
end

local function clear_later(id)
	vim.schedule(function()
		if state and state.id == id then
			M.clear()
		end
	end)
end

local function watch(buf)
	local owner = state
	if not owner or owner.attached[buf] or not api.nvim_buf_is_loaded(buf) or vim.bo[buf].buftype ~= "" then
		return
	end

	local file = owner.files[buf_path(buf)]
	if file then
		if not vim.deep_equal(buf_lines(buf), file.lines) then
			M.clear()
			echo(fn.fnamemodify(file.path, ":t") .. " changed; commit overlay cleared")
			return
		end
		draw(buf, file)
	end

	local id, attached = owner.id, owner.attached
	attached[buf] = true
	api.nvim_buf_attach(buf, false, {
		on_lines = function()
			clear_later(id)
			return true
		end,
		on_reload = function() clear_later(id) end,
		on_detach = function() attached[buf] = nil end,
	})
end

local function load_changes(root, blamed)
	local base, err = diff_base(root, blamed)
	if not base then
		return nil, nil, err
	end

	local changes
	changes, err = changed_files(root, base, blamed.sha)
	if not changes then
		return nil, nil, err
	end

	local ids = {}
	for _, change in ipairs(changes) do
		if change.old_id then
			table.insert(ids, change.old_id)
		end
		table.insert(ids, change.new_id)
	end

	local blobs
	blobs, err = read_blobs(root, ids)
	if not blobs then
		return nil, nil, err
	end
	return changes, blobs
end

local function activate(root, blamed)
	local sha = blamed.sha
	local changes, blobs, err = load_changes(root, blamed)
	if not changes then
		vim.notify(sha:sub(1, 7) .. ": " .. err, vim.log.levels.WARN)
		return
	end

	local bufs = {}
	for _, buf in ipairs(api.nvim_list_bufs()) do
		if api.nvim_buf_is_loaded(buf) then
			bufs[buf_path(buf)] = buf
		end
	end

	local names = current_names(root, sha, changes)
	local found = {}
	for _, change in ipairs(changes) do
		local old, new = change.old_id and blobs[change.old_id] or "", blobs[change.new_id]
		local path = change.new == blamed.filename and blamed.path or root .. "/" .. (names[change.new] or change.new)
		local lines = bufs[path] and buf_lines(bufs[path]) or disk_lines(path)
		if old and new and lines and not (old .. new):find("\0", 1, true) then
			local file = { path = path, old = change.old, new = change.new, lines = lines }
			local hunks = file_hunks(file, text_lines(old), text_lines(new))
			if #hunks > 0 then
				table.insert(found, { file = file, hunks = hunks })
			end
		end
	end

	if #found == 0 then
		echo(sha:sub(1, 7) .. " has no surviving changes")
		return
	end

	table.sort(found, function(a, b) return a.file.path < b.file.path end)
	serial = serial + 1
	state = { id = serial, root = root, sha = sha, subject = blamed.subject, files = {}, hunks = {}, attached = {} }
	for _, entry in ipairs(found) do
		entry.file.first = #state.hunks + 1
		vim.list_extend(state.hunks, entry.hunks)
		entry.file.last = #state.hunks
		state.files[entry.file.path] = entry.file
	end

	for _, buf in ipairs(api.nvim_list_bufs()) do
		watch(buf)
	end
	api.nvim_create_autocmd({ "BufReadPost", "BufEnter" }, {
		group = augroup,
		callback = function(args) watch(args.buf) end,
	})

	local owner = state
	vim.on_key(function(key, typed)
		if (dismiss[key] or dismiss[typed]) and api.nvim_get_mode().mode == "n" then
			clear_later(owner.id)
		end
	end, ns)
end

-- ╭─────────────────────────────────────────────────────────────────────────────╮
-- │ navigation: walk the commit's hunks across its files                        │
-- ╰─────────────────────────────────────────────────────────────────────────────╯

local function echo_position(ordinal)
	local head, tail = state.sha:sub(1, 7) .. " ", (" %d/%d"):format(ordinal, #state.hunks)
	local room = vim.v.echospace - #head - #tail
	local subject = state.subject
	if fn.strdisplaywidth(subject) > room then
		subject = fn.strcharpart(subject, 0, math.max(room - 1, 0)) .. "…"
	end
	echo(head .. subject .. tail)
end

local function land(ordinal, move)
	local owner = state
	local hunk = owner.hunks[ordinal]
	if buf_path() ~= hunk.file.path then
		local ok, err = pcall(vim.cmd, "edit " .. fn.fnameescape(hunk.file.path))
		if not ok then
			echo(tostring(err):match("E%d+:.*") or tostring(err))
			return
		end
		if state ~= owner then
			return
		end
	end

	if move ~= false then
		api.nvim_win_set_cursor(0, { hunk.rows[1], 0 })
	end
	owner.landing = { buf = api.nvim_get_current_buf(), line = cursor_line(), ordinal = ordinal }
	echo_position(ordinal)
end

local function wrap(ordinal) return (ordinal - 1) % #state.hunks + 1 end

local function landed()
	local landing = state.landing
	if landing and landing.buf == api.nvim_get_current_buf() and landing.line == cursor_line() then
		return landing
	end
	return nil
end

local function from_cursor(delta)
	local file = state.files[buf_path()]
	if not file then
		return delta == 1 and 1 or #state.hunks
	end

	local first, last = file.first, file.last
	if delta < 0 then
		first, last = last, first
	end
	local line = cursor_line()
	for ordinal = first, last, delta do
		if (state.hunks[ordinal].rows[1] - line) * delta > 0 then
			return ordinal
		end
	end
	return wrap(last + delta)
end

local function hunk_at_cursor()
	local landing = landed()
	if landing then
		return landing.ordinal
	end

	local file = state.files[buf_path()]
	if not file then
		return nil
	end

	local line = cursor_line()
	for ordinal = file.first, file.last do
		if vim.list_contains(state.hunks[ordinal].rows, line) then
			return ordinal
		end
	end
	return nil
end

local function step(direction)
	local delta = direction == "next" and 1 or -1
	local landing = landed()
	land(landing and wrap(landing.ordinal + delta) or from_cursor(delta))
end

function M.nav(direction, root)
	-- Step without re-blaming while landed: a pure-deletion anchor line may belong to another commit.
	if state and landed() then
		step(direction)
		return
	end

	local blamed, err = blame(root)
	if not blamed then
		echo(err)
		return
	end
	if state and state.root == root and state.sha == blamed.sha then
		step(direction)
		return
	end

	M.clear()
	activate(root, blamed)
	if not state then
		return
	end

	local ordinal = hunk_at_cursor()
	if ordinal then
		land(ordinal, false)
	else
		step(direction)
	end
end

function M.advance(direction)
	if not state then
		return false
	end
	step(direction)
	return true
end

function M.patch()
	local ordinal = state and hunk_at_cursor()
	if not ordinal then
		return nil
	end

	local hunk = state.hunks[ordinal]
	local file = hunk.file
	local function range(start, count) return count == 1 and tostring(start) or start .. "," .. count end
	local lines = {
		"--- " .. (file.old and "a/" .. file.old or "/dev/null"),
		"+++ b/" .. file.new,
		("@@ -%s +%s @@"):format(range(hunk.old_start, #hunk.removed), range(hunk.new_start, #hunk.added)),
	}
	for _, line in ipairs(hunk.removed) do
		table.insert(lines, "-" .. line)
	end
	for _, line in ipairs(hunk.added) do
		table.insert(lines, "+" .. line)
	end
	return table.concat(lines, "\n"), state.sha:sub(1, 7)
end

return M
