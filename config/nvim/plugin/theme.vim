" ╔╦╗┬ ┬┌─┐┌┬┐┌─┐
"  ║ ├─┤├┤ │││├┤
"  ╩ ┴ ┴└─┘┴ ┴└─┘
set background=dark
colorscheme iceberg

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
  let col_edge     = ['#c6c8d1', '#3d435c', 234, 245]
  let col_gradient = ['#c6c8d1', '#2e3244', 234, 241]
  let col_nc       = ['#c6c8d1', '#0f1117', 238, 233]
  let col_tabfill  = ['#c6c8d1', '#34394e', 243, 237]
  let col_normal   = ['#c6c8d1', '#454b68', 234, 245]
  let col_error    = ['#161821', '#e27878', 234, 203]
  let col_warning  = ['#161821', '#e2a478', 234, 216]
  let col_insert   = ['#161821', '#84a0c6', 234, 110]
  let col_replace  = ['#161821', '#e2a478', 234, 216]
  let col_visual   = ['#161821', '#b4be82', 234, 150]
  let col_tabsel   = ['#161821', '#818596', 234, 245]

  let p.normal.middle = [
        \ col_base]
  let p.normal.left = [
        \ col_normal,
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
hi SpellCap guifg=#f0c674
hi SpllRare guifg=#e278ad
hi SpellLocal guifg=#e278ad
