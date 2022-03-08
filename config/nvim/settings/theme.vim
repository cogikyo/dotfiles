" ████████╗██╗  ██╗███████╗███╗   ███╗███████╗
" ╚══██╔══╝██║  ██║██╔════╝████╗ ████║██╔════╝
"    ██║   ███████║█████╗  ██╔████╔██║█████╗
"    ██║   ██╔══██║██╔══╝  ██║╚██╔╝██║██╔══╝
"    ██║   ██║  ██║███████╗██║ ╚═╝ ██║███████╗
"    ╚═╝   ╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝╚══════╝

" ┬  ┬┌─┐┬ ┬┌┬┐┬  ┬┌┐┌┌─┐
" │  ││ ┬├─┤ │ │  ││││├┤
" ┴─┘┴└─┘┴ ┴ ┴ ┴─┘┴┘└┘└─┘
let s:save_cpo = &cpo
set cpo&vim

function! s:build_palette() abort
  let p = {
        \ 'normal':   {},
        \ 'inactive': {},
        \ 'insert':   {},
        \ 'replace':  {},
        \ 'visual':   {},
        \ 'tabline':  {}}

  let col_base     = ['#c6c8d1', '#1e2132', 243, 237]
  let col_edge     = ['#91acd1', '#454b68', 243, 237]
  let col_gradient = ['#89b8c2', '#2e3244', 234, 241]
  let col_nc       = ['#c6c8d1', '#0f1117', 238, 233]
  let col_tabfill  = ['#c6c8d1', '#34394e', 243, 237]
  let col_normal   = ['#c6c8d1', '#454b68', 234, 245]
  let col_error    = ['#161821', '#e27878', 234, 203]
  let col_warning  = ['#161821', '#e2a478', 234, 216]
  let col_insert   = ['#161821', '#84a0c6', 234, 110]
  let col_replace  = ['#161821', '#a376b8', 234, 216]
  let col_visual   = ['#161821', '#b4be82', 234, 150]
  let col_tabsel   = ['#161821', '#818596', 234, 245]

  let p.normal.middle = [
        \ col_base]
  let p.normal.left = [
        \ col_edge,
        \ col_gradient]
  let p.normal.right = [
        \ col_edge,
        \ col_gradient]
  let p.normal.error = [
        \ col_error]
  let p.normal.warning = [
        \ col_warning]

  let p.insert.left = [
        \ col_insert,
        \ col_gradient]
  let p.replace.left = [
        \ col_replace,
        \ col_gradient]
  let p.visual.left = [
        \ col_visual,
        \ col_gradient]

  let p.inactive.middle = [
        \ col_nc]
  let p.inactive.left = [
        \ col_nc,
        \ col_nc]
  let p.inactive.right = [
        \ col_nc,
        \ col_nc]

  let p.tabline.middle = [
        \ col_tabfill]
  let p.tabline.left = [
        \ col_tabfill]
  let p.tabline.tabsel = [
        \ col_tabsel]

  let p.tabline.right = copy(p.normal.right)

  return p
endfunction

let g:lightline#colorscheme#iceberg#palette = s:build_palette()

let &cpo = s:save_cpo
unlet s:save_cpo

" ⚓ lightline
let g:lightline = {
\ 'separator': { 'left': '', 'right': '' },
\ 'subseparator': { 'left': '╼╾', 'right': '╼╾'},
\ 'colorscheme': 'iceberg',
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

let g:lightline.mode_map = {
\ 'n' : 'normal ⟂',
\ 'i' : 'insert ⊢',
\ 'R' : 'replace ⧟',
\ 'v' : 'visual ⏿',
\ 'V' : 'v-line λ',
\ "\<C-v>": 'v-block β',
\ 'c' : 'command Ξ',
\ 's' : 'select',
\ 'S' : 's-line',
\ "\<C-s>": 's-block',
\ 't': 'terminal',
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

" ┌─┐┌┐┌┌─┐┌┬┐┌─┐┬─┐┬┌─  ┬ ┬┌─┐┌─┐┬┌─
" │ ││││├┤  ││├─┤├┬┘├┴┐  ├─┤├─┤│  ├┴┐
" └─┘┘└┘└─┘─┴┘┴ ┴┴└─┴ ┴  ┴ ┴┴ ┴└─┘┴ ┴

set background=dark
colorscheme onedark

" Doesn't actually apply, since the onedark plugin runs after... but used here
" for easier editing.
let s:overrides = get(g:, "onedark_color_overrides", {})
let s:colors = {
      \ "red": get(s:overrides, "red", { "gui": "#e4a97c", "cterm": "204", "cterm16": "1" }),
      \ "dark_red": get(s:overrides, "dark_red", { "gui": "#de9463", "cterm": "196", "cterm16": "9" }),
      \ "green": get(s:overrides, "green", { "gui": "#95d18f", "cterm": "114", "cterm16": "2" }),
      \ "yellow": get(s:overrides, "yellow", { "gui": "#e9dc89", "cterm": "180", "cterm16": "3" }),
      \ "dark_yellow": get(s:overrides, "dark_yellow", { "gui": "#e27878", "cterm": "173", "cterm16": "11" }),
      \ "blue": get(s:overrides, "blue", { "gui": "#7abef5", "cterm": "39", "cterm16": "4" }),
      \ "purple": get(s:overrides, "purple", { "gui": "#a376b8", "cterm": "170", "cterm16": "5" }),
      \ "cyan": get(s:overrides, "cyan", { "gui": "#7bd4b8", "cterm": "38", "cterm16": "6" }),
      \ "white": get(s:overrides, "white", { "gui": "#c6c8d1", "cterm": "145", "cterm16": "15" }),
      \ "black": get(s:overrides, "black", { "gui": "#161821", "cterm": "235", "cterm16": "0" }),
      \ "foreground": get(s:overrides, "foreground", { "gui": "#c6c8d1", "cterm": "145", "cterm16": "NONE" }),
      \ "background": get(s:overrides, "background", { "gui": "#161821", "cterm": "235", "cterm16": "NONE" }),
      \ "comment_grey": get(s:overrides, "comment_grey", { "gui": "#7B839E", "cterm": "59", "cterm16": "7" }),
      \ "gutter_fg_grey": get(s:overrides, "gutter_fg_grey", { "gui": "#484e6b", "cterm": "238", "cterm16": "8" }),
      \ "cursor_grey": get(s:overrides, "cursor_grey", { "gui": "#1E2132", "cterm": "236", "cterm16": "0" }),
      \ "visual_grey": get(s:overrides, "visual_grey", { "gui": "#3E496B", "cterm": "237", "cterm16": "8" }),
      \ "menu_grey": get(s:overrides, "menu_grey", { "gui": "#7b839e", "cterm": "237", "cterm16": "7" }),
      \ "special_grey": get(s:overrides, "special_grey", { "gui": "#7b839e", "cterm": "238", "cterm16": "7" }),
      \ "vertsplit": get(s:overrides, "vertsplit", { "gui": "#1E2132", "cterm": "59", "cterm16": "7" }),
      \}
