package show

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func stampResident(t *testing.T, dir, wfType, title string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "version: v1alpha8\nkind: workitem\nid: " + wfType + "-x-1\ntype: " + wfType + "\ntitle: " + title + "\n"
	if err := os.WriteFile(filepath.Join(dir, ".workitem"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestListResidentsByStatus(t *testing.T) {
	root := t.TempDir()
	stampResident(t, filepath.Join(root, "active", "bravo"), "design", "Bravo")
	stampResident(t, filepath.Join(root, "active", "alpha"), "explore", "Alpha")
	stampResident(t, filepath.Join(root, "ready", "ready-one"), "design", "Ready One")
	// Not residents: a festival, a plain dir, and a dot dir.
	if err := os.MkdirAll(filepath.Join(root, "active", ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	stampResident(t, filepath.Join(root, "active", ".hidden"), "design", "Hidden")
	if err := os.MkdirAll(filepath.Join(root, "active", "afest"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "active", "afest", "fest.yaml"), []byte("version: \"1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// planning is not a rail stage.
	stampResident(t, filepath.Join(root, "planning", "stray"), "design", "Stray")

	t.Run("active is sorted and excludes non-residents", func(t *testing.T) {
		got := ListResidentsByStatus(t.Context(), root, "active")
		if len(got) != 2 {
			t.Fatalf("got %d residents, want 2: %+v", len(got), got)
		}
		if got[0].Name != "alpha" || got[1].Name != "bravo" {
			t.Errorf("not sorted by name: %s, %s", got[0].Name, got[1].Name)
		}
		if got[0].Type != "explore" {
			t.Errorf("Type = %q, want explore", got[0].Type)
		}
	})

	t.Run("ready", func(t *testing.T) {
		got := ListResidentsByStatus(t.Context(), root, "ready")
		if len(got) != 1 || got[0].Title != "Ready One" {
			t.Fatalf("got %+v, want one Ready One", got)
		}
	})

	t.Run("non-rail stages yield nothing", func(t *testing.T) {
		for _, status := range []string{"planning", "ritual", "chains", "dungeon/completed"} {
			if got := ListResidentsByStatus(t.Context(), root, status); got != nil {
				t.Errorf("%s yielded %d residents, want none", status, len(got))
			}
		}
	})

	t.Run("missing stage dir is not an error", func(t *testing.T) {
		if got := ListResidentsByStatus(t.Context(), t.TempDir(), "active"); got != nil {
			t.Errorf("got %+v, want nil for an absent stage dir", got)
		}
	})
}

func TestResidentCard_TitleFallsBackToName(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "active", "untitled")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".workitem"),
		[]byte("version: v1alpha8\nkind: workitem\nid: design-untitled-1\ntype: design\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ListResidentsByStatus(t.Context(), root, "active")
	if len(got) != 1 {
		t.Fatalf("want 1 resident, got %d", len(got))
	}
	if got[0].Title != "untitled" {
		t.Errorf("Title = %q, want the basename fallback", got[0].Title)
	}
}

func TestResidentCard_Progress(t *testing.T) {
	tests := []struct {
		name string
		run  *StandaloneWorkflowInfo
		want string
	}{
		{"no runtime", nil, ""},
		{"steps", &StandaloneWorkflowInfo{CompletedSteps: 1, TotalSteps: 3}, "1/3 steps"},
		{"status only", &StandaloneWorkflowInfo{RunStatus: "blocked"}, "blocked"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &ResidentCard{Run: tc.run}
			if got := c.Progress(); got != tc.want {
				t.Errorf("Progress = %q, want %q", got, tc.want)
			}
		})
	}
}

// Both fest list and fest status list marshal this type, so the published shape
// must not include internal fields.
func TestResidentCard_MarshalJSONShape(t *testing.T) {
	c := &ResidentCard{
		Name:  "runres",
		Title: "Run Resident",
		Type:  "explore",
		Path:  "/absolute/should/not/leak",
		Run:   &StandaloneWorkflowInfo{RunStatus: "active", CompletedSteps: 1, TotalSteps: 3, RuntimeDir: "/leaky"},
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	wantKeys := []string{"name", "title", "type", "run_status", "completed_steps", "total_steps"}
	if len(got) != len(wantKeys) {
		t.Errorf("keys = %v, want exactly %v", keysOf(got), wantKeys)
	}
	for _, k := range wantKeys {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in %s", k, raw)
		}
	}
	for _, forbidden := range []string{"Path", "path", "Run", "RuntimeDir", "runtime_dir"} {
		if _, ok := got[forbidden]; ok {
			t.Errorf("internal field %q leaked into the contract: %s", forbidden, raw)
		}
	}

	t.Run("no runtime omits run fields", func(t *testing.T) {
		raw, err := json.Marshal(&ResidentCard{Name: "n", Title: "t", Type: "design"})
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		if len(m) != 3 {
			t.Errorf("want only name/title/type, got %v", keysOf(m))
		}
	})
}

func keysOf(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
