// Package shell provides the fest CLI's shell integration scripts, kept as
// embedded shell files (see scripts/) so they can be linted and tested as shell
// code rather than Go string literals.
package shell

import (
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// SupportedShells lists the shells with integration support.
var SupportedShells = []string{"zsh", "bash", "fish", "sh"}

// Generate returns the shell integration script for the given shell.
//
// bash and zsh share the POSIX core with sh and add their own completion
// machinery on top; the shells differ only in what they can hook, not in what
// fgo, fls, and fest do.
func Generate(shellType string) (string, error) {
	switch shellType {
	case "zsh", "bash":
		return posixCoreScript + "\n" + bashZshCompletionsScript, nil
	case "sh":
		return posixCoreScript, nil
	case "fish":
		return fishScript, nil
	default:
		return "", errors.Validation("unsupported shell - supported: "+strings.Join(SupportedShells, ", ")).
			WithField("shell", shellType)
	}
}

// IsSupported reports whether the shell type has integration support.
func IsSupported(shellType string) bool {
	for _, s := range SupportedShells {
		if s == shellType {
			return true
		}
	}
	return false
}
