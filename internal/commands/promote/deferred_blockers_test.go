package promote

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/commands/task"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
)

const deferredFixtureTask = "001_PHASE/01_seq/01_task.md"

// deferredFestival builds an active festival whose one task is blocked and
// deferred, and returns it with the festival path.
func deferredFestival(t *testing.T, deferTask bool) *show.FestivalInfo {
	t.Helper()
	ctx := t.Context()

	dir := t.TempDir()
	seqDir := filepath.Join(dir, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, "01_task.md"),
		[]byte("---\nfest_type: task\n---\n\n# Task\n"), 0o644); err != nil {
		t.Fatalf("write task: %v", err)
	}

	mgr, err := progress.NewManager(ctx, dir)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if deferTask {
		if err := mgr.ReportBlocker(ctx, deferredFixtureTask, "provider API removed", []string{"checked the changelog"}); err != nil {
			t.Fatalf("ReportBlocker() error = %v", err)
		}
		if err := mgr.DeferBlocker(ctx, deferredFixtureTask, "the vendor replies next week", progress.DeferralAudit{
			Actor: "operator", TTY: true, DeferredBy: "Ada Lovelace",
		}); err != nil {
			t.Fatalf("DeferBlocker() error = %v", err)
		}
	} else if err := mgr.MarkComplete(ctx, deferredFixtureTask); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	return &show.FestivalInfo{Name: filepath.Base(dir), Path: dir, Status: "active"}
}

func stubPromoteGuard(t *testing.T, audit *task.OperatorAudit, err error) *int {
	t.Helper()
	calls := 0
	restore := operatorGuardFn
	operatorGuardFn = func(context.Context, string) (*task.OperatorAudit, error) {
		calls++
		return audit, err
	}
	t.Cleanup(func() { operatorGuardFn = restore })
	return &calls
}

func passingAudit() *task.OperatorAudit {
	return &task.OperatorAudit{
		Actor:        "operator",
		TTY:          true,
		AgentMarkers: []string{"OBEY_AGENT", "CLAUDE_CODE", "CLAUDECODE", "CLAUDE_CODE_SESSION_ID", "CODEX_TASK", "OBEY_SESSION_ID"},
		Ancestry:     []string{"zsh", "login"},
		DeferredBy:   "Ada Lovelace",
	}
}

func readEvents(t *testing.T, festivalPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(festivalPath, progress.ProgressDir, progress.ProgressEventsFile))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	return string(data)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 0, 4096)
		chunk := make([]byte, 1024)
		for {
			n, readErr := r.Read(chunk)
			buf = append(buf, chunk[:n]...)
			if readErr != nil {
				break
			}
		}
		done <- string(buf)
	}()

	fn()
	_ = w.Close()
	os.Stdout = original
	return <-done
}

func TestPromoteRefusesWithDeferredBlockers(t *testing.T) {
	festival := deferredFestival(t, true)
	calls := stubPromoteGuard(t, nil, errors.Validation("the guard must not run without --force"))
	before := readEvents(t, festival.Path)

	var halt bool
	var err error
	output := captureStdout(t, func() {
		halt, err = enforceDeferredBlockers(t.Context(), festival, "completed", &promoteOptions{})
	})

	if err != nil {
		t.Fatalf("enforceDeferredBlockers() error = %v, want the printed refusal", err)
	}
	if !halt {
		t.Fatal("promotion must be refused while a blocker is deferred")
	}
	if *calls != 0 {
		t.Error("the operator guard ran without --force")
	}
	for _, want := range []string{
		"1 deferred blockers are still open. fest next will revisit them.",
		"Promote anyway with --force to record them as dropped.",
		"provider API removed",
		"the vendor replies next week",
		"--force drops every deferred blocker. There is no per-task drop yet.",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("refusal missing %q:\n%s", want, output)
		}
	}
	if after := readEvents(t, festival.Path); after != before {
		t.Error("a refused promotion appended an event")
	}
	if _, statErr := os.Stat(filepath.Join(festival.Path, shared.DroppedBlockerRecordFile)); statErr == nil {
		t.Error("a refused promotion wrote a dropped blocker record")
	}
}

