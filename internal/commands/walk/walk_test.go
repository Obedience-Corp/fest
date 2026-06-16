package walk

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
)

func makeMinimalFestival(t *testing.T, root, name, status string) string {
	t.Helper()
	dir := filepath.Join(root, "festivals", status, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "FESTIVAL_GOAL.md"), []byte("A test goal line.\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func makePhaseAndTask(t *testing.T, festivalDir, phaseName, seqName, taskName string) {
	t.Helper()
	phaseDir := filepath.Join(festivalDir, phaseName)
	seqDir := filepath.Join(phaseDir, seqName)
	if err := os.MkdirAll(seqDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte("# Phase\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte("# Seq\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, taskName), []byte("# Task\n"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestDetectKind_Festival(t *testing.T) {
	root := t.TempDir()
	festDir := makeMinimalFestival(t, root, "my-fest-MF0001", "active")
	kind := detectKind(festDir, "active")
	if kind != "festival" {
		t.Errorf("detectKind() = %q, want %q", kind, "festival")
	}
}

func TestDetectKind_RitualTemplate(t *testing.T) {
	root := t.TempDir()
	ritualDir := filepath.Join(root, "festivals", "ritual")
	festDir := filepath.Join(ritualDir, "my-ritual-RI-MR0001")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "FESTIVAL_GOAL.md"), []byte("goal\n"), 0644); err != nil {
		t.Fatal(err)
	}
	kind := detectKind(festDir, "ritual")
	if kind != "ritual-template" {
		t.Errorf("detectKind() = %q, want %q", kind, "ritual-template")
	}
}

func TestDetectKind_DungeonRun(t *testing.T) {
	root := t.TempDir()
	festDir := makeMinimalFestival(t, root, "old-fest-OF0001", "dungeon/completed")
	kind := detectKind(festDir, "dungeon/completed")
	if kind != "dungeon-run" {
		t.Errorf("detectKind() = %q, want %q", kind, "dungeon-run")
	}
}

func TestLoadGoal_ReturnsFirstNonHeaderLine(t *testing.T) {
	festDir := t.TempDir()
	content := "---\nfest_status: active\n---\n# Title\n\nThis is the goal text.\n"
	if err := os.WriteFile(filepath.Join(festDir, "FESTIVAL_GOAL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	goal := loadGoal(festDir)
	if goal != "This is the goal text." {
		t.Errorf("loadGoal() = %q, want %q", goal, "This is the goal text.")
	}
}

func TestLoadGoal_MissingFile(t *testing.T) {
	festDir := t.TempDir()
	goal := loadGoal(festDir)
	if goal != "" {
		t.Errorf("loadGoal() = %q, want empty", goal)
	}
}

func TestHasNumericPrefix_Walk(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"001_PLAN", true},
		{"01_setup", true},
		{"001", false},
		{"abc_test", false},
		{"", false},
	}
	for _, tc := range cases {
		got := hasNumericPrefix(tc.name)
		if got != tc.want {
			t.Errorf("hasNumericPrefix(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestResolveNext_ReturnsFirstIncompleteTask(t *testing.T) {
	root := t.TempDir()
	festDir := makeMinimalFestival(t, root, "my-fest-MF0001", "active")
	makePhaseAndTask(t, festDir, "001_IMPL", "01_core", "01_first_task.md")

	next := resolveNext(context.Background(), festDir, festDir)
	if next == "" {
		t.Fatal("resolveNext() returned empty string, want a task path")
	}
	if !strings.Contains(next, "01_first_task.md") {
		t.Errorf("resolveNext() = %q, want to contain '01_first_task.md'", next)
	}
}

func TestResolveNext_EmptyFestival(t *testing.T) {
	root := t.TempDir()
	festDir := makeMinimalFestival(t, root, "empty-fest-EF0001", "active")

	next := resolveNext(context.Background(), festDir, festDir)
	if next != "" {
		t.Errorf("resolveNext() on empty festival = %q, want empty", next)
	}
}

func TestRunWalk_JSONOutput(t *testing.T) {
	root := t.TempDir()
	festDir := makeMinimalFestival(t, root, "my-fest-MF0001", "active")
	makePhaseAndTask(t, festDir, "001_IMPL", "01_core", "01_task.md")

	origCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCWD) }()

	if err := os.Chdir(festDir); err != nil {
		t.Fatal(err)
	}

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	os.Stdout = w

	runErr := runWalk(context.Background(), &walkOptions{json: true})

	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, readErr := buf.ReadFrom(r); readErr != nil {
		t.Fatal(readErr)
	}

	if runErr != nil {
		t.Fatalf("runWalk() error: %v", runErr)
	}

	var view WalkView
	if unmarshalErr := json.Unmarshal(buf.Bytes(), &view); unmarshalErr != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", unmarshalErr, buf.String())
	}

	if view.Name == "" {
		t.Error("WalkView.Name is empty")
	}
	if view.Kind == "" {
		t.Error("WalkView.Kind is empty")
	}
	if view.Status == "" {
		t.Error("WalkView.Status is empty")
	}
}

func TestRunWalk_ErrorOutsideFestival(t *testing.T) {
	tmpDir := t.TempDir()

	origCWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(origCWD) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}

	err = runWalk(context.Background(), &walkOptions{})
	if err == nil {
		t.Fatal("runWalk() expected error outside festival, got nil")
	}
}

