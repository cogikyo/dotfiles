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
    autocmd TermOpen * setlocal nonu nornu scl=no nospell
    autocmd FileType * setlocal formatoptions="carqnpj"
    autocmd FileType spectre setlocal scl=no nospell cc= nonu nornu nocul
    autocmd FileType help setlocal cc=
    autocmd TextYankPost * silent! lua require'vim.highlight'.on_yank({timeout = 69})
    autocmd CursorHold * :echo
augroup END

augroup TEMP
    autocmd!

    " for waybar config
    autocmd BufWritePost config.jsonc execute 'silent !' 'killall waybar && exec waybar </dev/null &>/dev/null & disown'
    autocmd BufWritePost style.scss execute 'silent !' 'sleep 1 && killall waybar && exec waybar </dev/null &>/dev/null & disown'
augroup END

augroup TYPESCRIPT
    autocmd!
    autocmd FileType jsonc,typescript,javascript,html,javascriptreact,typescriptreact,css,scss setlocal nospell shiftwidth=2 softtabstop=2 tabstop=2
augroup END

" some random fix, not sure what for.
let g:switch_mapping = ""
