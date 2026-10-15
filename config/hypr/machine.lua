local fields = {}

local record = io.open("/etc/dctl/machine")
if record then
	for line in record:lines() do
		local key, value = line:match("^(%w+)=(.*)$")
		if key then
			fields[key] = value
		end
	end
	record:close()
end

local function cpulist(value)
	if value == nil then
		return false
	end
	if value == "" then
		return true
	end
	for item in (value .. ","):gmatch("([^,]*),") do
		if not (item:match("^%d+$") or item:match("^%d+%-%d+$")) then
			return false
		end
	end
	return true
end

if cpulist(fields.lane) and (fields.backlight == "yes" or fields.backlight == "no") then
	return { lane = fields.lane, backlight = fields.backlight }
end
return {}
