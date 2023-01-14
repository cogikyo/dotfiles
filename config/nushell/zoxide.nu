# Initialize hook to add new entries to the database.
if (not ($env | default false __zoxide_hooked | get __zoxide_hooked)) {
  let-env __zoxide_hooked = true
  let-env config = ($env | default {} config).config
  let-env config = ($env.config | default {} hooks)
  let-env config = ($env.config | update hooks ($env.config.hooks | default {} env_change))
  let-env config = ($env.config | update hooks.env_change ($env.config.hooks.env_change | default [] PWD))
  let-env config = ($env.config | update hooks.env_change.PWD ($env.config.hooks.env_change.PWD | append {|_, dir|
    zoxide add -- $dir
  }))
}

# Jump to a directory using only keywords.
def-env __zoxide_z [...rest:string] {
  # `z -` does not work yet, see https://github.com/nushell/nushell/issues/4769
  let arg0 = ($rest | append '~').0
  let path = if (($rest | length) <= 1) and ($arg0 == '-' or ($arg0 | path expand | path type) == dir) {
    $arg0
  } else {
    (zoxide query --exclude $env.PWD -- $rest | str trim -r -c "\n")
  }
  cd $path
}

# Jump to a directory using interactive search.
def-env __zoxide_zi  [...rest:string] {
  cd $'(zoxide query -i -- $rest | str trim -r -c "\n")'
}

alias t = __zoxide_z
alias ti = __zoxide_zi
