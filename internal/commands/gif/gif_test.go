package gif

import (
	"bytes"
	"context"
	"encoding/json"
	"image/gif"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/pkg/festgif"
	replay "github.com/Obedience-Corp/fest/pkg/festgif/festival"
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
	return writeFestivalEvents(t, rejectedThenApproved())
}

// writeFestivalEvents lays out the same festival with the given event log,
// written against phase 002_IMPLEMENT and moved onto the on-disk 001 phase.
func writeFestivalEvents(t *testing.T, log []progress.ProgressEvent) string {
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
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, e := range log {
		if err := enc.Encode(e); err != nil {
			t.Fatal(err)
		}
	}
	events := strings.ReplaceAll(buf.String(), `"gate:002_IMPLEMENT"`, `"gate:001_IMPLEMENT"`)
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

func TestGifOutputIsReadableByOthers(t *testing.T) {
	dir := writeFestival(t)
	t.Chdir(dir)
	cmd := NewGifCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fest gif: %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, "demo-DM0001.gif"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("mode = %o, want 644 (the temp file's 0600 must not survive the rename)", got)
	}
}

func TestGifAcceptsAFestivalPath(t *testing.T) {
	festival := writeFestival(t)
	elsewhere := t.TempDir()
	t.Chdir(elsewhere)
	for _, target := range []string{festival, filepath.Join(festival, "001_IMPLEMENT")} {
		cmd := NewGifCommand()
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetArgs([]string{target})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("fest gif %s: %v", target, err)
		}
		if _, err := os.Stat(filepath.Join(elsewhere, "demo-DM0001.gif")); err != nil {
			t.Fatalf("expected demo-DM0001.gif in the current directory for %s: %v", target, err)
		}
	}
}

func TestGifPathWithoutMarkersFallsThroughToNameLookup(t *testing.T) {
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	cmd := NewGifCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"plain"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("a directory without festival markers must not render")
	}
	if errors.Is(err, errors.ErrCodeValidation) {
		t.Fatalf("err = %v, want a lookup failure, not validation", err)
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

// The held last frame must show every row exactly as fest show does,
// including parents of a blocked gate.
func TestLastFrameMatchesFestShow(t *testing.T) {
	l := &eventLog{}
	l.add(progress.ProgressEvent{Event: progress.EventCompleted, Task: task})
	l.gate(progress.EventWorkflowStepStart, progress.ProgressEvent{})
	l.gate(progress.EventWorkflowJudgeStarted, progress.ProgressEvent{JudgeStatus: "running", JudgeRunID: "a"})
	l.gate(progress.EventWorkflowJudgeReturned, progress.ProgressEvent{JudgeStatus: "rejected", JudgeRunID: "a"})
	l.gate(progress.EventWorkflowStepBlock, progress.ProgressEvent{DecisionActor: "agent", Feedback: "missing evidence"})
	dir := writeFestivalEvents(t, l.events)

	ctx := context.Background()
	tree, err := show.BuildFestivalTree(ctx, dir)
	if err != nil {
		t.Fatalf("BuildFestivalTree: %v", err)
	}
	in, err := replay.Load(ctx, dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	r := festgif.Plan(in, festgif.DefaultTiming)
	roll := r.Rollups(r.StateAt(r.Frames - 1))

	var want []string
	var walk func(n *show.DisplayNode)
	walk = func(n *show.DisplayNode) {
		want = append(want, n.Name+"="+n.Status)
		for _, c := range n.Children {
			walk(c)
		}
	}
	for _, p := range tree.Children {
		walk(p)
	}
	var got []string
	for i, row := range r.Rows {
		name := row.Label
		if row.Kind == festgif.KindTask {
			name += ".md"
		}
		got = append(got, name+"="+roll[i].Status)
	}
	equal(t, got, want)
	if tree.Children[0].Status != "blocked" {
		t.Fatalf("fixture should leave the phase blocked in fest show, got %s", tree.Children[0].Status)
	}
}

func TestGifEmbedUsesFestivalDirectory(t *testing.T) {
	dir := writeFestival(t)
	t.Chdir(filepath.Join(dir, "001_IMPLEMENT", "01_build"))
	cmd := NewGifCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"--embed"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	overview, err := os.ReadFile(filepath.Join(dir, replay.OverviewFilename))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overview), "![Festival execution replay](festival-replay.gif)") {
		t.Fatalf("missing replay embed: %s", overview)
	}
	if _, err := os.Stat(filepath.Join(dir, replay.ReplayFilename)); err != nil {
		t.Fatal(err)
	}
}

func TestGifRejectsEmbedWithOut(t *testing.T) {
	cmd := NewGifCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--embed", "--out", "another.gif"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "embed") {
		t.Fatalf("expected mutually exclusive flags error, got %v", err)
	}
}

func TestGifRejectsEmbedWithMP4(t *testing.T) {
	cmd := NewGifCommand()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--embed", "--mp4"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "embed") {
		t.Fatalf("expected mutually exclusive flags error, got %v", err)
	}
}

func TestGifWritesMP4(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}
	dir := writeFestival(t)
	t.Chdir(dir)
	out := filepath.Join(t.TempDir(), "replay.mp4")
	cmd := NewGifCommand()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"--mp4", "--speed", "8", "--out", out})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), out) {
		t.Fatalf("stdout = %s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "demo-DM0001.gif")); err == nil {
		t.Fatal("mp4 export also wrote a gif")
	}
	probe := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height", "-of", "csv=p=0", out)
	got, err := probe.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != "h264,1080,1920" {
		t.Fatalf("probe = %q", got)
	}
}

func TestChooseOutput(t *testing.T) {
	gifPath, mp4, err := chooseOutput("demo", "", false)
	if err != nil || mp4 || gifPath != "demo.gif" {
		t.Fatalf("default gif = %q mp4=%v err=%v", gifPath, mp4, err)
	}
	mp4Path, mp4, err := chooseOutput("demo", "", true)
	if err != nil || !mp4 || mp4Path != "demo.mp4" {
		t.Fatalf("default mp4 = %q mp4=%v err=%v", mp4Path, mp4, err)
	}
	mp4Path, mp4, err = chooseOutput("demo", "replay.MP4", false)
	if err != nil || !mp4 || mp4Path != "replay.MP4" {
		t.Fatalf("extension mp4 = %q mp4=%v err=%v", mp4Path, mp4, err)
	}
	if _, _, err := chooseOutput("demo", "replay.gif", true); err == nil || !strings.Contains(err.Error(), "MP4") {
		t.Fatalf("gif path with --mp4: %v", err)
	}
	if _, _, err := chooseOutput("demo", "replay.mov", false); err == nil || !strings.Contains(err.Error(), ".gif or .mp4") {
		t.Fatalf("mov path: %v", err)
	}
}
