" ╔╦╗┬─┐┌─┐┌─┐┌─┐┬┌┬┐┌┬┐┌─┐┬─ac┐
"  ║ ├┬┘├┤ ├┤ └─┐│ │  │ ├┤ ├┬┘
"  ╩ ┴└─└─┘└─┘└─┘┴ ┴  ┴ └─┘┴└─
lua << EOF
require'nvim-treesitter.configs'.setup { indent = { enable = true }, highlight = { enable = true }, incremental_selection = { enable = true }, textobjects = { enable = true }}
EOF

nnoremap <leader>pg :TSPlaygroundToggle<CR>
