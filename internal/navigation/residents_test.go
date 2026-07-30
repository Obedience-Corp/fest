package navigation

import (
	"os"
	"path/filepath"
	"testing"
)

func mkResident(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "version: v1alpha8\nkind: workitem\nid: design-x-1\ntype: design\ntitle: X\n"
	if err := os.WriteFile(filepath.Join(dir, ".workitem"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkFestival(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fest.yaml"), []byte("version: \"1.0\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func targetNames(targets []FuzzyTarget) map[string]bool {
	out := make(map[string]bool, len(targets))
	for _, t := range targets {
		out[t.Name] = true
	}
	return out
}

func TestCollectNavigationTargets_IncludesResidents(t *testing.T) {
	root := t.TempDir()
	mkFestival(t, filepath.Join(root, "active", "named-fest-NF0001"))
	mkResident(t, filepath.Join(root, "active", "activeres"))
	mkResident(t, filepath.Join(root, "ready", "readyres"))
	// Out of scope for v1: a resident in the dungeon is terminal.
	mkResident(t, filepath.Join(root, ".dungeon", "completed", "2026-07-29", "shelvedres"))
	// A plain directory is still not navigable.
	if err := os.MkdirAll(filepath.Join(root, "active", "plaindir"), 0o755); err != nil {
		t.Fatal(err)
	}
	// planning is not a rail stage.
	mkResident(t, filepath.Join(root, "planning", "planningres"))

	names := targetNames(CollectNavigationTargets(root))

	for _, want := range []string{"named-fest-NF0001", "activeres", "readyres"} {
		if !names[want] {
			t.Errorf("missing navigation target %q", want)
		}
	}
	for _, unwanted := range []string{"shelvedres", "plaindir", "planningres"} {
		if names[unwanted] {
			t.Errorf("%q should not be a navigation target", unwanted)
		}
	}
}

// Festivals keep their place ahead of residents: residents are additional
// targets, not replacements.
func TestCollectNavigationTargets_FestivalsKeepPriority(t *testing.T) {
	root := t.TempDir()
	mkResident(t, filepath.Join(root, "active", "aaa-resident"))
	mkFestival(t, filepath.Join(root, "active", "zzz-fest-ZF0001"))

	targets := CollectNavigationTargets(root)
	var festPriority, resPriority int
	var seenFest, seenRes bool
	for _, tg := range targets {
		switch tg.Name {
		case "zzz-fest-ZF0001":
			festPriority, seenFest = tg.Priority, true
		case "aaa-resident":
			resPriority, seenRes = tg.Priority, true
		}
	}
	if !seenFest || !seenRes {
		t.Fatalf("expected both targets, fest=%v resident=%v", seenFest, seenRes)
	}
	if festPriority != resPriority {
		t.Errorf("same stage should give the same priority: fest=%d resident=%d", festPriority, resPriority)
	}
}

func TestCollectFestivalsInStatus_IncludesResidents(t *testing.T) {
	root := t.TempDir()
	mkFestival(t, filepath.Join(root, "active", "named-fest-NF0001"))
	mkResident(t, filepath.Join(root, "active", "activeres"))

	names := targetNames(CollectFestivalsInStatus(root, "active"))
	if !names["named-fest-NF0001"] || !names["activeres"] {
		t.Errorf("want both festival and resident, got %v", names)
	}
}

func TestCollectFestivalsInStatus_DungeonExcludesResidents(t *testing.T) {
	root := t.TempDir()
	mkResident(t, filepath.Join(root, ".dungeon", "completed", "shelvedres"))

	if names := targetNames(CollectFestivalsInStatus(root, "dungeon/completed")); names["shelvedres"] {
		t.Error("a dungeon resident must not be a navigation target in v1")
	}
}

// A festival-only campaign must produce the same targets as before residents
// existed.
func TestCollectNavigationTargets_NoResidentsUnchanged(t *testing.T) {
	root := t.TempDir()
	mkFestival(t, filepath.Join(root, "active", "one-fest-OF0001"))
	mkFestival(t, filepath.Join(root, "ready", "two-fest-TF0002"))

	names := targetNames(CollectNavigationTargets(root))
	if !names["one-fest-OF0001"] || !names["two-fest-TF0002"] {
		t.Errorf("festival targets missing: %v", names)
	}
	// Only the two festivals plus the status directories themselves.
	for name := range names {
		if name == "one-fest-OF0001" || name == "two-fest-TF0002" {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Errorf("unexpected target %q in a festival-only campaign", name)
		}
	}
}
