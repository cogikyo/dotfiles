local M = {}

local q = xplr.util.shell_quote

local archive_extensions = {
	"tar.gz",
	"tar.bz2",
	"tar.xz",
	"tar.zst",
	"tar.lz4",
	"tar.lzma",
	"tar.lz",
	"tar.br",
	"zip",
	"7z",
	"rar",
	"tar",
	"tgz",
	"tbz2",
	"txz",
	"gz",
	"bz2",
	"xz",
	"zst",
}

local archive_mimes = {
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

local function has_archive_extension(path)
	local lower = path:lower()
	for _, extension in ipairs(archive_extensions) do
		if lower:sub(-#extension - 1) == "." .. extension then
			return true
		end
	end
	return false
end

local function is_archive(node)
	return node and node.is_file and (archive_mimes[node.mime_essence or ""] or has_archive_extension(node.absolute_path))
end

local function is_media(node)
	local mime = node and node.mime_essence or ""
	return mime:match("^image/") or mime:match("^video/")
end

local function archive_stem(path)
	local name = path:match("([^/]+)$") or path
	for _, extension in ipairs(archive_extensions) do
		local suffix = "." .. extension
		if name:lower():sub(-#suffix) == suffix then
			return name:sub(1, #name - #suffix)
		end
	end
	return name:gsub("%.[^.]+$", "")
end

local function focused_archive(ctx)
	local node = ctx.focused_node
	if not is_archive(node) then
		return nil
	end
	return node.absolute_path
end

function M.open(ctx)
	if is_archive(ctx.focused_node) then
		return { { SwitchModeCustom = "archive" } }
	end
	return xplr.fn.custom.nuke_open(ctx) or {}
end

function M.enter(ctx)
	if is_archive(ctx.focused_node) then
		return { { SwitchModeCustom = "archive" } }
	end
	if is_media(ctx.focused_node) then
		return xplr.fn.custom.nuke_open(ctx) or {}
	end
	return { "PrintResultAndQuit" }
end

function M.list(ctx)
	local path = focused_archive(ctx)
	if not path then
		return { { LogError = "focus an archive first" }, "PopMode" }
	end

	return {
		{ BashExec = "ouch list --tree -- " .. q(path) .. " | nvimpager" },
		"PopMode",
	}
end

function M.extract(ctx)
	local path = focused_archive(ctx)
	if not path then
		return { { LogError = "focus an archive first" }, "PopMode" }
	end

	local stem = archive_stem(path)
	return {
		{
			BashExec = string.format(
				[===[
src=%s
dest=%s
if [ -e "$dest" ]; then
  i=1
  while [ -e "$dest-$i" ]; do
    i=$((i + 1))
  done
  dest="$dest-$i"
fi

if ouch d --yes --dir "$dest" -- "$src"; then
  "$XPLR" -m ExplorePwd -m 'FocusPath: %%q' "$dest"
fi

printf '\n[press enter to continue]'
read -r _
]===],
				q(path),
				q(stem)
			),
		},
		"PopMode",
	}
end

function M.extract_to(ctx)
	local path = focused_archive(ctx)
	local target = ctx.input_buffer
	if not path then
		return { { LogError = "focus an archive first" }, "PopMode" }
	end
	if not target or target == "" then
		return { { LogError = "enter an output directory" } }
	end

	return {
		{
			BashExec = string.format(
				[===[
src=%s
dest=%s
if ouch d --dir "$dest" -- "$src"; then
  "$XPLR" -m ExplorePwd -m 'FocusPath: %%q' "$dest"
fi

printf '\n[press enter to continue]'
read -r _
]===],
				q(path),
				q(target)
			),
		},
		"PopMode",
		"PopMode",
	}
end

function M.setup()
	xplr.fn.custom.archive_open = M.open
	xplr.fn.custom.archive_enter = M.enter
	xplr.fn.custom.archive_list = M.list
	xplr.fn.custom.archive_extract = M.extract
	xplr.fn.custom.archive_extract_to = M.extract_to

	xplr.config.modes.custom.archive = {
		name = "archive",
		layout = "HelpMenu",
		key_bindings = {
			on_key = {
				l = {
					help = "list",
					messages = { { CallLua = "custom.archive_list" } },
				},
				d = {
					help = "decompress",
					messages = { { CallLua = "custom.archive_extract" } },
				},
				t = {
					help = "decompress to",
					messages = {
						{ SwitchModeCustom = "archive extract to" },
						{ SetInputBuffer = "" },
					},
				},
				esc = {
					help = "cancel",
					messages = { "PopMode" },
				},
			},
		},
	}

	xplr.config.modes.custom["archive extract to"] = {
		name = "archive extract to",
		key_bindings = {
			on_key = {
				enter = {
					help = "decompress to target",
					messages = { { CallLua = "custom.archive_extract_to" } },
				},
				backspace = {
					help = "remove last character",
					messages = { "RemoveInputBufferLastCharacter" },
				},
				["ctrl-u"] = {
					help = "clear input",
					messages = { { SetInputBuffer = "" } },
				},
				esc = {
					help = "cancel",
					messages = { "PopMode" },
				},
			},
			default = {
				messages = { "BufferInputFromKey" },
			},
		},
	}
end

return M
