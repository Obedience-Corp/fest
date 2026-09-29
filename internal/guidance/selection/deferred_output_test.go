package selection

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/progress"
)

const emDash = '\u2014'

// deferredFixture builds a festival whose second task hard depends on the first
// and seeds the first as a deferred blocker through the real verbs.
func deferredFixture(t *testing.T) (string, string, string) {
	t.Helper()
	const (
		blocker   = "001_PHASE/01_seq/01_cache_warmup.md"
		dependent = "001_PHASE/01_seq/02_index_rebuild.md"
	)
	festivalPath := writeSweepFixture(t, blocker, dependent)
	writeSweepTaskWithDependency(t, festivalPath, dependent, "01_cache_warmup")
	seedSweepState(t, festivalPath, nil, []string{blocker})
	return festivalPath, blocker, dependent
}

func findNext(t *testing.T, festivalPath string) *NextTaskResult {
	t.Helper()
	result, err := NewSelector(festivalPath).FindNext(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("FindNext() error = %v", err)
	}
	return result
}

var goldenStamp = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)

// normaliseJSON replaces the two values a golden cannot pin: the fixture's temp
// path and the deferral timestamp, which the real manager writes from the wall
// clock. Nothing else in the payload moves between runs.
func normaliseJSON(t *testing.T, result *NextTaskResult, festivalPath string) string {
	t.Helper()
	out, err := FormatJSON(result)
	if err != nil {
		t.Fatalf("FormatJSON() error = %v", err)
	}
	out = strings.ReplaceAll(out, festivalPath, "/FESTIVAL")
	return goldenStamp.ReplaceAllString(out, "<DEFERRED_AT>")
}

