package yamlutil

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
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

func TestQuoteString_RoundTripsAsSingleLineScalar(t *testing.T) {
	tests := []string{
		`say "hi" \ and: # x`,
		"two\nlines\tand a tab",
		"Ünïcode & <b>",
		"",
		"null",
		"123",
		"- dash: start",
		strings.Repeat("long word ", 30),
	}
	for _, value := range tests {
		t.Run(value, func(t *testing.T) {
			quoted, err := QuoteString(value)
			if err != nil {
				t.Fatalf("QuoteString(%q): %v", value, err)
			}
			if strings.Contains(quoted, "\n") {
				t.Fatalf("QuoteString(%q) spans lines: %q", value, quoted)
			}

			var doc struct {
				Value string `yaml:"value"`
			}
			if err := yaml.Unmarshal([]byte("value: "+quoted+"\n"), &doc); err != nil {
				t.Fatalf("unmarshal %q: %v", quoted, err)
			}
			if doc.Value != value {
				t.Fatalf("round trip: got %q, want %q", doc.Value, value)
			}
		})
	}
}
