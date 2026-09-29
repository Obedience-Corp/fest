package selection

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// blockedFixture blocks the first task of a sequence, which the implicit
// sequential dependency turns into a stalled festival.
func blockedFixture(t *testing.T) (string, string, string) {
	t.Helper()
	const (
		blocked   = "001_PHASE/01_seq/01_cache_warmup.md"
		dependent = "001_PHASE/01_seq/02_index_rebuild.md"
	)
	festivalPath := writeSweepFixture(t, blocked, dependent)

	mgr, err := progress.NewManager(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := mgr.ReportBlocker(t.Context(), blocked, "provider API removed", []string{"checked the changelog"}); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	return festivalPath, blocked, dependent
}

// TestBlockedGuidanceIncludesPolicy asserts placement, not wording, so it uses
// the constant rather than repeating the sentence.
func TestBlockedGuidanceIncludesPolicy(t *testing.T) {
	festivalPath, blocked, _ := blockedFixture(t)

	result := findNext(t, festivalPath)
	text := FormatText(result, false)

	if !strings.Contains(text, guidance.ExecutorBlockerPolicy) {
		t.Errorf("blocked guidance is missing the executor blocker policy:\n%s", text)
	}
	if !strings.Contains(text, blocked) || !strings.Contains(text, "provider API removed") {
		t.Errorf("blocked guidance must name the blocker it is about:\n%s", text)
	}
	if strings.Index(text, guidance.ExecutorBlockerPolicy) < strings.Index(text, "provider API removed") {
		t.Errorf("the policy must sit under the blocker message:\n%s", text)
	}
	if strings.ContainsRune(text, '—') {
		t.Errorf("blocked guidance contains an em dash:\n%s", text)
	}

	if verbose := FormatVerbose(result, false); !strings.Contains(verbose, guidance.ExecutorBlockerPolicy) {
		t.Errorf("verbose blocked guidance is missing the policy:\n%s", verbose)
	}
}

// TestNextReturnsBlockedTaskNotDependent is design doc 05 scenario A1: an
// executor reports a blocker, runs fest next hoping to move on, and gets
// neither the blocked task as work nor its dependent.
func TestNextReturnsBlockedTaskNotDependent(t *testing.T) {
	festivalPath, blocked, dependent := blockedFixture(t)

	result := findNext(t, festivalPath)
	if result.Task != nil {
		t.Fatalf("Task = %+v, want no task while the blocker is open and undeferred", result.Task)
	}
	for _, parallel := range result.ParallelTasks {
		if strings.Contains(parallel.Path, "02_index_rebuild") {
			t.Errorf("the dependent of a blocked task must never be offered: %+v", parallel)
		}
	}

	out, err := FormatJSON(result)
	if err != nil {
		t.Fatalf("FormatJSON() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if _, ok := decoded["task"]; ok {
		t.Errorf("a stalled festival must offer no task:\n%s", out)
	}
	if strings.Contains(out, dependent) {
		t.Errorf("the dependent must not appear in the result:\n%s", out)
	}

	refs, ok := decoded["blocked_tasks"].([]any)
	if !ok || len(refs) != 1 {
		t.Fatalf("blocked_tasks = %v, want the one open blocker", decoded["blocked_tasks"])
	}
	if _, ok := decoded["deferred_blockers"]; ok {
		t.Errorf("nothing is deferred, so deferred_blockers must be omitted:\n%s", out)
	}
	_ = blocked
}

// TestBlockedSectionOmitsADeferredBlocker keeps the two states apart: once an
// operator defers, the sweep owns the blocker and the stalled-festival guidance
// is no longer what the executor is looking at.
func TestBlockedSectionOmitsADeferredBlocker(t *testing.T) {
	const (
		blocked   = "001_PHASE/01_seq/01_cache_warmup.md"
		dependent = "001_PHASE/01_seq/02_index_rebuild.md"
	)
	festivalPath := writeSweepFixture(t, blocked, dependent)
	seedSweepState(t, festivalPath, nil, []string{blocked})

	result := findNext(t, festivalPath)
	if len(result.BlockedTasks) != 0 {
		t.Errorf("BlockedTasks = %+v, want none once the blocker is deferred", result.BlockedTasks)
	}
	if result.Task == nil || !strings.Contains(result.Task.Path, "02_index_rebuild") {
		t.Fatalf("Task = %+v, want the dependent to become ready", result.Task)
	}
}

func TestBlockedSectionRendersNothingWhenEmpty(t *testing.T) {
	if got := buildBlockedSection(nil); got != "" {
		t.Errorf("buildBlockedSection(nil) = %q, want empty", got)
	}
}
