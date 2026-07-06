// Package shell provides the fest CLI's shell integration scripts, kept as
// embedded shell files (see scripts/) so they can be linted and tested as shell
// code rather than Go string literals.
package shell

import "github.com/Obedience-Corp/fest/internal/errors"

// SupportedShells lists the shells with integration support.
var SupportedShells = []string{"zsh", "bash", "fish"}

// Generate returns the shell integration script for the given shell. bash and
// zsh share one script that self-detects the running shell at runtime.
func Generate(shellType string) (string, error) {
	switch shellType {
	case "zsh", "bash":
		return bashZshScript, nil
	case "fish":
		return fishScript, nil
	default:
		return "", errors.Validation("unsupported shell - supported: zsh, bash, fish").WithField("shell", shellType)
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
