local status_ok, lualine = pcall(require, "lualine")
if not status_ok then
    return
end

local colors = require("tokyonight.colors").setup({})
local utils = require("tokyonight.util")
local fg_gutter_alt = "#2a2f46"

local tokyonight = {

    normal = {
        a = { bg = colors.blue, fg = colors.black },
        b = { bg = fg_gutter_alt, fg = colors.blue },
        c = { bg = colors.bg_statusline, fg = colors.blue7 },
    },
    insert = {
        a = { bg = colors.green, fg = colors.black },
        b = { bg = fg_gutter_alt, fg = colors.green },
    },
    command = {
        a = { bg = colors.yellow, fg = colors.black },
        b = { bg = fg_gutter_alt, fg = colors.yellow },
    },
    visual = {
        a = { bg = colors.magenta, fg = colors.black },
        b = { bg = fg_gutter_alt, fg = colors.magenta },
    },
    replace = {
        a = { bg = colors.red, fg = colors.black },
        b = { bg = fg_gutter_alt, fg = colors.red },
    },
    inactive = {
        a = { bg = colors.bg_statusline, fg = colors.blue },
        b = { bg = colors.bg_statusline, fg = fg_gutter_alt, gui = "bold" },
        c = { bg = colors.bg_statusline, fg = fg_gutter_alt },
    },
}

local hide_in_width = function()
    return vim.fn.winwidth(0) > 80
end

local branch = {
    "branch",
    icon = "",
}

-- see lualine.nvim/lua/lualine/utils/mode.lua for mode names
local modes = {
    ["NORMAL"] = "",
    ["INSERT"] = "",
    ["VISUAL"] = "",
    ["SELECT"] = "",
    ["V-LINE"] = "",
    ["V-BLOCK"] = "",
    ["COMMAND"] = "",
    ["REPLACE"] = "",
}

local mode = {
    "mode",
    fmt = function(str)
        if modes[str] then
            return modes[str]
        end
        return str
    end,
}

local diagnostics = {
    "diagnostics",
    sources = { "nvim_diagnostic" },
    sections = { "error", "warn", "hint", "info" },
    symbols = { error = " ", warn = " ", hint = " ", info = " " },
    colored = false,
    update_in_insert = false,
    padding = { left = 1, right = 1 },
    cond = hide_in_width,
}

local diff = {
    "diff",
    colored = true,
    symbols = { added = " ", modified = " ", removed = " " }, -- changes diff symbols
    cond = hide_in_width,
}

local filetype = {
    "filetype",
    colored = false,
    icon_only = true,
    padding = { left = 0, right = 1 },
}

local filename = {
    "filename",
    file_status = true,
    path = 0,
    icon = "",
    symbols = {
        modified = " 🞊",
        readonly = " ",
        unnamed = "--名前?--",
    },
    color = function()
        local mode_color = {
            n = utils.lighten(colors.blue, 0.75),
            i = utils.lighten(colors.green, 0.75),
            v = utils.lighten(colors.magenta, 0.75),
            V = utils.lighten(colors.magenta, 0.75),
            c = utils.lighten(colors.orange, 0.75),
            R = utils.lighten(colors.red, 0.75),
            s = utils.lighten(colors.teal, 0.75),
            S = utils.lighten(colors.teal, 0.75),
            [""] = utils.lighten(colors.magenta, 0.75),
        }
        return { fg = mode_color[vim.fn.mode()] }
    end,
}

local lsp = {
    function()
        local msg = "No Active Lsp"
        local buf_ft = vim.api.nvim_buf_get_option(0, "filetype")
        local clients = vim.lsp.get_active_clients()
        if next(clients) == nil then
            return msg
        end
        for _, client in ipairs(clients) do
            local filetypes = client.config.filetypes
            if filetypes and vim.fn.index(filetypes, buf_ft) ~= -1 then
                return client.name
            end
        end
        return msg
    end,
    icon = "",
    color = { gui = "italic" },
    cond = hide_in_width,
}

local buffers = {
    "buffers",
    max_length = vim.o.columns * 10 / 11,
    cond = hide_in_width,
}

local tsplayground = {
    sections = {
        lualine_b = { filename },
        lualine_z = { filetype },
    },
    inactive_sections = {
        lualine_z = { filetype },
    },
    filetypes = { "tsplayground" },
}

local spectre = {
    sections = {
        lualine_a = { mode },
        lualine_y = { filetype },
    },
    inactive_sections = {
        lualine_z = { filetype },
    },
    filetypes = { "spectre" },
}

local search = {
    function()
        local last_search = vim.fn.getreg("/")
        if not last_search or last_search == "" then
            return ""
        end
        local searchcount = vim.fn.searchcount({ maxcount = 9999 })
        if searchcount.total == 0 then
            vim.cmd([[ :call setreg("/", ['空'])<CR> ]])
        end
        return " " .. last_search .. "(" .. searchcount.current .. "/" .. searchcount.total .. ")"
    end,
    color = { fg = utils.darken(colors.orange, 0.25) },
}
lualine.setup({
    options = {
        icons_enabled = true,
        theme = tokyonight,
        section_separators = { left = "", right = "" },
        component_separators = { left = "⊸", right = "⟜" },
        disabled_filetypes = {},
        always_divide_middle = true,
        globalstatus = false,
    },
    sections = {
        lualine_a = { mode },
        lualine_b = { branch, filename },
        lualine_c = { diff, diagnostics },
        lualine_x = { lsp, search },
        lualine_y = { filetype },
        lualine_z = {},
    },
    inactive_sections = {
        lualine_a = { "" },
        lualine_b = { buffers },
        lualine_c = { "" },
        lualine_x = {},
        lualine_y = { "" },
        lualine_z = { "filetype" },
    },
    tabline = {},
    extensions = {
        "nvim-tree",
        "quickfix",
        "toggleterm",
        tsplayground,
        spectre,
    },
})
