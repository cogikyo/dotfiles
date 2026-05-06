local M = {}

local cache = {}
local displayed_image = false
local last_geometry = { x = 0, y = 0, width = 80, height = 20 }

local q = xplr.util.shell_quote
local preview_command = os.getenv("HOME") .. "/.config/xplr/bin/kitty-preview.py"

local archives = {
	["application/gzip"] = true,
	["application/java-archive"] = true,
	["application/vnd.debian.binary-package"] = true,
	["application/vnd.ms-cab-compressed"] = true,
	["application/x-7z-compressed"] = true,
	["application/x-alz-compressed"] = true,
	["application/x-archive"] = true,
	["application/x-arj"] = true,
	["application/x-bzip2"] = true,
	["application/x-compress"] = true,
	["application/x-cpio"] = true,
	["application/x-freearc"] = true,
	["application/x-gtar"] = true,
	["application/x-lzip"] = true,
	["application/x-lzma"] = true,
	["application/x-lzop"] = true,
	["application/x-rar"] = true,
	["application/x-rar-compressed"] = true,
	["application/x-rpm"] = true,
	["application/x-tar"] = true,
	["application/x-xz"] = true,
	["application/zip"] = true,
	["application/zstd"] = true,
}

local function run(command, args)
	local result = xplr.util.shell_execute(command, args)
	if result.returncode == 0 and result.stdout ~= "" then
		return result.stdout
	end
	return result.stderr or ""
end

local function sh(script, args)
	local command_args = { "-lc", script, "xplr-preview" }
	for _, arg in ipairs(args or {}) do
		table.insert(command_args, tostring(arg))
	end
	return run("bash", command_args)
end

local function dimensions(ctx)
	local width = math.max((ctx.layout_size and ctx.layout_size.width or 80) - 2, 20)
	local height = math.max((ctx.layout_size and ctx.layout_size.height or 20) - 2, 4)
	return width, height
end

local function image_dimensions(ctx)
	local width, height = dimensions(ctx)
	return width, height
end

local function async_shell(script)
	os.execute("bash -lc " .. q(script) .. " >/dev/null 2>&1 &")
end

local function focused_node(ctx)
	return (ctx.app and ctx.app.focused_node) or ctx.focused_node
end

local function mime(node)
	if node.mime_essence and node.mime_essence ~= "" then
		return node.mime_essence
	end
	return run("file", { "--brief", "--mime-type", node.absolute_path }):gsub("%s+$", "")
end

local function extension(node)
	return (node.extension or node.relative_path:match("%.([^.]+)$") or ""):lower()
end

local function cache_key(node, ctx)
	local width, height = dimensions(ctx)
	return table.concat({ node.absolute_path, node.last_modified or "", width, height }, "\0")
end

local function stats(node, detected_mime)
	local kind = detected_mime
	if node.is_dir then
		kind = "directory"
	elseif node.is_symlink then
		kind = node.is_broken and "broken symlink" or "symlink"
	end

	return table.concat({
		node.relative_path or node.absolute_path,
		"",
		"type     " .. (kind or "unknown"),
		"size     " .. (node.human_size or ""),
		"owner    " .. tostring(node.uid or "") .. ":" .. tostring(node.gid or ""),
		"modified " .. tostring(os.date("%Y-%m-%d %H:%M:%S", (node.last_modified or 0) / 1000000000)),
	}, "\n")
end

local function preview_directory(node, ctx)
	local _, height = dimensions(ctx)
	return sh("erd -L 2 -I -C none -- \"$1\" 2>/dev/null | head -n \"$2\"", {
		node.absolute_path,
		height,
	})
end

local function preview_image(node, detected_mime)
	return stats(node, detected_mime) .. "\n\npress I to draw image preview"
end

function M.clear_image(_)
	if not displayed_image then
		return {}
	end

	displayed_image = false
	async_shell("python3 " .. q(preview_command) .. " clear")
	return {}
end

local function preview_video(node, ctx)
	local width, height = image_dimensions(ctx)
	local thumb = "/tmp/xplr-preview-" .. tostring(node.absolute_path:gsub("[^%w_.-]", "_")) .. ".png"
	local info = sh("mediainfo --Inform='General;%Format% • %Duration/String% • %FileSize/String%' -- \"$1\" 2>/dev/null", {
		node.absolute_path,
	})
	if info ~= "" then
		return info .. "\n\npress I to draw video thumbnail"
	end
	return stats(node, "video")
