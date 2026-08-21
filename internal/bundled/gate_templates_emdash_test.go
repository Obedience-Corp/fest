package bundled

import (
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/methodology"
)

const emDash = '\u2014'

func TestIsShippedGateTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "empty path", path: "", want: false},
		{name: "workflow is not a gate template", path: ".festival/templates/phases/planning/WORKFLOW.md", want: false},
		{name: "readme is not a gate template", path: ".festival/templates/phases/planning/plan/README.md", want: false},
		{name: "phase GATES.md", path: ".festival/templates/phases/implementation/GATES.md", want: true},
		{name: "QUALITY_GATE_FEST_COMMIT.md", path: ".festival/templates/phases/implementation/gates/QUALITY_GATE_FEST_COMMIT.md", want: true},
		{name: "other QUALITY_GATE file", path: ".festival/templates/phases/implementation/gates/QUALITY_GATE_TESTING.md", want: true},
		{name: "generated fest_commit task", path: "active/demo/001_IMPLEMENT/01_core/11_fest_commit.md", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := isShippedGateTemplate(tt.path); got != tt.want {
				t.Fatalf("isShippedGateTemplate(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestShippedGateTemplatesContainNoEmDash(t *testing.T) {
	t.Parallel()

	var checked int
	var foundFestCommit bool
	err := fs.WalkDir(methodology.FS, methodology.Root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !isShippedGateTemplate(p) {
			return nil
		}

		data, err := methodology.FS.ReadFile(p)
		if err != nil {
			return err
		}
		checked++
		if path.Base(p) == "QUALITY_GATE_FEST_COMMIT.md" {
			foundFestCommit = true
		}

		text := string(data)
		if i := strings.IndexRune(text, emDash); i >= 0 {
			rel := strings.TrimPrefix(p, methodology.Root+"/")
			line := 1 + strings.Count(text[:i], "\n")
			t.Errorf("%s:%d contains U+2014; new festivals copy this file and fail the no-em-dash rule", rel, line)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded methodology: %v", err)
	}
	if checked == 0 {
		t.Fatal("no gate templates found in the bundled methodology; the check would be vacuous")
	}
	if !foundFestCommit {
		t.Fatal("QUALITY_GATE_FEST_COMMIT.md missing from bundled methodology")
	}
}

func isShippedGateTemplate(p string) bool {
	base := path.Base(p)
	switch {
	case base == "GATES.md":
		return true
	case strings.HasPrefix(base, "QUALITY_GATE_") && strings.HasSuffix(base, ".md"):
		return true
	case strings.HasSuffix(base, "_fest_commit.md"):
		return true
	default:
		return false
	}
}
