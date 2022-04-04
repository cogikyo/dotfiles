function! TrimWhitespace()
  let l:save = winsaveview()
  keeppatterns %s/\s\+$//e
  call winrestview(l:save)
endfun

function! EmptyReg()
  let regs=split('abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789/-"', '\zs')
  for r in regs
    call setreg(r, [])
  endfor
endfun

function! ToggleSpellCheck()
  set spell!
  if &spell
    echo "Spellcheck ON"
  else
    echo "Spellcheck OFF"
  endif
endfunction

function! ToggleWrap()
 if (&wrap == 1)
   set nowrap
   echo "Wrap OFF"
 else
   set wrap
   echo "Wrap ON"
 endif
endfunction

augroup AUTO_COMMANDS
  autocmd!
  autocmd BufWritePre * :call TrimWhitespace()
  autocmd TermOpen * setlocal nonumber norelativenumber signcolumn=no nospell
  autocmd FileType help setlocal cc=
  autocmd TextYankPost * silent! lua require'vim.highlight'.on_yank({timeout = 69})
  autocmd CursorHold * :echo
augroup END

" some random fix, not sure what for.
let g:switch_mapping = ""
