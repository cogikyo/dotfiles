typeset -gA ZSH_HIGHLIGHT_STYLES

ZSH_HIGHLIGHT_HIGHLIGHTERS+=(regexp)

# https://github.com/zsh-users/zsh-syntax-highlighting/blob/master/docs/highlighters/main.md

# Other Plugins
ZSH_AUTOSUGGEST_HIGHLIGHT_STYLE="fg=#4e5492"

# Regex
ZSH_HIGHLIGHT_REGEXP+=('^trash .*' fg=#824141)
ZSH_HIGHLIGHT_REGEXP+=('^rm .*'    fg=#f36978)
ZSH_HIGHLIGHT_REGEXP+=('^rm -rf.*' fg=#f34658,bold)
ZSH_HIGHLIGHT_REGEXP+=('^touch' fg=#f5d599)
ZSH_HIGHLIGHT_REGEXP+=('^mkdir' fg=#f4ce88)
ZSH_HIGHLIGHT_REGEXP+=('^mv' fg=#75e6d5)
ZSH_HIGHLIGHT_REGEXP+=('^ln' fg=#38d2ba)
ZSH_HIGHLIGHT_REGEXP+=('^cp' fg=#2bb198)

# Main
ZSH_HIGHLIGHT_STYLES[default]='fg=#aeb9f8'
ZSH_HIGHLIGHT_STYLES[comment]='fg=#484e75'
ZSH_HIGHLIGHT_STYLES[unknown-token]='fg=#343852'

ZSH_HIGHLIGHT_STYLES[reserved-word]='fg=#a188df,bold'
ZSH_HIGHLIGHT_STYLES[commandseparator]='fg=#9376d8,bold'

ZSH_HIGHLIGHT_STYLES[function]='fg=#f2a170'
ZSH_HIGHLIGHT_STYLES[command]='fg=#f8b486'
ZSH_HIGHLIGHT_STYLES[arg0]='fg=#f8b486'
ZSH_HIGHLIGHT_STYLES[hashed-command]='fg=#f8b486'
ZSH_HIGHLIGHT_STYLES[precommand]='fg=#eb905d,bold'

ZSH_HIGHLIGHT_STYLES[alias]='fg=#ed9acd'
ZSH_HIGHLIGHT_STYLES[suffix-alias]='fg=#e887c3'
ZSH_HIGHLIGHT_STYLES[global-alias]='fg=#ea76c0,bold'

ZSH_HIGHLIGHT_STYLES[autodirectory]='fg=#8aa4f3,bold'
ZSH_HIGHLIGHT_STYLES[path]='fg=#7492ef'
ZSH_HIGHLIGHT_STYLES[path_pathseparator]='fg=#4a6be3'
ZSH_HIGHLIGHT_STYLES[path_prefix]='fg=#4a6be3,bold'

ZSH_HIGHLIGHT_STYLES[globbing]='fg=#5a61aa'
ZSH_HIGHLIGHT_STYLES[history-expansion]='fg=#5a61aa,bold'

ZSH_HIGHLIGHT_STYLES[command-substitution]='fg=#ea834b'
ZSH_HIGHLIGHT_STYLES[command-substitution-unquoted]='fg=#ea834b'
ZSH_HIGHLIGHT_STYLES[command-substitution-quoted]='fg=#ea834b,bold'
ZSH_HIGHLIGHT_STYLES[command-substitution-delimiter]='fg=#ea834b'
ZSH_HIGHLIGHT_STYLES[command-substitution-delimiter-unquoted]='fg=#ea834b'
ZSH_HIGHLIGHT_STYLES[command-substitution-delimiter-quoted]='fg=#ea834b,bold'

ZSH_HIGHLIGHT_STYLES[process-substitution]='fg=#54b0e2,bold'
ZSH_HIGHLIGHT_STYLES[process-substitution-delimiter]='fg=#54b0e2,bold'

ZSH_HIGHLIGHT_STYLES[arithmetic-expansion]='fg=#6bbdec,bold'

ZSH_HIGHLIGHT_STYLES[single-hyphen-option]='fg=#75e6d5'
ZSH_HIGHLIGHT_STYLES[double-hyphen-option]='fg=#38d2ba'

ZSH_HIGHLIGHT_STYLES[single-quoted-argument]='fg=#9fd883'
ZSH_HIGHLIGHT_STYLES[double-quoted-argument]='fg=#95cb79'
ZSH_HIGHLIGHT_STYLES[dollar-quoted-argument]='fg=#85ba6d'
ZSH_HIGHLIGHT_STYLES[back-quoted-argument]='fg=#73ad5a,bold'
ZSH_HIGHLIGHT_STYLES[back-quoted-argument-delimiter]='fg=#73ad5a,bold'

ZSH_HIGHLIGHT_STYLES[single-quoted-argument-unclosed]='fg=#505831'
ZSH_HIGHLIGHT_STYLES[double-quoted-argument-unclosed]='fg=#505831'
ZSH_HIGHLIGHT_STYLES[dollar-quoted-argument-unclosed]='fg=#505831'
ZSH_HIGHLIGHT_STYLES[back-quoted-argument-unclosed]='fg=#505831'

ZSH_HIGHLIGHT_STYLES[dollar-double-quoted-argument]='fg=#f5d599,bold'
ZSH_HIGHLIGHT_STYLES[back-double-quoted-argument]='fg=#375c47'
ZSH_HIGHLIGHT_STYLES[back-dollar-quoted-argument]='fg=#375c47'

ZSH_HIGHLIGHT_STYLES[assign]='fg=#6380ec'
ZSH_HIGHLIGHT_STYLES[redirection]='fg=#6380ec,bold'

ZSH_HIGHLIGHT_STYLES[named-fd]='fg=#4a6be3'
ZSH_HIGHLIGHT_STYLES[numeric-fd]='fg=#4a6be3'