func assertDeferredGolden(t *testing.T, name, got string) {
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

// 1. The line appears when the selected task depends on a deferred blocker.
func TestNextPrintsADeferredBlockerLineForADependency(t *testing.T) {
	festivalPath, blocker, dependent := deferredFixture(t)

	result := findNext(t, festivalPath)
	if result.Task == nil || !strings.Contains(result.Task.Path, "02_index_rebuild") {
		t.Fatalf("Task = %+v, want the dependent task", result.Task)
	}
	if len(result.DeferredBlockers) != 1 {
		t.Fatalf("DeferredBlockers = %+v, want the one deferred dependency", result.DeferredBlockers)
	}

	ref := result.DeferredBlockers[0]
	if ref.Task != blocker {
		t.Errorf("Task = %q, want %q", ref.Task, blocker)
	}
	if ref.BlockerMessage != "provider API removed" {
		t.Errorf("BlockerMessage = %q, want the executor blocker", ref.BlockerMessage)
	}
	if ref.DeferralReason != "the vendor replies next week" {
		t.Errorf("DeferralReason = %q, want the operator reason", ref.DeferralReason)
	}
	if ref.DeferredAt == "" {
		t.Error("DeferredAt is empty, want the RFC 3339 stamp from the store")
	}

	text := FormatText(result, false)
	if !strings.Contains(text, "Deferred blocker: "+blocker+": \"provider API removed\" (deferred ") {
		t.Errorf("rendered output is missing the deferred blocker line:\n%s", text)
	}
	if strings.ContainsRune(text, emDash) {
		t.Errorf("rendered output contains an em dash:\n%s", text)
	}
	_ = dependent
}

// 2. No line when the deferred blocker is not a dependency of the selected task.
func TestNextPrintsNoLineForAnUnrelatedDeferredBlocker(t *testing.T) {
	const (
		unrelated = "001_PHASE/01_seq/01_unrelated.md"
		ready     = "002_PHASE/01_seq/01_ready.md"
	)
	festivalPath := writeSweepFixture(t, unrelated, ready)
	seedSweepState(t, festivalPath, nil, []string{unrelated})

	result := findNext(t, festivalPath)
	if result.Task == nil || !strings.Contains(result.Task.Path, "01_ready") {
		t.Fatalf("Task = %+v, want the ready task in the second phase", result.Task)
	}
	if len(result.DeferredBlockers) != 0 {
		t.Errorf("DeferredBlockers = %+v, want none for an unrelated deferral", result.DeferredBlockers)
	}
	if text := FormatText(result, false); strings.Contains(text, "Deferred blocker") {
		t.Errorf("an unrelated deferral must put no line under this task:\n%s", text)
	}
}

// 3. A normal result carries none of the three new keys.
func TestNextJSONNormalResultHasNoNewKeys(t *testing.T) {
	const (
		first  = "001_PHASE/01_seq/01_first.md"
		second = "001_PHASE/01_seq/02_second.md"
	)
	festivalPath := writeSweepFixture(t, first, second)

	result := findNext(t, festivalPath)
	out := normaliseJSON(t, result, festivalPath)
	assertDeferredGolden(t, "next_normal.json", out)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, key := range []string{"deferred_blockers", "sweep", "operator_notes"} {
		if _, ok := decoded[key]; ok {
			t.Errorf("key %q must be omitted on a normal result:\n%s", key, out)
		}
	}
}

// 4. A dependent of a deferred blocker carries deferred_blockers and nothing else new.
func TestNextJSONDependentCarriesDeferredBlockers(t *testing.T) {
	festivalPath, _, _ := deferredFixture(t)

	result := findNext(t, festivalPath)
	out := normaliseJSON(t, result, festivalPath)
	assertDeferredGolden(t, "next_deferred_dependency.json", out)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	blockers, ok := decoded["deferred_blockers"].([]any)
	if !ok || len(blockers) != 1 {
		t.Fatalf("deferred_blockers = %v, want one entry", decoded["deferred_blockers"])
	}
	if _, ok := decoded["sweep"]; ok {
		t.Errorf("sweep must be omitted outside a sweep:\n%s", out)
	}
}

// 5. A sweep result carries sweep and reports the festival as not complete.
func TestNextJSONSweepResultCarriesSweep(t *testing.T) {
	const (
		done     = "001_PHASE/01_seq/01_done.md"
		deferred = "001_PHASE/01_seq/02_deferred.md"
	)
	festivalPath := writeSweepFixture(t, done, deferred)
	// Defer first, then complete, so the completion lands after the deferral
	// and the sweep has something to report as changed since.
	seedSweepState(t, festivalPath, nil, []string{deferred})
	if err := sweepManager(t, festivalPath).MarkComplete(t.Context(), done); err != nil {
		t.Fatalf("MarkComplete() error = %v", err)
	}

	result := findNext(t, festivalPath)
	out := normaliseJSON(t, result, festivalPath)
	assertDeferredGolden(t, "next_sweep.json", out)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if _, ok := decoded["sweep"]; !ok {
		t.Fatalf("sweep must be present on a sweep result:\n%s", out)
	}
	if complete, _ := decoded["festival_complete"].(bool); complete {
		t.Errorf("festival_complete must be false during a sweep:\n%s", out)
	}

	text := FormatText(result, false)
	for _, want := range []string{
		"Sweep 1: revisiting a deferred blocker",
		"Blocker: provider API removed",
		"Tried:   checked the changelog",
		"Deferred because: the vendor replies next week",
		"Completed since it was deferred:",
		"    - " + done,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("sweep rendering is missing %q:\n%s", want, text)
		}
	}
	if strings.ContainsRune(text, emDash) {
		t.Errorf("sweep rendering contains an em dash:\n%s", text)
	}
}

