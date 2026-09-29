package task

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// blockedListNow is the fixed reference time every blocked-list golden is
// rendered against. The fixtures seed absolute event timestamps relative to it,
// so both the rendered ages and the JSON timestamps are deterministic and no
// golden needs timestamp normalisation.
var blockedListNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func blockedListStamp(daysAgo int) string {
	return blockedListNow.AddDate(0, 0, -daysAgo).Format(time.RFC3339Nano)
}

// blockedListFixture builds an active festival with three implementation tasks
// and seeds the progress event log directly, which is the store's source of
// truth. Seeding the log rather than calling the verbs keeps the timestamps
// fixed and lets a fixture carry a deferral without passing the operator guard.
func blockedListFixture(t *testing.T, events ...string) string {
	t.Helper()
	dir := t.TempDir()

	festYAML := "version: \"1.0\"\nname: blocked-list-test\nid: BLT-001\nmetadata:\n  id: BLT-001\n" +
		"  status_history:\n    - status: active\n      timestamp: 2026-02-10T00:00:00Z\n"
	writeFixtureFile(t, filepath.Join(dir, "fest.yaml"), festYAML)

	seqDir := filepath.Join(dir, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, progress.ProgressDir), 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}

	writeFixtureFile(t, filepath.Join(dir, "001_PHASE", "PHASE_GOAL.md"),
		"---\nfest_type: phase_goal\nfest_id: 001_PHASE\nfest_phase_type: implementation\n---\n# Phase Goal\n")
	writeFixtureFile(t, filepath.Join(seqDir, "SEQUENCE_GOAL.md"),
		"---\nfest_type: sequence_goal\nfest_id: 01_seq\n---\n# Sequence Goal\n")
	for _, name := range []string{"01_cache_warmup", "02_legacy_import", "03_ship_it"} {
		writeFixtureFile(t, filepath.Join(seqDir, name+".md"),
			"---\nfest_type: task\nfest_id: "+name+"\n---\n# Task "+name+"\n")
	}

	writeFixtureFile(t, filepath.Join(dir, progress.ProgressDir, progress.ProgressEventsFile),
		strings.Join(events, "\n")+"\n")

	t.Setenv("HOME", t.TempDir())
	return dir
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func blockedEvent(task, reason string, daysAgo int, attempts ...string) string {
	event := map[string]any{
		"ts":     blockedListStamp(daysAgo),
		"event":  string(progress.EventBlocked),
		"task":   "001_PHASE/01_seq/" + task,
		"reason": reason,
	}
	if len(attempts) > 0 {
		event["attempts"] = attempts
	}
	return marshalEvent(event)
}

func deferredEvent(task, blockerMessage, deferralReason, by string, daysAgo int) string {
	return marshalEvent(map[string]any{
		"ts":              blockedListStamp(daysAgo),
		"event":           string(progress.EventBlockerDeferred),
		"task":            "001_PHASE/01_seq/" + task,
		"reason":          blockerMessage,
		"deferral_reason": deferralReason,
		"deferred_by":     by,
		"actor":           "operator",
		"tty":             true,
	})
}

func marshalEvent(event map[string]any) string {
	encoded, err := json.Marshal(event)
	if err != nil {
		panic(err)
	}
	return string(encoded)
}

func allOpenFixture(t *testing.T) string {
	t.Helper()
	return blockedListFixture(t,
		blockedEvent("01_cache_warmup.md", "provider API removed; warm-up not possible", 3,
			"checked the changelog", "asked the vendor, no replacement"),
		blockedEvent("02_legacy_import.md", "upstream schema gone", 6,
			"read the migration notes"),
	)
}

func allDeferredFixture(t *testing.T) string {
	t.Helper()
	return blockedListFixture(t,
		blockedEvent("01_cache_warmup.md", "provider API removed; warm-up not possible", 3,
			"checked the changelog"),
		blockedEvent("02_legacy_import.md", "upstream schema gone", 6,
			"read the migration notes"),
		deferredEvent("01_cache_warmup.md", "provider API removed; warm-up not possible",
			"the vendor replies next week", "Lance Rogers", 1),
		deferredEvent("02_legacy_import.md", "upstream schema gone",
			"revisit after the v2 migration lands", "Lance Rogers", 2),
	)
}

func mixedFixture(t *testing.T) string {
	t.Helper()
	return blockedListFixture(t,
		blockedEvent("01_cache_warmup.md", "provider API removed; warm-up not possible", 3,
			"checked the changelog", "asked the vendor, no replacement"),
		blockedEvent("02_legacy_import.md", "upstream schema gone", 6,
			"read the migration notes"),
		deferredEvent("02_legacy_import.md", "upstream schema gone",
			"revisit after the v2 migration lands", "Lance Rogers", 2),
	)
}

