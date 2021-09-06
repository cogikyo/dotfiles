" " ╦  ╔═╗╔═╗┌─┐┌─┐┌┐┌┌─┐┬┌─┐
" " ║  ╚═╗╠═╝│  │ ││││├┤ ││ ┬
" " ╩═╝╚═╝╩  └─┘└─┘┘└┘└  ┴└─┘
" " https://github.com/neovim/nvim-lspconfig
" " https://github.com/neovim/nvim-lspconfig/blob/master/CONFIG.md

" lua << EOF
" --python
" require'lspconfig'.pyright.setup{}

" --html/css
" local capabilities = vim.lsp.protocol.make_client_capabilities()
" capabilities.textDocument.completion.completionItem.snippetSupport = true

" require'lspconfig'.html.setup { capabilities = capabilities }
" require'lspconfig'.cssls.setup { capabilities = capabilities }

" --vim
" require'lspconfig'.vimls.setup{}

" --typescript/javascript
" require'lspconfig'.tsserver.setup{}

" --tex
" -- require'lspconfig'.texlab.setup{
" --     cmd = { "texlab" },
" --     filetypes = { "tex", "bib" },
" --     settings = {
" --       texlab = {
" --         auxDirectory = ".",
" --         bibtexFormatter = "texlab",
" --         build = {
" --           args = { "-pdflatex=lualatex", "-pdf", "-interaction=nonstopmode", "-synctex=1", "%f" },
" --           executable = "latexmk",
" --           forwardSearchAfter = false,
" --           onSave = false
" --         },
" --         chktex = {
" --           onEdit = false,
" --           onOpenAndSave = true
" --         },
" --         diagnosticsDelay = 300,
" --         formatterLineLength = 80,
" --         forwardSearch = {
" --           args = {}
" --         },
" --         latexFormatter = "latexindent",
" --         latexindent = {
" --           modifyLineBreaks = false
" --         }
" --       }
" --     }
" -- }

" --bash
" require'lspconfig'.bashls.setup{}

" --go
" require'lspconfig'.gopls.setup{}
" EOF

" let g:completion_matching_strategy_list = ['exact','substring', 'fuzzy']
" let g:completion_trigger_keyword_length = 2 " default = 1
" let g:completion_enable_snippet = 'UltiSnips'
