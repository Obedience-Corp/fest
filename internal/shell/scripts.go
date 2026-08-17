package shell

import _ "embed"

// posixCoreScript holds the integration that is valid POSIX sh: the guard, the
// fgo/fls/fest wrappers, and their helpers. It is the entire script for sh and
// the shared base for bash and zsh, so a change to a wrapper reaches every
// Bourne-family shell at once instead of being ported by hand.
//
//go:embed scripts/posix_core.sh
var posixCoreScript string

// bashZshCompletionsScript holds the bash/zsh-only completion machinery.
//
//go:embed scripts/bash_zsh_completions.sh
var bashZshCompletionsScript string

//go:embed scripts/fish.sh
var fishScript string
