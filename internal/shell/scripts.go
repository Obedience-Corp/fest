package shell

import _ "embed"

//go:embed scripts/bash_zsh.sh
var bashZshScript string

//go:embed scripts/fish.sh
var fishScript string