end

local function preview_archive(node, ctx)
	local _, height = dimensions(ctx)
	return sh("ouch list --tree -- \"$1\" 2>/dev/null | head -n \"$2\"", {
		node.absolute_path,
		height,
	})
end

local function preview_pdf(node, ctx)
	local _, height = dimensions(ctx)
	return sh("pdftotext -f 1 -l 2 -layout -- \"$1\" - 2>/dev/null | head -n \"$2\"", {
		node.absolute_path,
		height,
	})
end

local function preview_json(node, ctx)
	local _, height = dimensions(ctx)
	return sh("jq -M . -- \"$1\" 2>/dev/null | head -n \"$2\"", {
		node.absolute_path,
		height,
	})
end

local function preview_text(node, ctx)
	local _, height = dimensions(ctx)
	local out = run("bat", {
		"--color=never",
		"--style=plain",
		"--line-range=1:" .. height,
		node.absolute_path,
	})
	if out ~= "" then
		return out
	end
	return run("head", { "-" .. height, node.absolute_path })
end

local function render_uncached(ctx)
	local node = focused_node(ctx)
	if not node then
		return ""
	end

	if node.is_dir then
		local out = preview_directory(node, ctx)
		return out ~= "" and out or stats(node, "directory")
	end

	local detected_mime = mime(node)
	local ext = extension(node)
	local out = ""
	if detected_mime:match("^image/") then
		out = preview_image(node, detected_mime)
	elseif detected_mime:match("^video/") then
		out = preview_video(node, ctx)
	elseif archives[detected_mime] then
		out = preview_archive(node, ctx)
	elseif detected_mime == "application/pdf" then
		out = preview_pdf(node, ctx)
	elseif detected_mime == "application/json" or ext == "json" then
		out = preview_json(node, ctx)
	elseif detected_mime:match("^text/") or ext == "md" or ext == "lua" or ext == "go" or ext == "sh" then
		out = preview_text(node, ctx)
	end

	if out ~= "" then
		return out
	end
	return stats(node, detected_mime)
end

function M.render(ctx)
	last_geometry = {
		x = math.max((ctx.layout_size and ctx.layout_size.x or 0) + 1, 0),
		y = math.max((ctx.layout_size and ctx.layout_size.y or 0) + 1, 0),
		width = math.max((ctx.layout_size and ctx.layout_size.width or 80) - 2, 20),
		height = math.max((ctx.layout_size and ctx.layout_size.height or 20) - 2, 4),
	}

	local node = focused_node(ctx)
	if not node then
		M.clear_image()
		return ""
	end

	local key = cache_key(node, ctx)
	if cache[key] == nil then
		cache[key] = render_uncached(ctx)
	end
	return cache[key]
end

function M.show_image(ctx)
	local node = focused_node(ctx)
	if not node or not node.is_file then
		return { { LogError = "focus an image or video first" } }
	end

	local detected_mime = mime(node)
	local path = node.absolute_path
	local image_id = 424242
	local script = nil
	if detected_mime:match("^image/") then
		script = string.format(
			"python3 %s display %s %d %d %d %d %d",
			q(preview_command),
			q(path),
			image_id,
			last_geometry.x,
			last_geometry.y,
			last_geometry.width,
			last_geometry.height
		)
	elseif detected_mime:match("^video/") then
		local thumb = "/tmp/xplr-preview-" .. tostring(path:gsub("[^%w_.-]", "_")) .. ".png"
		script = string.format(
			"ffmpegthumbnailer -i %s -o %s -s 0 -q 8 && python3 %s display %s %d %d %d %d %d",
			q(path),
			q(thumb),
			q(preview_command),
			q(thumb),
			image_id,
			last_geometry.x,
			last_geometry.y,
			last_geometry.width,
			last_geometry.height
		)
	else
		return { { LogError = "not an image or video" } }
	end

	displayed_image = true
	async_shell(script)
	return {}
end

function M.setup()
	xplr.fn.custom.preview = {
		render = M.render,
		clear_image = M.clear_image,
		show_image = M.show_image,
	}
end

return M
