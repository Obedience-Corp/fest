package gif

import (
	"bytes"
	"encoding/json"
	"image/gif"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
)

const gatesMD = `---
fest_type: phase_gate
fest_id: 001_IMPLEMENT-GATE
---

# Implementation Phase Gate

## Step 1: COMPLETENESS - Verify Completeness

**Checkpoint:** APPROVAL REQUIRED
`

// writeFestival lays out a one-phase festival with a task, a gate, and an
// event log in which the judge rejects the gate once and then approves it.
func writeFestival(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "demo-DM0001")
	seq := filepath.Join(dir, "001_IMPLEMENT", "01_build")
	files := map[string]string{
		filepath.Join(dir, "FESTIVAL_GOAL.md"):               "# Demo\n",
		filepath.Join(dir, "001_IMPLEMENT", "GATES.md"):      gatesMD,
		filepath.Join(seq, "01_task.md"):                     "---\nfest_type: task\nfest_tracking: true\n---\n# Task\n",
		filepath.Join(dir, ".fest", "progress_events.jsonl"): "",
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var log bytes.Buffer
	enc := json.NewEncoder(&log)
	for _, e := range rejectedThenApproved() {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	events := strings.ReplaceAll(log.String(), `"gate:002_IMPLEMENT"`, `"gate:001_IMPLEMENT"`)
	events = strings.ReplaceAll(events, `"002_IMPLEMENT/01_build/01_task.md"`, `"001_IMPLEMENT/01_build/01_task.md"`)
	if err := os.WriteFile(filepath.Join(dir, ".fest", "progress_events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGifWritesTheFestivalReplay(t *testing.T) {
	dir := writeFestival(t)
	t.Chdir(dir)

	var out bytes.Buffer
	cmd := NewGifCommand()
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fest gif: %v", err)
	}

	path := filepath.Join(dir, "demo-DM0001.gif")
	if !strings.Contains(out.String(), "Wrote "+path) {
		t.Fatalf("output %q does not name %s", out.String(), path)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening GIF: %v", err)
	}
	defer func() { _ = f.Close() }()
	g, err := gif.DecodeAll(f)
	if err != nil {
		t.Fatalf("decoding GIF: %v", err)
	}
	if len(g.Image) < 2 {
		t.Fatalf("expected an animation, got %d frames", len(g.Image))
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, ".fest-gif-*.tmp"))
	if len(leftovers) > 0 {
		t.Fatalf("temporary files left behind: %v", leftovers)
	}
}

func TestGifHonorsOut(t *testing.T) {
	dir := writeFestival(t)
	t.Chdir(dir)
	cmd := NewGifCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"-o", "out/replay.gif"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fest gif: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "replay.gif")); err != nil {
		t.Fatalf("expected out/replay.gif relative to the current directory: %v", err)
	}
}

func TestGifOutsideAFestivalExplainsHowToPickOne(t *testing.T) {
	t.Chdir(t.TempDir())
	cmd := NewGifCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(nil)
	err := cmd.Execute()
	if !errors.Is(err, errors.ErrCodeNotFound) {
		t.Fatalf("err = %v, want NOT_FOUND", err)
	}
	if !strings.Contains(err.Error(), "--festival") {
		t.Fatalf("hint should point at --festival: %v", err)
	}
}

func TestGifRejectsNameAndSelectorTogether(t *testing.T) {
	cmd := NewGifCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"demo", "--festival", "DM0001"})
	if err := cmd.Execute(); !errors.Is(err, errors.ErrCodeValidation) {
		t.Fatalf("err = %v, want VALIDATION", err)
	}
}
