package frontmatter

import (
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
)

// UpdateFields replaces the values of existing top-level keys inside a
// document's frontmatter block, leaving every other byte alone: unknown
// keys, comments, ordering, and the body all survive untouched. Keys not
// present in the block are not added; content without a frontmatter block
// is returned unchanged. The boolean reports whether anything changed.
//
// Use this for targeted single-field updates where preserving the file's
// existing formatting matters; use Parse/Inject for full rewrites.
func UpdateFields(content []byte, fields map[string]string) ([]byte, bool) {
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return content, false
	}
	changed := false
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" {
			break
		}
		for key, value := range fields {
			if strings.HasPrefix(trimmed, key+":") {
				lines[i] = key + ": " + value
				changed = true
			}
		}
	}
	if !changed {
		return content, false
	}
	return []byte(strings.Join(lines, "\n")), true
}

// UpdateFieldsInFile applies UpdateFields to a file in place, preserving
// its mode. Files without frontmatter or without any of the keys are left
// untouched.
func UpdateFieldsInFile(path string, fields map[string]string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return errors.IO("frontmatter.UpdateFieldsInFile.read", err).WithField("path", path)
	}
	updated, changed := UpdateFields(content, fields)
	if !changed {
		return nil
	}
	mode := os.FileMode(0644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode()
	}
	if err := os.WriteFile(path, updated, mode); err != nil {
		return errors.IO("frontmatter.UpdateFieldsInFile.write", err).WithField("path", path)
	}
	return nil
}