func TestPromoteRefusalIsStructuredInJSON(t *testing.T) {
	festival := deferredFestival(t, true)
	stubPromoteGuard(t, nil, errors.Validation("the guard must not run without --force"))

	var halt bool
	var err error
	output := captureStdout(t, func() {
		halt, err = enforceDeferredBlockers(t.Context(), festival, "completed", &promoteOptions{json: true})
	})

	if !halt {
		t.Fatal("promotion must be refused while a blocker is deferred")
	}
	if err == nil {
		t.Fatal("the JSON path must signal that it already printed")
	}

	var decoded map[string]any
	if jsonErr := json.Unmarshal([]byte(output), &decoded); jsonErr != nil {
		t.Fatalf("decoding %q: %v", output, jsonErr)
	}
	if decoded["success"] != false {
		t.Errorf("success = %v, want false", decoded["success"])
	}
	blockers, ok := decoded["deferred_blockers"].([]any)
	if !ok || len(blockers) != 1 || blockers[0] != deferredFixtureTask {
		t.Errorf("deferred_blockers = %v, want the one deferred task", decoded["deferred_blockers"])
	}
}

func TestForceGuardedOnlyWhenDeferred(t *testing.T) {
	t.Run("deferred runs the guard", func(t *testing.T) {
		festival := deferredFestival(t, true)
		calls := stubPromoteGuard(t, passingAudit(), nil)

		var halt bool
		var err error
		captureStdout(t, func() {
			halt, err = enforceDeferredBlockers(t.Context(), festival, "completed", &promoteOptions{force: true})
		})
		if err != nil || halt {
			t.Fatalf("enforceDeferredBlockers() = halt %v, err %v, want the forced completion to proceed", halt, err)
		}
		if *calls != 1 {
			t.Errorf("guard calls = %d, want 1", *calls)
		}
	})

	t.Run("nothing deferred never runs the guard", func(t *testing.T) {
		festival := deferredFestival(t, false)
		calls := stubPromoteGuard(t, nil, errors.Validation("the guard must not run with nothing deferred"))

		halt, err := enforceDeferredBlockers(t.Context(), festival, "completed", &promoteOptions{force: true})
		if err != nil || halt {
			t.Fatalf("enforceDeferredBlockers() = halt %v, err %v, want an untouched promotion", halt, err)
		}
		if *calls != 0 {
			t.Errorf("guard calls = %d, want 0", *calls)
		}
	})
}

// TestForceUnguardedWithNothingDeferred is the regression guard for existing
// users: --force off a terminal on a festival with nothing deferred must behave
// exactly as it did before this festival (D009).
func TestForceUnguardedWithNothingDeferred(t *testing.T) {
	festival := deferredFestival(t, false)
	restore := operatorGuardFn
	operatorGuardFn = func(context.Context, string) (*task.OperatorAudit, error) {
		t.Fatal("--force must not reach the operator guard when nothing is deferred")
		return nil, nil
	}
	t.Cleanup(func() { operatorGuardFn = restore })

	before := readEvents(t, festival.Path)

	for _, status := range []string{"completed", "dungeon/completed", "ready", "active"} {
		halt, err := enforceDeferredBlockers(t.Context(), festival, status, &promoteOptions{force: true})
		if halt || err != nil {
			t.Errorf("promotion to %q was disturbed: halt %v, err %v", status, halt, err)
		}
	}

	if after := readEvents(t, festival.Path); after != before {
		t.Error("an unrelated --force appended an event")
	}
}

