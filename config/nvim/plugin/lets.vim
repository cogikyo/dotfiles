" ╦  ┌─┐┌┬┐┌─┐
" ║  ├┤  │ └─┐
" ╩═╝└─┘ ┴ └─┘
"❕syntastic-lets
let g:syntastic_check_on_open = 1
let g:syntastic_check_on_wq = 0

" ⚙  misc-lets
" let g:user_emmet_install_global = 0
let g:languagetool_cmd='/usr/bin/languagetool'

" 🌈 colorizer-
let g:colorizer_colornames = 0

" 📓 latex
let g:tex_flavor='latex'
let g:vimtex_view_method='zathura'
let g:vimtex_quickfix_mode=0

let g:tex_conceal='abdmg'
let g:tex_superscripts= "[0-9a-zA-W.,:;+-<>/()=]"
let g:tex_subscripts= "[0-9aehijklmnoprstuvx,+-/().]"
let g:tex_conceal_frac=1

if empty(v:servername) && exists('*remote_startserver')
    call remote_startserver('VIM')
endif

" ✂  snippets/completions
let g:UltiSnipsExpandTrigger="<tab>"
let g:UltiSnipsJumpForwardTrigger="<tab>"
let g:UltiSnipsJumpBackwardTrigger="<s-tab>"
let g:UltiSnipsSnippetDirectories=[$HOME.'/.config/nvim/UltiSnips']


" let g:completion_matching_strategy_list = ['exact','substring', 'fuzzy']
" let g:completion_enable_snippet = 'UltiSnips'
