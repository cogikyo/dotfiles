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

" ╔═╗┌┬┐┌┬┐┌─┐
" ║  │││ ││└─┐
" ╚═╝┴ ┴─┴┘└─┘
augroup AUTO_COMMANDS
    autocmd!
    autocmd BufWritePre * :call TrimWhitespace()
    autocmd BufEnter * set scrolloff=19
    " autocmd FileType html,css EmmetInstall
    " autocmd BufEnter * lua require'completion'.on_attach()
    autocmd TermOpen * setlocal nonumber norelativenumber signcolumn=no
augroup END

augroup SPELLING
    autocmd!
    autocmd FileType latex,tex,md,markdown,python,cpp setlocal spell spelllang=en_us
augroup END

augroup COMMENTARY
    autocmd!
    autocmd FileType c,cpp,cs,java setlocal commentstring=//\ %s
augroup END

augroup PYTHON
    autocmd!
    autocmd FileType python setlocal tabstop=4 softtabstop=4
    autocmd FileType python setlocal shiftwidth=4
    autocmd FileTYpe python nnoremap <buffer> <leader>rr :w<CR>:exec '!python3' shellescape(@%, 1)<CR>
augroup END

augroup HIGHLIGHT_YANK
    autocmd!
    autocmd TextYankPost * silent! lua require'vim.highlight'.on_yank({timeout = 69})
augroup END
