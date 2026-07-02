package frontmatter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateFields(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		fields      map[string]string
		wantChanged bool
		wantContain []string
		wantSame    bool
	}{
		{
			name:        "replaces existing keys only",
			content:     "---\nfest_id: 01_a.md\nfest_order: 1\ncustom: keep\n---\n\nbody fest_id: not-fm\n",
			fields:      map[string]string{"fest_id": "02_a.md", "fest_order": "2", "fest_parent": "new"},
			wantChanged: true,
			wantContain: []string{"fest_id: 02_a.md", "fest_order: 2", "custom: keep", "body fest_id: not-fm"},
		},
		{
			name:     "no frontmatter untouched",
			content:  "# Doc\n\nfest_id: not-frontmatter\n",
			fields:   map[string]string{"fest_id": "x"},
			wantSame: true,
		},
		{
			name:     "keys absent untouched",
			content:  "---\nfest_type: task\n---\n\nbody\n",
			fields:   map[string]string{"fest_id": "x"},
			wantSame: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := UpdateFields([]byte(tt.content), tt.fields)
			if changed != tt.wantChanged {
				t.Fatalf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if tt.wantSame && string(got) != tt.content {
				t.Fatalf("content changed:\n%s", got)
			}
			for _, want := range tt.wantContain {
				if !strings.Contains(string(got), want) {
					t.Errorf("missing %q in:\n%s", want, got)
				}
			}
			if tt.wantChanged && strings.Contains(string(got), "fest_parent: new") {
				t.Error("absent key was added")
			}
		})
	}
}

func TestUpdateFieldsInFile_PreservesMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "01_a.md")
	if err := os.WriteFile(path, []byte("---\nfest_id: 01_a.md\n---\n\nbody\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := UpdateFieldsInFile(path, map[string]string{"fest_id": "02_a.md"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("mode = %v, want 0600 preserved", info.Mode().Perm())
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "fest_id: 02_a.md") {
		t.Errorf("field not updated:\n%s", content)
	}
}

func TestRoundTripPreservesUnknownKeys(t *testing.T) {
	content := []byte("---\nfest_type: task\nfest_id: 01_a.md\nfest_status: pending\nfest_created: 2026-07-02T00:00:00Z\nmy_custom_field: precious\nanother.custom: also-precious\n---\n\n# Body\n")

	fm, body, err := Parse(content)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if fm.Extra["my_custom_field"] != "precious" {
		t.Fatalf("unknown key not captured: %#v", fm.Extra)
	}

	fm.Status = StatusCompleted
	out, err := Inject(body, fm)
	if err != nil {
		t.Fatalf("Inject: %v", err)
	}
	for _, want := range []string{"my_custom_field: precious", "another.custom: also-precious", "fest_status: completed"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("round-trip lost %q:\n%s", want, out)
		}
	}
}
