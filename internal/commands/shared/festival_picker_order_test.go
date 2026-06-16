package shared

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var orderBaseTime = time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)

func at(hours int) time.Time {
	return orderBaseTime.Add(time.Duration(hours) * time.Hour)
}

func ranked(name, status string, mt time.Time) rankedCandidate {
	return rankedCandidate{candidate: FestivalPickCandidate{Name: name, Status: status}, modTime: mt}
}

func rankedOrder(items []rankedCandidate) []string {
	names := make([]string, len(items))
	for i, r := range items {
		names[i] = r.candidate.Name
	}
	return names
}

func TestSortWatchPickerCandidatesGroupsByStatusThenRecency(t *testing.T) {
	items := []rankedCandidate{
		ranked("plan-old", "planning", at(1)),
		ranked("active-new", "active", at(5)),
		ranked("ready-mid", "ready", at(3)),
		ranked("active-old", "active", at(2)),
		ranked("plan-new", "planning", at(4)),
		ranked("ready-old", "ready", at(1)),
	}

	sortRankedCandidates(items)

	want := []string{"active-new", "active-old", "ready-mid", "ready-old", "plan-new", "plan-old"}
	if got := rankedOrder(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestWatchPickerStatusPrecedenceBeatsRecency(t *testing.T) {
	items := []rankedCandidate{
		ranked("plan-newest", "planning", at(100)),
		ranked("active-oldest", "active", at(1)),
	}

	sortRankedCandidates(items)

	want := []string{"active-oldest", "plan-newest"}
	if got := rankedOrder(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestWatchPickerMissingModTimeSortsLastInGroup(t *testing.T) {
	items := []rankedCandidate{
		ranked("active-missing", "active", time.Time{}),
		ranked("active-dated", "active", at(5)),
		ranked("ready-dated", "ready", at(5)),
	}

	sortRankedCandidates(items)

	want := []string{"active-dated", "active-missing", "ready-dated"}
	got := rankedOrder(items)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
	if len(items) != 3 {
		t.Fatalf("candidate count = %d, want 3 (none dropped)", len(items))
	}
}

func TestWatchPickerNameTieBreak(t *testing.T) {
	items := []rankedCandidate{
		ranked("bbb", "active", at(5)),
		ranked("aaa", "active", at(5)),
	}

	sortRankedCandidates(items)

	want := []string{"aaa", "bbb"}
	if got := rankedOrder(items); !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %#v, want %#v", got, want)
	}
}

func TestWatchPickerUnknownStatusSortsAfterKnown(t *testing.T) {
	items := []rankedCandidate{
		ranked("ritual-x", "ritual", at(100)),
		ranked("active-x", "active", at(1)),
	}

	sortRankedCandidates(items)

	want := []string{"active-x", "ritual-x"}
	if got := rankedOrder(items); !reflect.DeepEqual(got, want) {
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

func writeFestival(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("# Goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFestivalPickerItemsFallbackStaysWithinFallbackStatuses(t *testing.T) {
	festivals := t.TempDir()
	writeFestival(t, filepath.Join(festivals, "ritual", "weekly-RI-WR0001"))
	writeFestival(t, filepath.Join(festivals, "dungeon", "completed", "2026-05", "old-OW0001"))

	items := FestivalPickerItems(festivals, FestivalPickerOptions{
		PreferredStatuses:        []string{"active"},
		FallbackStatuses:         []string{"active", "ready", "planning"},
		OrderByStatusThenRecency: true,
	})

	for _, it := range items {
		if strings.Contains(it.Value, "/ritual/") || strings.Contains(it.Value, "/dungeon/") {
			t.Fatalf("watch picker leaked a non-watchable festival via fallback: %q", it.Value)
		}
	}
}

func TestFestivalPickerItemsFallbackBroadensWithinWatchableSet(t *testing.T) {
	festivals := t.TempDir()
	writeFestival(t, filepath.Join(festivals, "ready", "ready-RW0001"))
	writeFestival(t, filepath.Join(festivals, "ritual", "weekly-RI-WR0001"))

	items := FestivalPickerItems(festivals, FestivalPickerOptions{
		PreferredStatuses: []string{"active"},
		FallbackStatuses:  []string{"active", "ready", "planning"},
	})

	var found bool
	for _, it := range items {
		if strings.Contains(it.Value, "/ritual/") {
			t.Fatalf("watch picker leaked a ritual festival via fallback: %q", it.Value)
		}
		if strings.Contains(it.Value, "ready-RW0001") {
			found = true
		}
	}
	if !found {
		t.Fatal("fallback should broaden from empty active to ready")
	}
}