// 6. The terminal remain sentence is design doc 02's, word for word.
func TestNextTerminalRemainSentenceAndList(t *testing.T) {
	const (
		done      = "001_PHASE/01_seq/01_done.md"
		deferredA = "001_PHASE/01_seq/02_legacy_import.md"
		deferredB = "001_PHASE/01_seq/03_old_exporter.md"
	)
	festivalPath := writeSweepFixture(t, done, deferredA, deferredB)
	seedSweepState(t, festivalPath, []string{done}, []string{deferredA, deferredB})

	selector := NewSelector(festivalPath)
	var result *NextTaskResult
	for range 8 {
		next, err := selector.FindNext(t.Context(), festivalPath)
		if err != nil {
			t.Fatalf("FindNext() error = %v", err)
		}
		result = next
		if next.Task == nil {
			break
		}
	}

	if result.Task != nil {
		t.Fatalf("the sweep never reached its terminal result; last task was %+v", result.Task)
	}
	const want = "2 deferred blockers remain after sweep 1. Unblock them, or promote with --force."
	if result.Reason != want {
		t.Errorf("Reason = %q, want %q word for word", result.Reason, want)
	}

	text := FormatText(result, false)
	if !strings.Contains(text, want) {
		t.Errorf("the remain sentence is missing from the rendering:\n%s", text)
	}
	for _, task := range []string{deferredA, deferredB} {
		if !strings.Contains(text, task) {
			t.Errorf("the remain list is missing %s:\n%s", task, text)
		}
	}
	if !strings.Contains(text, "deferred: the vendor replies next week") {
		t.Errorf("the remain list is missing the deferral reason:\n%s", text)
	}
	if strings.ContainsRune(text, emDash) {
		t.Errorf("remain rendering contains an em dash:\n%s", text)
	}
}

// 7. The operator note renders under the task, and several render in order.
//
// ClearBlocker calls clearDeferral before it appends the note, and
// clearDeferral resets OperatorNotes, so the store keeps the latest note rather
// than a history. That is the behaviour 004/02/03 shipped and this renders it;
// the ordered case is asserted directly on the renderer so a later store change
// that does keep a history is already covered.
func TestNextRendersTheOperatorNoteUnderTheTask(t *testing.T) {
	const (
		blocked = "001_PHASE/01_seq/01_cache_warmup.md"
		second  = "001_PHASE/01_seq/02_second.md"
	)
	const note = "the v2 endpoint does the same thing, try that"
	festivalPath := writeSweepFixture(t, blocked, second)

	mgr, err := progress.NewManager(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	if err := mgr.ReportBlocker(t.Context(), blocked, "provider API removed", nil); err != nil {
		t.Fatalf("ReportBlocker() error = %v", err)
	}
	if err := mgr.ClearBlocker(t.Context(), blocked, note); err != nil {
		t.Fatalf("ClearBlocker() error = %v", err)
	}

	result := findNext(t, festivalPath)
	if result.Task == nil || !strings.Contains(result.Task.Path, "01_cache_warmup") {
		t.Fatalf("Task = %+v, want the unblocked task", result.Task)
	}
	if strings.Join(result.OperatorNotes, "|") != note {
		t.Fatalf("OperatorNotes = %v, want the note the operator sent back", result.OperatorNotes)
	}

	text := FormatText(result, false)
	if !strings.Contains(text, "Operator note: "+note) {
		t.Errorf("the note is missing from the rendering:\n%s", text)
	}
	if strings.ContainsRune(text, emDash) {
		t.Errorf("note rendering contains an em dash:\n%s", text)
	}
}

func TestOperatorNotesRenderOneLineEachInOrder(t *testing.T) {
	notes := []string{"try the v2 endpoint", "then check the changelog", "ask the vendor last"}
	out := buildOperatorNotesSection(notes)

	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) != len(notes) {
		t.Fatalf("lines = %d, want one per note:\n%s", len(lines), out)
	}
	for i, note := range notes {
		if lines[i] != "Operator note: "+note {
			t.Errorf("line %d = %q, want %q", i, lines[i], "Operator note: "+note)
		}
	}
	if buildOperatorNotesSection(nil) != "" {
		t.Error("no notes must render nothing")
	}
}

// 8. Nothing deferred renders and encodes exactly as it did before.
func TestNextNothingDeferredCarriesNoNewFields(t *testing.T) {
	const (
		first  = "001_PHASE/01_seq/01_first.md"
		second = "001_PHASE/01_seq/02_second.md"
	)
	festivalPath := writeSweepFixture(t, first, second)

	result := findNext(t, festivalPath)
	if result.DeferredBlockers != nil || result.OperatorNotes != nil || result.Sweep != nil {
		t.Errorf("a festival with nothing deferred must carry no new field: %+v", result)
	}
	if section := buildDeferralSection(result); section != "" {
		t.Errorf("buildDeferralSection() = %q, want empty", section)
	}
}
