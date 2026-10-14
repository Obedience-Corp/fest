package dirlink

import (
	"path/filepath"
	"strings"
)

type targetClass int

const (
	targetLocal targetClass = iota
	targetNetwork
)

// classifyAbsTarget decides whether an absolute path is local or network/UNC.
func classifyAbsTarget(absTarget string) targetClass {
	if isUNCPath(absTarget) {
		return targetNetwork
	}
	return targetLocal
}

func isUNCPath(p string) bool {
	p = filepath.Clean(p)
	if strings.HasPrefix(p, `\\`) {
		return true
	}
	// Extended-length UNC prefix.
	if strings.HasPrefix(strings.ToLower(p), `\\?\unc\`) {
		return true
	}
	return false
}
