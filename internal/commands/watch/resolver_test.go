package watch

import (
	"reflect"
	"strings"
	"testing"
)

func TestPreferredPickerStatusesNarrowsToWorkingStatus(t *testing.T) {
	got := preferredPickerStatuses("/campaign/festivals/active/launch-LW0001", "/campaign/festivals")
	if !reflect.DeepEqual(got, []string{"active"}) {
		t.Fatalf("preferred statuses = %#v, want [active]", got)
	}
}

func TestPreferredPickerStatusesIgnoresDungeon(t *testing.T) {
	// Dungeon festivals are terminal and never watched, so a dungeon cwd must
	// not narrow (or surface) dungeon festivals in the picker.
	got := preferredPickerStatuses("/campaign/festivals/dungeon/completed/2026-05", "/campaign/festivals")
	if got != nil {
		t.Fatalf("preferred statuses for a dungeon cwd = %#v, want nil", got)
	}
}

func TestPreferredPickerStatusesIgnoresRitual(t *testing.T) {
	got := preferredPickerStatuses("/campaign/festivals/ritual/weekly-review-RI-WR0001", "/campaign/festivals")
	if got != nil {
		t.Fatalf("preferred statuses for a ritual cwd = %#v, want nil", got)
	}
}

func TestPickerStatusesFallsBackToWatchableStatuses(t *testing.T) {
	got := pickerStatuses("/campaign/festivals/dungeon/completed/2026-05", "/campaign/festivals")
	if !reflect.DeepEqual(got, watchPickerStatuses) {
		t.Fatalf("picker statuses for a dungeon cwd = %#v, want watchPickerStatuses", got)
	}
}

func TestWatchableStatusesExcludeRitualAndOrderActiveFirst(t *testing.T) {
	want := []string{"active", "ready", "planning"}
	if !reflect.DeepEqual(watchPickerStatuses, want) {
		t.Fatalf("watchPickerStatuses = %#v, want %#v", watchPickerStatuses, want)
	}
	for _, status := range watchPickerStatuses {
		if status == "ritual" || strings.HasPrefix(status, "dungeon") {
			t.Fatalf("watch picker must not surface %q (not a watch target)", status)
		}
	}
}
