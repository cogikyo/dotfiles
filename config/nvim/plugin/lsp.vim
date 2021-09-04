" ╦  ╔═╗╔═╗┌─┐┌─┐┌┐┌┌─┐┬┌─┐
" ║  ╚═╗╠═╝│  │ ││││├┤ ││ ┬
" ╩═╝╚═╝╩  └─┘└─┘┘└┘└  ┴└─┘
" https://github.com/neovim/nvim-lspconfig
" https://github.com/neovim/nvim-lspconfig/blob/master/CONFIG.md
"


lua << EOF
--python
require'lspconfig'.pyright.setup{}

--html/css
local capabilities = vim.lsp.protocol.make_client_capabilities()
capabilities.textDocument.completion.completionItem.snippetSupport = true
require'lspconfig'.html.setup { capabilities = capabilities }
require'lspconfig'.cssls.setup { capabilities = capabilities }

--vim
require'lspconfig'.vimls.setup{}

--typescript/javascript
require'lspconfig'.tsserver.setup{}

--tex
require'lspconfig'.texlab.setup{}

--bash
require'lspconfig'.bashls.setup{}

--go
require'lspconfig'.gopls.setup{}
EOF

" autocompletion
set completeopt=menuone,noselect
set shortmess+=c

let g:completion_matching_strategy_list = ['exact','substring', 'fuzzy']
let g:completion_trigger_keyword_length = 3 " default = 1

