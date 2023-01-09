local status_ok, comment = pcall(require, "Comment")
if not status_ok then
    return
end

comment.setup {
    -- Add a space b/w comment and the line
    padding = true,

    -- Whether the cursor should stay at its position
    -- This only affects NORMAL mode mappings and doesn't work with dot-repeat
    sticky = true,

    -- Lines to be ignored while comment/uncomment.
    -- Could be a regex string or a function that returns a regex string.
    -- Example: Use '^$' to ignore empty lines
    ignore = nil,

    -- LHS of toggle mappings in NORMAL + VISUAL mode
    toggler = {
        line = 'gcc',
        block = 'gbc',
    },

    -- LHS of operator-pending mappings in NORMAL + VISUAL mode
    opleader = {
        line = 'gc',
        block = 'gb',
    },

    -- LHS of extra mappings
    extra = {
        -- Add comment on the line above
        above = 'gcO',
        -- Add comment on the line below
        below = 'gco',
        -- Add comment at the end of line
        eol = 'gcA',
    },

    pre_hook = function(ctx)
        -- Only calculate commentstring for tsx filetypes
        if vim.bo.filetype == 'typescriptreact' then
            local U = require('Comment.utils')

            -- Determine whether to use linewise or blockwise commentstring
            local type = ctx.ctype == U.ctype.line and '__default' or '__multiline'

            -- Determine the location where to calculate commentstring from
            local location = nil
            if ctx.ctype == U.ctype.block then
                location = require('ts_context_commentstring.utils').get_cursor_location()
            elseif ctx.cmotion == U.cmotion.v or ctx.cmotion == U.cmotion.V then
                location = require('ts_context_commentstring.utils').get_visual_start_location()
            end

            return require('ts_context_commentstring.internal').calculate_commentstring({
                key = type,
                location = location,
            })
        end
    end,

    -- Post-hook, called after commenting is done
    -- @type fun(ctx: Ctx)
    post_hook = nil,
}
