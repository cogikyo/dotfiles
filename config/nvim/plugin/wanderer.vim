lua require 'colorizer'.setup()

let s:_white   = '#d3d5df'
let s:_black_D = '#090b18'
let s:_black   = '#1b1d27'
let s:_grey_D  = '#1e2132'
let s:_grey_d  = '#282b3c'
let s:_grey    = '#3d3f52'
let s:_grey_l  = '#454B68'
let s:_grey_L  = '#6c749d'
let s:_red     = '#e67979'
let s:_orange  = '#e6a579'
let s:_yellow  = '#d3bf6f'
let s:_green   = '#94b47e'
let s:_cyan    = '#7fb5b1'
let s:_azure   = '#b4e0f8'
let s:_blue    = '#77a2cf'
let s:_purple  = '#a98fd1'
let s:_pink    = '#e46c9c'

let s:p = {'normal': {}, 'inactive': {}, 'insert': {}, 'replace': {}, 'visual': {}, 'tabline': {}}

let s:p.normal.left     = [ [s:_black, s:_blue, 'bold'   ], [s:_blue, s:_grey_d   ] ]
let s:p.insert.left     = [ [s:_black, s:_azure, 'bold'  ], [s:_azure, s:_grey_d  ] ]
let s:p.visual.left     = [ [s:_black, s:_green, 'bold'  ], [s:_green,  s:_grey_d ] ]
let s:p.replace.left    = [ [s:_black, s:_purple, 'bold' ], [s:_purple, s:_grey_d ] ]
let s:p.inactive.left   = [ [s:_black, s:_pink, 'bold'   ], [s:_pink, s:_grey_d   ] ]

let s:p.normal.middle   = [ [s:_white, s:_grey_D ] ]

let s:p.normal.right    = [ [s:_black, s:_blue   ], [s:_blue,  s:_grey_d  ] ]
let s:p.insert.right    = [ [s:_black, s:_azure  ], [s:_azure, s:_grey_d ] ]
let s:p.visual.right    = [ [s:_black, s:_green  ], [s:_green, s:_grey_d ] ]
let s:p.replace.right   = [ [s:_black, s:_purple ], [s:_green, s:_grey_d ] ]
let s:p.inactive.right  = [ [s:_black, s:_pink   ], [s:_pink, s:_grey_d  ] ]

let s:p.normal.error    = [ [s:_red, s:_grey_D    ] ]
let s:p.normal.warning  = [ [s:_yellow, s:_grey_D ] ]

let g:lightline#colorscheme#wanderer#palette = lightline#colorscheme#fill(s:p)

" ⚓ lightline
let g:lightline = {
\ 'separator': { 'left': '', 'right': '' },
\ 'subseparator': { 'left': '⊸', 'right': ''},
\ 'colorscheme': 'wanderer',
\ 'active': {
\   'left': [ [ 'mode', ],
\             [ 'fugitive', 'filename', 'modified',] ],
\   'right': [ [ 'filetype' ],
\              [ 'readonly'],
\              [ 'syntastic'] ]
\ },
\ 'inactive': {
\   'left': [ [ 'relativepath' ],
\             [ 'modified' ] ],
\   'right': [ [ 'filetype' ],
\              [ 'readonly'] ]
\ },
\ 'component_function': {
\   'fugitive': 'LightlineFugitive',
\   'filename': 'LightlineFilename',
\   'readonly': 'LightlineReadonly',
\   'modified': 'LightlineModified',
\ },
\ 'component_expand': {
\   'syntastic': 'SyntasticStatuslineFlag',
\   'filename': 'LightlineFilename',
\ },
\ 'component_type': {
\   'syntastic': 'error',
\ },
\ }

let g:lightline.mode_map = {
\ 'n' : '⟂',
\ 'i' : '⊢',
\ 'R' : '﯒',
\ 'v' : '∋',
\ 'V' : 'λ',
\ "\<C-v>": 'β',
\ 'c' : '',
\ }

fun! LightlineFugitive()
    try
	    if expand('%:t') !~? 'Tagbar\|Gundo\|NERD' && &ft !~? 'vimfiler' && exists('*FugitiveHead')
	        let mark = ' '   " edit here for cool mark
            let branch = FugitiveHead()
	        return branch !=# '' ? mark.branch : ''
	    endif
    catch
    endtry
    return ''
endfun

fun! LightlineFilename()
	return expand('%:t') !=# '' ? expand('%:t') : '名前?'
endfun

fun! LightlineReadonly()
	return &readonly ? '' : ''
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
