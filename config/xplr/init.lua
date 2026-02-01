---@diagnostic disable
version = "1.0.0"
local xplr = xplr
---@diagnostic enable

local home = os.getenv("HOME")
package.path = home
	.. "/.config/xplr/?.lua;"
	.. home
	.. "/.config/xplr/?/init.lua;"
	.. home
	.. "/.config/xplr/plugins/?/init.lua;"
	.. home
	.. "/.config/xplr/plugins/?.lua;"
	.. package.path

require("lib.helpers")
require("config.general")
require("config.node_types")
require("config.layouts")
require("config.modes")
require("plugins")
require("plugins.config")
