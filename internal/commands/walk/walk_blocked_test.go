package walk

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// captureWalkText runs the festival text emitter against a hand-built view and
// returns its stdout. Nothing in the Blocked section is derived from the clock,
// so the output is deterministic and needs no normalisation.
func captureWalkText(t *testing.T, view *WalkView) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		out, _ := io.ReadAll(r)
		done <- string(out)
	}()

	emitErr := emitText(view, "/camp/festivals/active/walk-demo", "/camp")

	_ = w.Close()
	os.Stdout = orig
	out := <-done
	_ = r.Close()

	if emitErr != nil {
		t.Fatalf("emitText() error = %v", emitErr)
	}
	return out
}

func walkBlockedView(blocked ...walkBlocker) *WalkView {
	return &WalkView{
		Kind:    "festival",
		Name:    "walk-demo",
		Status:  "active",
		Path:    "/camp/festivals/active/walk-demo",
		Blocked: blocked,
	}
}

func assertWalkGolden(t *testing.T, name, got string) {
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

// TestWalkBlockedNothingDeferredIsUnchanged is golden 1. It was captured from
// the Blocked section before the deferred suffix existed, covering both the
// entry that carries a blocker reason and the entry that does not.
func TestWalkBlockedNothingDeferredIsUnchanged(t *testing.T) {
	out := captureWalkText(t, walkBlockedView(
		walkBlocker{Task: "001_PHASE/01_seq/01_cache_warmup.md", Reason: "provider API removed"},
		walkBlocker{Task: "001_PHASE/01_seq/02_legacy_import.md"},
	))

	assertWalkGolden(t, "walk_blocked_nothing_deferred.txt", out)

	if strings.Contains(out, "deferred") {
		t.Errorf("nothing deferred must print no deferred suffix:\n%s", out)
	}
}

// TestWalkBlockedDeferredEntryKeepsItsPlace is golden 2: the suffix appears and
// the deferred entry is still listed alongside the open one.
func TestWalkBlockedDeferredEntryKeepsItsPlace(t *testing.T) {
	out := captureWalkText(t, walkBlockedView(
		walkBlocker{Task: "001_PHASE/01_seq/01_cache_warmup.md", Reason: "provider API removed"},
		walkBlocker{
			Task:           "001_PHASE/01_seq/02_legacy_import.md",
			Reason:         "upstream schema gone",
			Deferred:       true,
			DeferralReason: "revisit after the v2 migration lands",
		},
	))

	assertWalkGolden(t, "walk_blocked_one_deferred.txt", out)

	if !strings.Contains(out, "001_PHASE/01_seq/02_legacy_import.md upstream schema gone deferred\n") {
		t.Errorf("the deferred entry must keep its reason and gain the suffix:\n%s", out)
	}
	if !strings.Contains(out, "001_PHASE/01_seq/01_cache_warmup.md provider API removed\n") {
		t.Errorf("the open entry must be unchanged:\n%s", out)
	}
	if strings.Contains(out, "revisit after the v2 migration lands") {
		t.Errorf("the walk scan must not render the deferral reason:\n%s", out)
	}
}

// TestWalkBlockedDeferredWithNoReasonHasNoDoubleSpace is golden 3, the one that
// catches the spacing bug the single-path restructure can introduce.
func TestWalkBlockedDeferredWithNoReasonHasNoDoubleSpace(t *testing.T) {
	out := captureWalkText(t, walkBlockedView(
		walkBlocker{Task: "001_PHASE/01_seq/02_legacy_import.md", Deferred: true},
	))

	assertWalkGolden(t, "walk_blocked_deferred_no_reason.txt", out)

	if !strings.Contains(out, "001_PHASE/01_seq/02_legacy_import.md deferred\n") {
		t.Errorf("a deferred entry with no blocker reason must have one space:\n%s", out)
	}
	if strings.Contains(out, "  deferred") {
		t.Errorf("double space before the deferred suffix:\n%q", out)
	}
}

// TestWalkBlockerJSONIsAdditive pins the contract rule: the deferral fields are
// omitted entirely when nothing is deferred, so an existing consumer sees the
// same bytes it saw before they existed.
func TestWalkBlockerJSONIsAdditive(t *testing.T) {
	encoded, err := json.Marshal(walkBlocker{Task: "t.md", Reason: "why"})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got, want := string(encoded), `{"task":"t.md","reason":"why"}`; got != want {
		t.Errorf("open blocker JSON = %s, want %s", got, want)
	}

	encoded, err = json.Marshal(walkBlocker{
		Task: "t.md", Reason: "why", Deferred: true, DeferralReason: "later",
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if got, want := string(encoded),
		`{"task":"t.md","reason":"why","deferred":true,"deferral_reason":"later"}`; got != want {
		t.Errorf("deferred blocker JSON = %s, want %s", got, want)
	}
}