func TestRunWalk_WithFromFlag(t *testing.T) {
	root := t.TempDir()
	festDir := makeMinimalFestival(t, root, "my-fest-MF0001", "active")

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	os.Stdout = w

	err := runWalk(context.Background(), &walkOptions{json: true, from: festDir})

	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, readErr := buf.ReadFrom(r); readErr != nil {
		t.Fatal(readErr)
	}

	if err != nil {
		t.Fatalf("runWalk() with --from error: %v", err)
	}

	var view WalkView
	if unmarshalErr := json.Unmarshal(buf.Bytes(), &view); unmarshalErr != nil {
		t.Fatalf("failed to parse JSON: %v\noutput: %s", unmarshalErr, buf.String())
	}
	if view.Name != "my-fest-MF0001" {
		t.Errorf("WalkView.Name = %q, want %q", view.Name, "my-fest-MF0001")
	}
}

func TestKindWarnings_RitualTemplate(t *testing.T) {
	root := t.TempDir()
	ritualDir := filepath.Join(root, "festivals", "ritual")
	festDir := filepath.Join(ritualDir, "my-ritual-RI-MR0001")
	if err := os.MkdirAll(festDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "FESTIVAL_GOAL.md"), []byte("goal\n"), 0644); err != nil {
		t.Fatal(err)
	}

	festival := &show.FestivalInfo{
		Path: festDir,
		Name: "my-ritual-RI-MR0001",
	}
	warnings := kindWarnings("ritual-template", festival)
	if len(warnings) == 0 {
		t.Fatal("kindWarnings() returned no warnings for ritual-template kind")
	}
	if !strings.Contains(warnings[0], "ritual template") {
		t.Errorf("warning = %q, want to mention 'ritual template'", warnings[0])
	}
}

func TestKindWarnings_NonRitual(t *testing.T) {
	root := t.TempDir()
	festDir := makeMinimalFestival(t, root, "my-fest-MF0001", "active")

	festival := &show.FestivalInfo{
		Path: festDir,
		Name: "my-fest-MF0001",
	}
	warnings := kindWarnings("festival", festival)
	if len(warnings) != 0 {
		t.Errorf("kindWarnings() returned %d warnings for non-ritual, want 0", len(warnings))
	}
}

func TestRunWalk_GoalInOutput(t *testing.T) {
	root := t.TempDir()
	festDir := makeMinimalFestival(t, root, "my-fest-MF0001", "active")

	origStdout := os.Stdout
	r, w, pipeErr := os.Pipe()
	if pipeErr != nil {
		t.Fatal(pipeErr)
	}
	os.Stdout = w

	err := runWalk(context.Background(), &walkOptions{from: festDir})

	if closeErr := w.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	os.Stdout = origStdout

	var buf bytes.Buffer
	if _, readErr := buf.ReadFrom(r); readErr != nil {
		t.Fatal(readErr)
	}

	if err != nil {
		t.Fatalf("runWalk() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "A test goal line.") {
		t.Errorf("output missing goal text; got:\n%s", output)
	}
}
