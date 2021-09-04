" ╔╦╗┬ ┬┌─┐┌┬┐┌─┐
"  ║ ├─┤├┤ │││├┤
"  ╩ ┴ ┴└─┘┴ ┴└─┘
colorscheme iceberg

" colorizer
let g:colorizer_auto_filetype='vim,css,scss,html,tex,txt,cls,sty,rasi,conf'
let g:colorizer_colornames = 0
nnoremap <leader>ct :ColorToggle<CR>

" ╦  ┬┌─┐┬ ┬┌┬┐┬  ┌┐┌┌─┐
" ║  ││ ┬├─┤ │ │  │││├┤
" ╩═╝┴└─┘┴ ┴ ┴ ┴─┘┘└┘└─┘
let g:lightline = {
\ 'separator': { 'left': '', 'right': '' },
\ 'subseparator': { 'left': '╼╾', 'right': '╼╾'},
\ 'colorscheme': 'icebergDark',
\ 'active': {
\   'left': [ [ 'mode', 'paste' ],
\             [ 'fugitive', 'readonly', 'filename', 'modified' ] ],
\   'right': [ [ 'lineinfo' ],
\              [ 'percent' ],
\              [ 'bufnum', ] ]
\ },
\ 'component_function': {
\   'fugitive': 'LightlineFugitive',
\   'filename': 'LightlineFilename',
\   'readonly': 'LightlineReadonly',
\   'modified': 'LightlineModified',
\ },
\ }

" this is fun
let g:lightline.mode_map = {
\ 'n' : '正常',
\ 'i' : '入れる',
\ 'R' : '交換',
\ 'v' : '見る',
\ 'V' : '行-見る',
\ "\<C-v>": '区画-見る',
\ 'c' : '令',
\ 's' : '撰ぶ',
\ 'S' : '行-選ぶ',
\ "\<C-s>": '区画-撰ぶ',
\ 't': 'ターミナル',
\ }

function! LightlineFugitive()
    try
	    if expand('%:t') !~? 'Tagbar\|Gundo\|NERD' && &ft !~? 'vimfiler' && exists('*FugitiveHead')
	        let mark = ' '   " edit here for cool mark
            let branch = FugitiveHead()
	        return branch !=# '' ? mark.branch : ''
	    endif
    catch
    endtry
    return ''
endfunction

function! LightlineFilename()
	return &ft ==# 'vimfiler' ? vimfiler#get_status_string() :
	      \  &ft ==# 'unite' ? unite#get_status_string() :
	      \ expand('%:t') !=# '' ? expand('%:t') : '名前?'
endfunction

function! LightlineReadonly()
	return &ft !~? 'help\|vimfiler' && &readonly ? '' : ''
endfunction

function! LightlineModified()
	return &modifiable && &modified ? '' : ''
endfunction

function! SyntasticCheckHook(errors)
    call lightline#update()
endfunction

" ╔╦╗┌─┐┬  ┬┬┌─┐┌─┐┌┐┌┌─┐
"  ║║├┤ └┐┌┘││  │ ││││└─┐
" ═╩╝└─┘ └┘ ┴└─┘└─┘┘└┘└─┘
lua << EOF
require'nvim-web-devicons'.setup {
 default = true;
}
EOF
