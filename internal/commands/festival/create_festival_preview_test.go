package festival

import (
	"strings"
	"testing"
)

func TestRenderFestivalPreviewTree(t *testing.T) {
	t.Parallel()

	tree := renderFestivalPreviewTree("example-EX0001", []festivalPreviewEntry{
		{path: "FESTIVAL_GOAL.md"},
		{path: "001_INGEST", isDir: true},
		{path: "001_INGEST/PHASE_GOAL.md"},
		{path: "001_INGEST/input_specs", isDir: true},
		{path: "001_INGEST/input_specs/seed.md"},
	})

	for _, expected := range []string{
		"example-EX0001/",
		"├── 001_INGEST/",
		"│   ├── PHASE_GOAL.md",
		"│   └── input_specs/",
		"│       └── seed.md",
		"└── FESTIVAL_GOAL.md",
	} {
		if !strings.Contains(tree, expected) {
			t.Fatalf("preview tree missing %q:\n%s", expected, tree)
		}
	}
}
