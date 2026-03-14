package selection

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// ExtractWorkingDir reads fest_working_dir from the sequence GOAL frontmatter.
// Returns empty string if the GOAL doesn't exist or has no working dir set.
func ExtractWorkingDir(sequencePath string) string {
	goalPath := filepath.Join(sequencePath, "SEQUENCE_GOAL.md")
	content, err := os.ReadFile(goalPath)
	if err != nil {
		return ""
	}
	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		return ""
	}
	return strings.TrimSpace(fm.WorkingDir)
}