func TestForceRefusedOffTTYWithDeferred(t *testing.T) {
	festival := deferredFestival(t, true)
	stubPromoteGuard(t, nil, errors.Validation("forced completion is an operator decision; run this from your terminal"))
	before := readEvents(t, festival.Path)

	halt, err := enforceDeferredBlockers(t.Context(), festival, "completed", &promoteOptions{force: true})
	if !halt {
		t.Fatal("a guard refusal must stop the promotion")
	}
	if err == nil || !strings.Contains(err.Error(), "run this from your terminal") {
		t.Fatalf("error = %v, want the guard refusal", err)
	}
	if after := readEvents(t, festival.Path); after != before {
		t.Error("a guard refusal appended an event")
	}
	if _, statErr := os.Stat(filepath.Join(festival.Path, shared.DroppedBlockerRecordFile)); statErr == nil {
		t.Error("a guard refusal wrote a dropped blocker record")
	}
}

func TestForcedCompletionAppendsEvent(t *testing.T) {
	festival := deferredFestival(t, true)
	stubPromoteGuard(t, passingAudit(), nil)

	captureStdout(t, func() {
		if halt, err := enforceDeferredBlockers(t.Context(), festival, "completed", &promoteOptions{force: true}); halt || err != nil {
			t.Fatalf("enforceDeferredBlockers() = halt %v, err %v", halt, err)
		}
	})

	events := readEvents(t, festival.Path)
	if !strings.Contains(events, `"event":"forced_complete"`) {
		t.Fatalf("no forced_complete event:\n%s", events)
	}
	for _, want := range []string{
		`"dropped_tasks":["` + deferredFixtureTask + `"]`,
		`"actor":"operator"`,
		`"tty":true`,
		`"deferred_by":"Ada Lovelace"`,
	} {
		if !strings.Contains(events, want) {
			t.Errorf("forced_complete event missing %s:\n%s", want, events)
		}
	}

	mgr, err := progress.NewManager(t.Context(), festival.Path)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	deferred, ok := mgr.GetTaskProgress(deferredFixtureTask)
	if !ok {
		t.Fatal("task missing from the store")
	}
	if !deferred.BlockerDeferred {
		t.Error("BlockerDeferred = false; a reopened festival must still sweep")
	}
}

func TestForcedCompletionRecordsDroppedTasks(t *testing.T) {
	festival := deferredFestival(t, true)
	stubPromoteGuard(t, passingAudit(), nil)

	captureStdout(t, func() {
		if halt, err := enforceDeferredBlockers(t.Context(), festival, "completed", &promoteOptions{force: true}); halt || err != nil {
			t.Fatalf("enforceDeferredBlockers() = halt %v, err %v", halt, err)
		}
	})

	data, err := os.ReadFile(filepath.Join(festival.Path, shared.DroppedBlockerRecordFile))
	if err != nil {
		t.Fatalf("reading the dropped blocker record: %v", err)
	}
	record := string(data)
	for _, want := range []string{
		"# Dropped blockers",
		"## " + deferredFixtureTask,
		"**Blocker:** provider API removed",
		"**Deferral reason:** the vendor replies next week",
		"**Deferred by:** Ada Lovelace",
		"**Attempts:**",
		"  - checked the changelog",
		"**Sweeps run:**",
	} {
		if !strings.Contains(record, want) {
			t.Errorf("record missing %q:\n%s", want, record)
		}
	}
}

// TestChainStatusMappingTreatsForcedCompletionAsComplete is design doc 05 D8
// and H10. Chains gate on festival status, so a forced completion is a
// completion and a festival mid sweep is not.
func TestChainStatusMappingTreatsForcedCompletionAsComplete(t *testing.T) {
	if got := mapFestivalStatus("completed"); got != "completed" {
		t.Errorf("mapFestivalStatus(completed) = %v, want completed", got)
	}
	if got := mapFestivalStatus("dungeon/completed"); got != "completed" {
		t.Errorf("mapFestivalStatus(dungeon/completed) = %v, want completed", got)
	}
	if got := mapFestivalStatus("active"); got == "completed" {
		t.Error("a festival running a sweep is still active, so a downstream chain festival must wait")
	}
}