func runListInto(t *testing.T, festDir string) string {
	t.Helper()
	var out bytes.Buffer
	if err := runBlockedList(t.Context(), &out, festDir, blockedListNow); err != nil {
		t.Fatalf("runBlockedList() error = %v", err)
	}
	return out.String()
}

// assertGolden compares against testdata, rewriting it when UPDATE_GOLDEN is
// set so a deliberate output change is a reviewable diff.
func assertGolden(t *testing.T, name, got string) {
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

func TestBlockedListAllOpenGolden(t *testing.T) {
	resetTaskFlags()
	festDir := allOpenFixture(t)

	assertGolden(t, "blocked_list_all_open.txt", runListInto(t, festDir))

	blockedJSON = true
	t.Cleanup(resetTaskFlags)
	rendered := runListInto(t, festDir)
	assertGolden(t, "blocked_list_all_open.json", rendered)

	if strings.Contains(rendered, "deferral_reason") || strings.Contains(rendered, "deferred_at") ||
		strings.Contains(rendered, "deferred_by") {
		t.Errorf("an all-open festival must carry no deferral keys:\n%s", rendered)
	}
}

func TestBlockedListAllDeferredGolden(t *testing.T) {
	resetTaskFlags()
	festDir := allDeferredFixture(t)

	assertGolden(t, "blocked_list_all_deferred.txt", runListInto(t, festDir))

	blockedJSON = true
	t.Cleanup(resetTaskFlags)
	rendered := runListInto(t, festDir)
	assertGolden(t, "blocked_list_all_deferred.json", rendered)

	var result blockedListResult
	if err := json.Unmarshal([]byte(rendered), &result); err != nil {
		t.Fatalf("decoding list JSON: %v", err)
	}
	if result.OpenCount != 0 || result.DeferredCount != 2 {
		t.Errorf("counts = open %d deferred %d, want 0 and 2", result.OpenCount, result.DeferredCount)
	}
	for _, entry := range result.Blocked {
		if !entry.Deferred || entry.DeferralReason == "" || entry.DeferredAt == nil || entry.DeferredBy == "" {
			t.Errorf("entry %s is missing a deferral field: %+v", entry.Task, entry)
		}
	}
}

func TestBlockedListMixedGolden(t *testing.T) {
	resetTaskFlags()
	festDir := mixedFixture(t)

	rendered := runListInto(t, festDir)
	assertGolden(t, "blocked_list_mixed.txt", rendered)

	openAt := strings.Index(rendered, "01_cache_warmup.md")
	deferredAt := strings.Index(rendered, "02_legacy_import.md")
	if openAt < 0 || deferredAt < 0 || openAt > deferredAt {
		t.Errorf("open entries must be listed before deferred ones:\n%s", rendered)
	}

	blockedJSON = true
	t.Cleanup(resetTaskFlags)
	jsonOut := runListInto(t, festDir)
	assertGolden(t, "blocked_list_mixed.json", jsonOut)

	var result blockedListResult
	if err := json.Unmarshal([]byte(jsonOut), &result); err != nil {
		t.Fatalf("decoding list JSON: %v", err)
	}
	if result.OpenCount != 1 || result.DeferredCount != 1 {
		t.Errorf("counts = open %d deferred %d, want 1 and 1", result.OpenCount, result.DeferredCount)
	}
}

func TestBlockedListFiltersAreExclusiveViews(t *testing.T) {
	festDir := mixedFixture(t)

	t.Run("open", func(t *testing.T) {
		resetTaskFlags()
		t.Cleanup(resetTaskFlags)
		blockedListOpen = true
		out := runListInto(t, festDir)
		if !strings.Contains(out, "01_cache_warmup.md") || strings.Contains(out, "02_legacy_import.md") {
			t.Errorf("--open must list only the open blocker:\n%s", out)
		}
		if strings.Contains(out, "Deferred") {
			t.Errorf("--open must not print an empty Deferred heading:\n%s", out)
		}
	})

	t.Run("deferred", func(t *testing.T) {
		resetTaskFlags()
		t.Cleanup(resetTaskFlags)
		blockedListDeferred = true
		out := runListInto(t, festDir)
		if !strings.Contains(out, "02_legacy_import.md") || strings.Contains(out, "01_cache_warmup.md") {
			t.Errorf("--deferred must list only the deferred blocker:\n%s", out)
		}
		if strings.Contains(out, "Open") {
			t.Errorf("--deferred must not print an empty Open heading:\n%s", out)
		}
	})

	t.Run("deferred_json_counts", func(t *testing.T) {
		resetTaskFlags()
		t.Cleanup(resetTaskFlags)
		blockedListDeferred, blockedJSON = true, true
		var result blockedListResult
		if err := json.Unmarshal([]byte(runListInto(t, festDir)), &result); err != nil {
			t.Fatalf("decoding list JSON: %v", err)
		}
		if result.OpenCount != 0 || result.DeferredCount != 1 || len(result.Blocked) != 1 {
			t.Errorf("filtered counts = open %d deferred %d entries %d, want 0, 1 and 1",
				result.OpenCount, result.DeferredCount, len(result.Blocked))
		}
	})
}

func TestBlockedListEmptyFestivalPrintsOneLine(t *testing.T) {
	resetTaskFlags()
	festDir := blockedListFixture(t)

	out := runListInto(t, festDir)
	if out != "No blocked tasks.\n" {
		t.Errorf("output = %q, want the single empty-result line", out)
	}
	if strings.Contains(out, "Open") || strings.Contains(out, "Deferred") {
		t.Errorf("an empty result must print no section headings:\n%s", out)
	}
}

func TestBlockedListEmptyJSONIsAnEmptyArray(t *testing.T) {
	resetTaskFlags()
	t.Cleanup(resetTaskFlags)
	blockedJSON = true
	festDir := blockedListFixture(t)

	out := runListInto(t, festDir)
	var result blockedListResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decoding list JSON: %v", err)
	}
	if result.Blocked == nil || len(result.Blocked) != 0 {
		t.Errorf("blocked = %v, want an empty array rather than null", result.Blocked)
	}
	if !strings.Contains(out, `"blocked": []`) {
		t.Errorf("empty JSON must carry an empty array:\n%s", out)
	}
}

