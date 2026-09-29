package progress

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	progresspkg "github.com/Obedience-Corp/fest/internal/progress"
)

// displayNow is the reference time the display fixtures seed their event
// timestamps against. Nothing in the Blockers section renders a relative age,
// so only the calendar-time Duration line moves with the wall clock and it is
// normalised out below.
var displayNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

var durationLine = regexp.MustCompile(`(?m)^Duration .*$`)

func displayFixture(t *testing.T, events ...string) string {
	t.Helper()
	dir := t.TempDir()

	writeDisplayFile(t, filepath.Join(dir, "fest.yaml"),
		"version: \"1.0\"\nname: display-blockers-test\nid: DBT-001\nmetadata:\n  id: DBT-001\n"+
			"  status_history:\n    - status: active\n      timestamp: 2026-02-10T00:00:00Z\n")

	seqDir := filepath.Join(dir, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, progresspkg.ProgressDir), 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}

	writeDisplayFile(t, filepath.Join(dir, "001_PHASE", "PHASE_GOAL.md"),
		"---\nfest_type: phase_goal\nfest_id: 001_PHASE\nfest_phase_type: implementation\n---\n# Phase Goal\n")
	writeDisplayFile(t, filepath.Join(seqDir, "SEQUENCE_GOAL.md"),
		"---\nfest_type: sequence_goal\nfest_id: 01_seq\n---\n# Sequence Goal\n")
	for _, name := range []string{"01_cache_warmup", "02_legacy_import", "03_ship_it"} {
		writeDisplayFile(t, filepath.Join(seqDir, name+".md"),
			"---\nfest_type: task\nfest_id: "+name+"\n---\n# Task "+name+"\n")
	}

	writeDisplayFile(t, filepath.Join(dir, progresspkg.ProgressDir, progresspkg.ProgressEventsFile),
		strings.Join(events, "\n")+"\n")

	t.Setenv("HOME", t.TempDir())
	return dir
}

func writeDisplayFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func displayStamp(daysAgo int) string {
	return displayNow.AddDate(0, 0, -daysAgo).Format(time.RFC3339Nano)
}

func displayBlockedEvent(task, reason string, daysAgo int) string {
	return marshalDisplayEvent(map[string]any{
		"ts":     displayStamp(daysAgo),
		"event":  string(progresspkg.EventBlocked),
		"task":   "001_PHASE/01_seq/" + task,
		"reason": reason,
	})
}

func displayDeferredEvent(task, blockerMessage, deferralReason string, daysAgo int) string {
	return marshalDisplayEvent(map[string]any{
		"ts":              displayStamp(daysAgo),
		"event":           string(progresspkg.EventBlockerDeferred),
		"task":            "001_PHASE/01_seq/" + task,
		"reason":          blockerMessage,
		"deferral_reason": deferralReason,
		"deferred_by":     "Lance Rogers",
		"actor":           "operator",
		"tty":             true,
	})
}

func marshalDisplayEvent(event map[string]any) string {
	encoded, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

// renderFestivalProgress runs the real display path and returns its stdout with
// the wall-clock Duration line normalised, which is the only volatile line.
func renderFestivalProgress(t *testing.T, festDir string) string {
	t.Helper()

	mgr, err := progresspkg.NewManagerReadOnly(t.Context(), festDir)
	if err != nil {
		t.Fatalf("NewManagerReadOnly() error = %v", err)
	}
	loc := &show.LocationInfo{
		Type: "festival",
		Festival: &show.FestivalInfo{
			Name:   "display-blockers-test",
			Path:   festDir,
			Status: "active",
		},
	}

	orig := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatalf("stdout pipe: %v", pipeErr)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		out, _ := io.ReadAll(r)
		done <- string(out)
	}()

	runErr := showFestivalProgress(t.Context(), mgr, loc, &progressOptions{})

	_ = w.Close()
	os.Stdout = orig
	out := <-done
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("showFestivalProgress() error = %v", runErr)
	}
	return durationLine.ReplaceAllString(out, "Duration <normalised> (calendar time)")
}

func assertDisplayGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if os.Getenv("UPDATE_GOLDEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with UPDATE_GOLDEN=1 to create it)", path, err)
	}
	if got != string(want) {
		t.Errorf("%s mismatch\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}

// TestFestivalProgressNothingDeferredIsUnchanged is golden 1. It was captured
// from the display before the Open and Deferred split existed, so a later diff
// against it proves the split changed nothing for a festival with no deferral.
func TestFestivalProgressNothingDeferredIsUnchanged(t *testing.T) {
	festDir := displayFixture(t,
		displayBlockedEvent("01_cache_warmup.md", "provider API removed; warm-up not possible", 3),
		displayBlockedEvent("02_legacy_import.md", "upstream schema gone", 6),
	)

	out := renderFestivalProgress(t, festDir)
	assertDisplayGolden(t, "progress_nothing_deferred.txt", out)

	if !strings.Contains(out, "2 task(s) blocked\n") {
		t.Errorf("blocked line must keep its current text with nothing deferred:\n%s", out)
	}
	if strings.Contains(out, "deferred") || strings.Contains(out, "Deferred") {
		t.Errorf("nothing deferred must print no deferral wording:\n%s", out)
	}
}

// TestFestivalProgressSomeDeferred is golden 2: the count line carries both
// numbers and the Blockers section splits into Open and Deferred.
func TestFestivalProgressSomeDeferred(t *testing.T) {
	festDir := displayFixture(t,
		displayBlockedEvent("01_cache_warmup.md", "provider API removed; warm-up not possible", 3),
		displayBlockedEvent("02_legacy_import.md", "upstream schema gone", 6),
		displayDeferredEvent("02_legacy_import.md", "upstream schema gone",
			"revisit after the v2 migration lands", 2),
	)

	out := renderFestivalProgress(t, festDir)
	assertDisplayGolden(t, "progress_some_deferred.txt", out)

	if !strings.Contains(out, "2 task(s) blocked (1 deferred)") {
		t.Errorf("count line must carry both numbers:\n%s", out)
	}
	openAt, deferredAt := strings.Index(out, "\nOpen\n"), strings.Index(out, "\nDeferred\n")
	if openAt < 0 || deferredAt < 0 || openAt > deferredAt {
		t.Errorf("Open must be grouped before Deferred:\n%s", out)
	}
	if !strings.Contains(out, "    deferred: revisit after the v2 migration lands") {
		t.Errorf("a deferred entry must show its deferral reason:\n%s", out)
	}
}

// TestFestivalProgressAllDeferred is golden 3: only the Deferred group appears
// and no empty Open heading is printed.
func TestFestivalProgressAllDeferred(t *testing.T) {
	festDir := displayFixture(t,
		displayBlockedEvent("01_cache_warmup.md", "provider API removed; warm-up not possible", 3),
		displayBlockedEvent("02_legacy_import.md", "upstream schema gone", 6),
		displayDeferredEvent("01_cache_warmup.md", "provider API removed; warm-up not possible",
			"the vendor replies next week", 1),
		displayDeferredEvent("02_legacy_import.md", "upstream schema gone",
			"revisit after the v2 migration lands", 2),
	)

	out := renderFestivalProgress(t, festDir)
	assertDisplayGolden(t, "progress_all_deferred.txt", out)

	if !strings.Contains(out, "2 task(s) blocked (2 deferred)") {
		t.Errorf("count line must carry both numbers:\n%s", out)
	}
	if strings.Contains(out, "\nOpen\n") {
		t.Errorf("an empty Open heading must not be printed:\n%s", out)
	}
}

// TestFestivalProgressBlockedCountIncludesDeferred pins design doc 05 scenario
// E2: deferring a blocker must never make the blocked count fall.
func TestFestivalProgressBlockedCountIncludesDeferred(t *testing.T) {
	before := displayFixture(t,
		displayBlockedEvent("01_cache_warmup.md", "provider API removed; warm-up not possible", 3),
		displayBlockedEvent("02_legacy_import.md", "upstream schema gone", 6),
	)
	after := displayFixture(t,
		displayBlockedEvent("01_cache_warmup.md", "provider API removed; warm-up not possible", 3),
		displayBlockedEvent("02_legacy_import.md", "upstream schema gone", 6),
		displayDeferredEvent("02_legacy_import.md", "upstream schema gone",
			"revisit after the v2 migration lands", 2),
	)

	if !strings.Contains(renderFestivalProgress(t, before), "2 task(s) blocked") {
		t.Fatal("the fixture must start with two blocked tasks")
	}
	out := renderFestivalProgress(t, after)
	if strings.Contains(out, "1 task(s) blocked") {
		t.Errorf("the blocked count dropped after a deferral:\n%s", out)
	}
	if !strings.Contains(out, "2 task(s) blocked (1 deferred)") {
		t.Errorf("the blocked count must still be 2:\n%s", out)
	}
}
