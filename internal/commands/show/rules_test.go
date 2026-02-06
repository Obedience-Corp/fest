package show

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunRulesDisplaysContent(t *testing.T) {
	// Create a temp festival with FESTIVAL_RULES.md
	festDir := setupTempFestival(t)

	rulesContent := "---\ntitle: Rules\n---\n# Do not skip tests\n\nAlways run the full suite.\n"
	if err := os.WriteFile(filepath.Join(festDir, "FESTIVAL_RULES.md"), []byte(rulesContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Run from inside the festival
	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(festDir)

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	err := runRules(cmd)
	if err != nil {
		t.Fatalf("runRules returned error: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Fatal("expected output, got empty string")
	}
	if !bytes.Contains([]byte(output), []byte("Do not skip tests")) {
		t.Errorf("expected rules content in output, got: %s", output)
	}
	// Frontmatter should be stripped
	if bytes.Contains([]byte(output), []byte("title: Rules")) {
		t.Errorf("frontmatter should be stripped, got: %s", output)
	}
}

func TestRunRulesErrorsWhenNoFile(t *testing.T) {
	festDir := setupTempFestival(t)

	origDir, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(origDir) })
	os.Chdir(festDir)

	cmd := &cobra.Command{}
	cmd.SetOut(&bytes.Buffer{})

	err := runRules(cmd)
	if err == nil {
		t.Fatal("expected error when FESTIVAL_RULES.md is missing")
	}
}

func TestStripRulesFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with frontmatter",
			input:    "---\ntitle: Rules\n---\n# Body\nContent here.",
			expected: "# Body\nContent here.",
		},
		{
			name:     "no frontmatter",
			input:    "# Just content\nNo frontmatter.",
			expected: "# Just content\nNo frontmatter.",
		},
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripRulesFrontmatter(tc.input)
			if got != tc.expected {
				t.Errorf("stripRulesFrontmatter(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

// setupTempFestival creates a minimal festival directory structure for testing.
// Includes fest.yaml so ResolveFestivalPath can locate the root.
func setupTempFestival(t *testing.T) string {
	t.Helper()
	// Structure: tmpdir/festivals/active/test-fest/
	root := t.TempDir()
	festDir := filepath.Join(root, "festivals", "active", "test-fest")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	// fest.yaml is required for ResolveFestivalPath detection
	if err := os.WriteFile(filepath.Join(festDir, "fest.yaml"), []byte("name: test-fest\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return festDir
}
