package watch

import (
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

func TestWatchCompletionIncludesDungeonDateBucketFestivals(t *testing.T) {
	got := formatWatchSelectorCompletions(watchCompletionCandidates(), "")
	if !containsString(got, "old-work-OW0001") {
		t.Fatalf("watch completion should include dungeon date-bucket festivals, got %#v", got)
	}
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
