local status_ok, configs = pcall(require, "nvim-treesitter.configs")
if not status_ok then
    return
end

configs.setup({

    -- Auto install
    ensure_installed = "all",
    sync_install = false,

    -- ┌┬┐┌─┐┌┬┐┬ ┬┬  ┌─┐┌─┐
    -- ││││ │ │││ ││  ├┤ └─┐
    -- ┴ ┴└─┘─┴┘└─┘┴─┘└─┘└─┘

    -- Consistent syntax highlighting.
    highlight = {
        enable = true, -- false will disable the whole extension
        disable = { "" }, -- list of language that will be disabled
        additional_vim_regex_highlighting = false,
    },

    -- Incremental selection based on the named nodes from the grammar.
    incremental_selection = {
        enable = true,
        keymaps = {
            init_selection = "gn", -- normal: start incremental selection.
            node_incremental = "gn", -- visual: increment to the upper named parent.
            scope_incremental = "gy", -- visual: increment to the upper scope
            node_decremental = "gp", -- visual: decrement to the previous named node.
        },
    },

    -- Playground
    playground = {
        enable = true,
        disable = {},
        updatetime = 25, -- Debounced time for highlighting nodes in the playground from source code
        persist_queries = false, -- Whether the query persists across vim sessions
        keybindings = {
            toggle_query_editor = "o",
            toggle_hl_groups = "i",
            toggle_injected_languages = "t",
            toggle_anonymous_nodes = "a",
            toggle_language_display = "I",
            focus_language = "f",
            unfocus_language = "F",
            update = "R",
            goto_node = "<CR>",
            show_help = "?",
        },
    },

    indent = {
        enable = true
    },

    context_commentstring = {
        enable = true,
        enable_autocmd = false, -- integration with Comment.nvim
    },

    autopairs = { enable = true },

    matchup = {
        enable = true,
        disable = {},
        -- [options] -- TODO: learn to use this module beyond basics
    },
})

local tsht_ok, tsht = pcall(require, "tsht")
if not tsht_ok then
    return
end

vim.cmd([[
omap     <silent> <leader><C-v> :<C-U>lua require('tsht').nodes()<CR>
vnoremap <silent> <leader><C-v> :lua require('tsht').nodes()<CR>
nnoremap <silent> <leader><C-v> v :lua require('tsht').nodes()<CR>
]])

tsht.config.hint_keys = { "a", "s", "e", "t", "n", "i", "o", "l" }
