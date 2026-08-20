package watch

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
)

func TestWatchCompletionExcludesStatusDirectories(t *testing.T) {
	got := formatWatchSelectorCompletions(watchCompletionCandidates(), "")
	for _, forbidden := range []string{"active", "ready", "planning", "ritual", "dungeon", "completed", "archived", "someday"} {
		if containsString(got, forbidden) {
			t.Fatalf("watch completion included status directory %q in %#v", forbidden, got)
		}
	}
}

func TestWatchCompletionExcludesDungeonFestivals(t *testing.T) {
	festivals := t.TempDir()
	makeWatchTestFestival(t, filepath.Join(festivals, "active", "launch-work-LW0001"))
	makeWatchTestFestival(t, filepath.Join(festivals, "dungeon", "completed", "2026-05-01", "old-work-OW0001"))

	// Production watch setting: restrict completion to watchable statuses.
	working := shared.CollectFestivalPickCandidates(festivals, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        watchPickerStatuses,
	})
	if candidateNamePresent(working, "old-work-OW0001") {
		t.Fatalf("watch completion must exclude dungeon festivals (terminal, never watched): %v", candidateNames(working))
	}
	if !candidateNamePresent(working, "launch-work-LW0001") {
		t.Fatalf("watch completion should include working festivals: %v", candidateNames(working))
	}

	// Without the working-status restriction the default surfaces dungeon
	// festivals — the leak the fix avoids by passing WorkingStatusDirectories.
	all := shared.CollectFestivalPickCandidates(festivals, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
	})
	if !candidateNamePresent(all, "old-work-OW0001") {
		t.Fatalf("default collection should still surface dungeon festivals: %v", candidateNames(all))
	}
}

func TestWatchCompletionExcludesRitualFestivals(t *testing.T) {
	festivals := t.TempDir()
	makeWatchTestFestival(t, filepath.Join(festivals, "active", "launch-work-LW0001"))
	makeWatchTestFestival(t, filepath.Join(festivals, "ritual", "weekly-review-RI-WR0001"))

	working := shared.CollectFestivalPickCandidates(festivals, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        watchPickerStatuses,
	})
	if candidateNamePresent(working, "weekly-review-RI-WR0001") {
		t.Fatalf("watch completion must exclude ritual festivals (templates, never watched): %v", candidateNames(working))
	}
	if !candidateNamePresent(working, "launch-work-LW0001") {
		t.Fatalf("watch completion should include working festivals: %v", candidateNames(working))
	}
}

func makeWatchTestFestival(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func candidateNames(candidates []shared.FestivalPickCandidate) []string {
	names := make([]string, 0, len(candidates))
	for _, c := range candidates {
		names = append(names, c.Name)
	}
	return names
}

func candidateNamePresent(candidates []shared.FestivalPickCandidate, name string) bool {
	for _, c := range candidates {
		if c.Name == name {
			return true
		}
	}
	return false
}

func TestWatchCompletionFuzzyNarrowsByNameAndID(t *testing.T) {
	byName := formatWatchSelectorCompletions(watchCompletionCandidates(), "laun")
	if !reflect.DeepEqual(byName, []string{"launch-work-LW0001"}) {
		t.Fatalf("name completion = %#v", byName)
	}

	byID := formatWatchSelectorCompletions(watchCompletionCandidates(), "OW")
	if len(byID) == 0 || byID[0] != "old-work-OW0001" {
		t.Fatalf("ID completion = %#v", byID)
	}
}

func TestWatchCommandHasCompletionFunction(t *testing.T) {
	cmd := NewWatchCommand()
	if cmd.ValidArgsFunction == nil {
		t.Fatal("watch command must set ValidArgsFunction")
	}
}

func watchCompletionCandidates() []shared.FestivalPickCandidate {
	return []shared.FestivalPickCandidate{
		{Name: "active", Path: "/campaign/festivals/active", Status: "active", StatusDirectory: true},
		{Name: "ready", Path: "/campaign/festivals/ready", Status: "ready", StatusDirectory: true},
		{Name: "planning", Path: "/campaign/festivals/planning", Status: "planning", StatusDirectory: true},
		{Name: "ritual", Path: "/campaign/festivals/ritual", Status: "ritual", StatusDirectory: true},
		{Name: "dungeon/completed", Path: "/campaign/festivals/dungeon/completed", Status: "dungeon/completed", StatusDirectory: true},
		{Name: "launch-work-LW0001", ID: "LW0001", Path: "/campaign/festivals/active/launch-work-LW0001", Status: "active"},
		{Name: "old-work-OW0001", ID: "OW0001", Path: "/campaign/festivals/dungeon/completed/2026-05/old-work-OW0001", Status: "dungeon/completed"},
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// The dungeon stays out of watch completion even when the campaign has no
// watchable festival left, which is when an unbounded fallback would surface it.
func TestWatchCompletionExcludesDungeonWhenNoWorkingFestival(t *testing.T) {
	festivals := t.TempDir()
	makeWatchTestFestival(t, filepath.Join(festivals, "dungeon", "completed", "2026-05-01", "old-work-OW0001"))

	working := shared.CollectFestivalPickCandidates(festivals, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        watchPickerStatuses,
	})
	if len(working) != 0 {
		t.Fatalf("watch completion must stay empty rather than offer dungeon festivals: %v", candidateNames(working))
	}
}
