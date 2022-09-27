local null_ls_status_ok, null_ls = pcall(require, "null-ls")
if not null_ls_status_ok then
    return
end

local code_actions = null_ls.builtins.code_actions
local diagnostics = null_ls.builtins.diagnostics
local formatting = null_ls.builtins.formatting
local hover = null_ls.builtins.hover

null_ls.setup({
    debug = false,
    sources = {
        formatting.prettier.with({
            extra_args = {
                "--single-quote",
                "--jsx-single-quote",
                "--print-width 80",
            },
        }),
        formatting.black.with({
            extra_args = { "--fast" },
        }),
        formatting.shellharden,

        diagnostics.flake8,
        diagnostics.shellcheck,
        diagnostics.eslint,
        diagnostics.jsonlint,
        diagnostics.luacheck.with({
            extra_args = { "--globals" },
        }),

        code_actions.shellcheck,
        code_actions.eslint,

        hover.dictionary,
    },
})
