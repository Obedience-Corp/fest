package yamlutil

import (
	"strings"
	"testing"
)

func TestMarshal_TwoSpaceIndent(t *testing.T) {
	data := map[string]any{
		"parent": map[string]any{
			"child": "value",
		},
	}

	out, err := Marshal(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	s := string(out)

	// Must use 2-space indent, not 4
	if !strings.Contains(s, "  child: value") {
		t.Errorf("expected 2-space indent, got:\n%s", s)
	}
	if strings.Contains(s, "    child") {
		t.Errorf("found 4-space indent (should be 2):\n%s", s)
	}
}
