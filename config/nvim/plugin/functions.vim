" ╔═╗┬ ┬┌┐┌┌─┐┌┬┐┬┌─┐┌┐┌┌─┐
" ╠╣ │ │││││   │ ││ ││││└─┐
" ╚  └─┘┘└┘└─┘ ┴ ┴└─┘┘└┘└─┘
fun! TrimWhitespace()
  let l:save = winsaveview()
  keeppatterns %s/\s\+$//e
  call winrestview(l:save)
endfun

fun! EmptyReg()
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

" ╔═╗┌┬┐┌┬┐┌─┐
" ║  │││ ││└─┐
" ╚═╝┴ ┴─┴┘└─┘
augroup AUTO_COMMANDS
  autocmd!
  autocmd BufWritePre * :call TrimWhitespace()
  autocmd BufEnter * set scrolloff=19
  autocmd TermOpen * setlocal nonumber norelativenumber signcolumn=no
augroup END

augroup NMC2
  autocmd!
  autocmd BufEnter * call ncm2#enable_for_buffer()
  au User Ncm2Plugin call ncm2#register_source({
        \ 'name' : 'vimtex',
        \ 'priority': 1,
        \ 'subscope_enable': 1,
        \ 'complete_length': 1,
        \ 'scope': ['tex'],
        \ 'matcher': {'name': 'combine',
        \           'matchers': [
          \               {'name': 'abbrfuzzy', 'key': 'menu'},
          \               {'name': 'prefix', 'key': 'word'},
          \           ]},
          \ 'mark': 'tex',
          \ 'word_pattern': '\w+',
          \ 'complete_pattern': g:vimtex#re#ncm,
          \ 'on_complete': ['ncm2#on_complete#omni', 'vimtex#complete#omnifunc'],
          \ })
augroup END

augroup SPELLING
  autocmd!
  autocmd FileType latex,markdown setlocal spell spelllang=en_us
augroup END

augroup COMMENTARY
  autocmd!
  autocmd FileType c,cpp,cs,java setlocal commentstring=//\ %s
augroup END

augroup PYTHON
  autocmd!
  autocmd FileType python setlocal tabstop=4 softtabstop=4
  autocmd FileType python setlocal shiftwidth=4
  autocmd FileTYpe python nnoremap <buffer> <leader>rr :w<Cr> :exec '!python3' shellescape(@%, 1)<CR>
  autocmd FileType python nnoremap <leader>is :Isort<CR>
  autocmd BufWritePre *.py :call Black()
augroup END

augroup HIGHLIGHT_YANK
  autocmd!
  autocmd TextYankPost * silent! lua require'vim.highlight'.on_yank({timeout = 69})
augroup END
