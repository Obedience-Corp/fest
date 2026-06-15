package shared

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var orderBaseTime = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func at(hours int) time.Time {
	return orderBaseTime.Add(time.Duration(hours) * time.Hour)
}

func candidateOrder(candidates []FestivalPickCandidate) []string {
	names := make([]string, len(candidates))
	for i, c := range candidates {
		names[i] = c.Name
	}
	return names
}

func TestSortWatchPickerCandidatesGroupsByStatusThenRecency(t *testing.T) {
	candidates := []FestivalPickCandidate{
		{Name: "plan-old", Status: "planning", ModTime: at(1)},
		{Name: "active-new", Status: "active", ModTime: at(5)},
		{Name: "ready-mid", Status: "ready", ModTime: at(3)},
		{Name: "active-old", Status: "active", ModTime: at(2)},
		{Name: "plan-new", Status: "planning", ModTime: at(4)},
		{Name: "ready-old", Status: "ready", ModTime: at(1)},
	}

	sortWatchPickerCandidates(candidates)

	want := []string{"active-new", "active-old", "ready-mid", "ready-old", "plan-new", "plan-old"}
	if got := candidateOrder(candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestWatchPickerStatusPrecedenceBeatsRecency(t *testing.T) {
	candidates := []FestivalPickCandidate{
		{Name: "plan-newest", Status: "planning", ModTime: at(100)},
		{Name: "active-oldest", Status: "active", ModTime: at(1)},
	}

	sortWatchPickerCandidates(candidates)

	want := []string{"active-oldest", "plan-newest"}
	if got := candidateOrder(candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestWatchPickerMissingModTimeSortsLastInGroup(t *testing.T) {
	candidates := []FestivalPickCandidate{
		{Name: "active-missing", Status: "active"},
		{Name: "active-dated", Status: "active", ModTime: at(5)},
		{Name: "ready-dated", Status: "ready", ModTime: at(5)},
	}

	sortWatchPickerCandidates(candidates)

	want := []string{"active-dated", "active-missing", "ready-dated"}
	got := candidateOrder(candidates)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidate count = %d, want 3 (none dropped)", len(candidates))
	}
}

func TestWatchPickerNameTieBreak(t *testing.T) {
	candidates := []FestivalPickCandidate{
		{Name: "bbb", Status: "active", ModTime: at(5)},
		{Name: "aaa", Status: "active", ModTime: at(5)},
	}

	sortWatchPickerCandidates(candidates)

	want := []string{"aaa", "bbb"}
	if got := candidateOrder(candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestWatchPickerUnknownStatusSortsAfterKnown(t *testing.T) {
	candidates := []FestivalPickCandidate{
		{Name: "ritual-x", Status: "ritual", ModTime: at(100)},
		{Name: "active-x", Status: "active", ModTime: at(1)},
	}

	sortWatchPickerCandidates(candidates)

	want := []string{"active-x", "ritual-x"}
	if got := candidateOrder(candidates); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestLatestModTimeReturnsNewestNestedFile(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "001_PHASE")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	top := filepath.Join(dir, "FESTIVAL_GOAL.md")
	nested := filepath.Join(sub, "TASK.md")
	for _, f := range []string{top, nested} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	newest := at(2)
	chtimes(t, dir, at(-1))
	chtimes(t, top, at(1))
	chtimes(t, sub, at(0))
	chtimes(t, nested, newest)

	got := latestModTime(dir)
	if !got.Equal(newest) {
		t.Fatalf("latestModTime = %v, want %v", got, newest)
	}
}

func TestLatestModTimeMissingPathReturnsZero(t *testing.T) {
	got := latestModTime(filepath.Join(t.TempDir(), "does-not-exist"))
	if !got.IsZero() {
		t.Fatalf("latestModTime on missing path = %v, want zero", got)
	}
}

func chtimes(t *testing.T, path string, mt time.Time) {
	t.Helper()
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}
}
