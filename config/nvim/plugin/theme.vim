" ╔╦╗┬ ┬┌─┐┌┬┐┌─┐
"  ║ ├─┤├┤ │││├┤
"  ╩ ┴ ┴└─┘┴ ┴└─┘
colorscheme iceberg

" ⚓ lightline
let g:lightline = {
\ 'separator': { 'left': '', 'right': '' },
\ 'subseparator': { 'left': '╼╾', 'right': '╼╾'},
\ 'colorscheme': 'icebergDark',
\ 'active': {
\   'left': [ [ 'mode', 'paste' ],
\             [ 'fugitive', 'readonly', 'filename', 'modified' ] ],
\   'right': [ [ 'filetype' ],
\              [ 'charvalue' ],
\              [ 'syntastic'] ]
\ },
\ 'component_function': {
\   'fugitive': 'LightlineFugitive',
\   'filename': 'LightlineFilename',
\   'readonly': 'LightlineReadonly',
\   'modified': 'LightlineModified',
\ },
\ 'component_expand': {
\   'syntastic': 'SyntasticStatuslineFlag',
\ },
\ 'component_type': {
\   'syntastic': 'error',
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

fun! LightlineFugitive()
    try
	    if expand('%:t') !~? 'Tagbar\|Gundo\|NERD' && &ft !~? 'vimfiler' && exists('*FugitiveHead')
	        let mark = ' '   " edit here for cool mark
            let branch = FugitiveHead()
	        return branch !=# '' ? mark.branch : ''
	    endif
    catch
    endtry
    return ''
endfun

fun! LightlineFilename()
	return &ft ==# 'vimfiler' ? vimfiler#get_status_string() :
	      \  &ft ==# 'unite' ? unite#get_status_string() :
	      \ expand('%:t') !=# '' ? expand('%:t') : '名前?'
endfun

fun! LightlineReadonly()
	return &ft !~? 'help\|vimfiler' && &readonly ? '' : ''
endfun

fun! LightlineModified()
	return &modifiable && &modified ? '' : ''
endfun

fun! SyntasticCheckHook(errors)
    call lightline#update()
endfun

lua << EOF
require'nvim-web-devicons'.setup {
 default = true;
}
EOF

hi clear SpellBad
hi clear SpellCap
hi SpellBad guifg=#e27878
hi SpellCap guifg=#e278ad
hi SpllRare guifg=#e278ad
hi SpellLocal guifg=#e278ad
