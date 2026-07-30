package resident

import (
	"os"
	"path/filepath"
	"testing"
)

func writeMarker(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, MarkerFilename), []byte(body), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
}

func TestRead(t *testing.T) {
	tests := []struct {
		name      string
		body      string // "" means write no marker at all
		wantNil   bool
		wantErr   bool
		wantType  string
		wantID    string
		wantTitle string
	}{
		{
			name:    "absent marker is not an error",
			wantNil: true,
		},
		{
			name: "valid marker parses",
			body: "version: v1alpha8\nkind: workitem\nid: design-thing-2026-07-29\n" +
				"type: design\ntitle: Thing\nref: WI-abc123\n",
			wantType:  "design",
			wantID:    "design-thing-2026-07-29",
			wantTitle: "Thing",
		},
		{
			name:    "another kind is not a resident",
			body:    "version: v1alpha8\nkind: something-else\nid: x\ntype: design\n",
			wantNil: true,
		},
		{
			name:    "missing kind is not a resident",
			body:    "version: v1alpha8\nid: x\ntype: design\n",
			wantNil: true,
		},
		{
			// A newer camp schema must not break fest: camp owns the schema.
			name: "unknown fields are ignored",
			body: "version: v9alpha1\nkind: workitem\nid: design-future-1\ntype: design\n" +
				"title: Future\nsomething_new: true\nnested:\n  a: 1\n  b: [x, y]\n",
			wantType:  "design",
			wantID:    "design-future-1",
			wantTitle: "Future",
		},
		{
			name:    "malformed yaml is an error",
			body:    "kind: workitem\n\tbad indent: [\n",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.body != "" {
				writeMarker(t, dir, tc.body)
			}

			got, err := Read(dir)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got marker %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantNil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil, want a marker")
			}
			if got.Type != tc.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tc.wantType)
			}
			if got.ID != tc.wantID {
				t.Errorf("ID = %q, want %q", got.ID, tc.wantID)
			}
			if got.Title != tc.wantTitle {
				t.Errorf("Title = %q, want %q", got.Title, tc.wantTitle)
			}
		})
	}
}

func TestIsResident(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"no marker", "", false},
		{"workitem marker", "kind: workitem\nid: x\ntype: design\n", true},
		{"other kind", "kind: festival\nid: x\n", false},
		{"malformed reports false", "kind: workitem\n\tbad: [\n", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.body != "" {
				writeMarker(t, dir, tc.body)
			}
			if got := IsResident(dir); got != tc.want {
				t.Errorf("IsResident = %v, want %v", got, tc.want)
			}
		})
	}
}

// D007: the package must expose no way to write the marker.
func TestPackageIsReadOnly(t *testing.T) {
	dir := t.TempDir()
	writeMarker(t, dir, "kind: workitem\nid: x\ntype: design\ntitle: T\n")
	before, err := os.ReadFile(filepath.Join(dir, MarkerFilename))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Read(dir); err != nil {
		t.Fatal(err)
	}
	_ = IsResident(dir)

	after, err := os.ReadFile(filepath.Join(dir, MarkerFilename))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("reading the marker must not modify it")
	}
}

func TestIsFestivalDir(t *testing.T) {
	for _, marker := range []string{FestivalGoalFile, FestivalOverviewFile, FestivalConfigFile} {
		t.Run(marker, func(t *testing.T) {
			dir := t.TempDir()
			if IsFestivalDir(dir) {
				t.Fatal("empty dir should not read as a festival")
			}
			if err := os.WriteFile(filepath.Join(dir, marker), []byte("x"), 0o644); err != nil {
				t.Fatal(err)
			}
			if !IsFestivalDir(dir) {
				t.Errorf("%s should mark a festival directory", marker)
			}
		})
	}

	t.Run("a directory named like a marker does not count", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, FestivalGoalFile), 0o755); err != nil {
			t.Fatal(err)
		}
		if IsFestivalDir(dir) {
			t.Error("a directory named FESTIVAL_GOAL.md is not a festival marker")
		}
	})
}

func TestClassify(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantStr string
		want    DirKind
	}{
		{"empty", nil, "neither", KindNeither},
		{"festival", map[string]string{FestivalConfigFile: "version: \"1.0\"\n"}, "festival", KindFestival},
		{"resident", map[string]string{MarkerFilename: "kind: workitem\nid: x\ntype: design\n"}, "resident", KindResident},
		{
			// The workitem marker wins: camp writes it only on a real promote,
			// while a stale fest.yaml proves nothing about the current owner.
			name: "both markers classify as resident",
			files: map[string]string{
				MarkerFilename:     "kind: workitem\nid: x\ntype: design\n",
				FestivalConfigFile: "version: \"1.0\"\n",
			},
			wantStr: "resident",
			want:    KindResident,
		},
		{
			name:    "wrong-kind sidecar plus festival marker is a festival",
			files:   map[string]string{MarkerFilename: "kind: other\n", FestivalConfigFile: "version: \"1.0\"\n"},
			wantStr: "festival",
			want:    KindFestival,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, body := range tc.files {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := Classify(dir)
			if got != tc.want {
				t.Errorf("Classify = %v, want %v", got, tc.want)
			}
			if got.String() != tc.wantStr {
				t.Errorf("String = %q, want %q", got.String(), tc.wantStr)
			}
		})
	}
}
