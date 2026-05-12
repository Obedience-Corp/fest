package status

import (
	"reflect"
	"testing"
)

func TestStatusChangeCommitPaths_AppendsLifecycleEventLog(t *testing.T) {
	changedPaths := []string{
		"/repo/festivals/planning/my-fest",
		"/repo/festivals/ready/my-fest",
	}
	eventsPath := "/repo/festivals/.festival/.state/festival_events.jsonl"

	got := statusChangeCommitPaths(changedPaths, eventsPath)
	want := []string{
		"/repo/festivals/planning/my-fest",
		"/repo/festivals/ready/my-fest",
		"/repo/festivals/.festival/.state/festival_events.jsonl",
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statusChangeCommitPaths() = %#v, want %#v", got, want)
	}
}

func TestStatusChangeCommitPaths_DeduplicatesLifecycleEventLog(t *testing.T) {
	eventsPath := "/repo/festivals/.festival/.state/festival_events.jsonl"
	changedPaths := []string{
		"/repo/festivals/planning/my-fest",
		eventsPath,
		"/repo/festivals/ready/my-fest",
	}

	got := statusChangeCommitPaths(changedPaths, eventsPath)
	want := changedPaths

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statusChangeCommitPaths() = %#v, want %#v", got, want)
	}
}

func TestStatusChangeCommitPaths_EmptyEventPathLeavesScopeUnchanged(t *testing.T) {
	changedPaths := []string{
		"/repo/festivals/planning/my-fest",
		"/repo/festivals/ready/my-fest",
	}

	got := statusChangeCommitPaths(changedPaths, "")
	want := changedPaths

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("statusChangeCommitPaths() = %#v, want %#v", got, want)
	}
}