// TestBlockedWithoutReasonKeepsItsOriginalError pins the behaviour --list must
// not change: the reporting path stays required-flag driven, so an operator who
// forgets --reason sees cobra's own message.
func TestBlockedWithoutReasonKeepsItsOriginalError(t *testing.T) {
	resetTaskFlags()
	cmd := newBlockedCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(nil)

	err := cmd.ExecuteContext(t.Context())
	if err == nil {
		t.Fatal("fest task blocked without --reason must fail")
	}
	if got, want := err.Error(), `required flag(s) "reason" not set`; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestBlockedListDoesNotRequireReason(t *testing.T) {
	resetTaskFlags()
	t.Cleanup(resetTaskFlags)
	cmd := newBlockedCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--list"})

	err := cmd.ExecuteContext(t.Context())
	if err == nil {
		t.Fatal("expected the missing festival context error, not a flag error")
	}
	if strings.Contains(err.Error(), "reason") {
		t.Errorf("error = %q, want --list to clear the required --reason", err)
	}
}

func TestBlockedFiltersWithoutListAreRefused(t *testing.T) {
	for _, flag := range []string{"--open", "--deferred"} {
		t.Run(flag, func(t *testing.T) {
			resetTaskFlags()
			t.Cleanup(resetTaskFlags)
			cmd := newBlockedCmd()
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{flag, "--reason", "x"})

			err := cmd.ExecuteContext(t.Context())
			if err == nil {
				t.Fatalf("%s without --list must fail", flag)
			}
			if !strings.Contains(err.Error(), "--list") {
				t.Errorf("error = %q, want it to name --list", err)
			}
		})
	}
}

func TestBlockedFiltersAreMutuallyExclusive(t *testing.T) {
	resetTaskFlags()
	t.Cleanup(resetTaskFlags)
	cmd := newBlockedCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--list", "--open", "--deferred"})

	err := cmd.ExecuteContext(t.Context())
	if err == nil {
		t.Fatal("--open and --deferred together must fail")
	}
	if !strings.Contains(err.Error(), "open") || !strings.Contains(err.Error(), "deferred") {
		t.Errorf("error = %q, want it to name both flags", err)
	}
}

// TestBlockedListJSONUsesSnakeCase records the convention observed in the
// sibling commands rather than assuming it: fest task show --json, fest status
// --json and fest next --json all emit snake_case keys with no envelope.
func TestBlockedListJSONUsesSnakeCase(t *testing.T) {
	var out bytes.Buffer
	if err := shared.EncodeJSON(&out, blockedListJSON(nil, nil)); err != nil {
		t.Fatalf("EncodeJSON() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, key := range []string{"blocked", "open_count", "deferred_count"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("missing key %q in %s", key, out.String())
		}
	}
	if len(decoded) != 3 {
		t.Errorf("unexpected keys in %s", out.String())
	}
}

func TestBlockedListRefusesATaskArgument(t *testing.T) {
	resetTaskFlags()
	t.Cleanup(resetTaskFlags)
	cmd := newBlockedCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--list", "001_PHASE/01_seq/01_task.md"})

	err := cmd.ExecuteContext(t.Context())
	if err == nil {
		t.Fatal("--list with a task must fail rather than ignore the task")
	}
	if !strings.Contains(err.Error(), "takes no task") {
		t.Errorf("error = %q, want it to say --list takes no task", err)
	}
}
