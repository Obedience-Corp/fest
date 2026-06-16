package progress

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
)

func TestResolveTaskID_WithTaskPath(t *testing.T) {
	festivalDir := setupFestival(t, []string{
		"002_FOUNDATION/01_alpha/01_design.md",
	})

	taskPath := filepath.Join(festivalDir, "002_FOUNDATION", "01_alpha", "01_design.md")
	opts := &progressOptions{taskPath: taskPath}

	got, err := resolveTaskID(festivalDir, opts)
	if err != nil {
		t.Fatalf("resolveTaskID() error = %v", err)
	}

	want := filepath.ToSlash(filepath.Join("002_FOUNDATION", "01_alpha", "01_design.md"))
	if got != want {
		t.Errorf("resolveTaskID() = %q, want %q", got, want)
	}
}

func TestResolveTaskID_PhaseSequence(t *testing.T) {
	festivalDir := setupFestival(t, []string{
		"002_FOUNDATION/01_alpha/01_design.md",
	})

	opts := &progressOptions{
		taskID:   "01_design",
		phase:    "002_FOUNDATION",
		sequence: "01_alpha",
	}

	got, err := resolveTaskID(festivalDir, opts)
	if err != nil {
		t.Fatalf("resolveTaskID() error = %v", err)
	}

	want := filepath.ToSlash(filepath.Join("002_FOUNDATION", "01_alpha", "01_design.md"))
	if got != want {
		t.Errorf("resolveTaskID() = %q, want %q", got, want)
	}
}

func TestResolveTaskID_UniqueBareName(t *testing.T) {
	festivalDir := setupFestival(t, []string{
		"002_FOUNDATION/01_alpha/02_unique.md",
		"003_OTHER/01_beta/01_design.md",
	})

	opts := &progressOptions{taskID: "02_unique.md"}

	got, err := resolveTaskID(festivalDir, opts)
	if err != nil {
		t.Fatalf("resolveTaskID() error = %v", err)
	}

	want := filepath.ToSlash(filepath.Join("002_FOUNDATION", "01_alpha", "02_unique.md"))
	if got != want {
		t.Errorf("resolveTaskID() = %q, want %q", got, want)
	}
}

func TestResolveTaskID_AmbiguousBareName(t *testing.T) {
	festivalDir := setupFestival(t, []string{
		"002_FOUNDATION/01_alpha/01_design.md",
		"003_OTHER/01_beta/01_design.md",
	})

	opts := &progressOptions{taskID: "01_design.md"}

	_, err := resolveTaskID(festivalDir, opts)
	if err == nil {
		t.Fatal("resolveTaskID() expected error, got nil")
	}
	if errors.Code(err) != errors.ErrCodeValidation {
		t.Fatalf("resolveTaskID() error code = %q, want %q", errors.Code(err), errors.ErrCodeValidation)
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("resolveTaskID() error = %q, want ambiguous error", err.Error())
	}
}

func setupFestival(t *testing.T, taskPaths []string) string {
	t.Helper()

	dir := t.TempDir()
	for _, rel := range taskPaths {
		writeTask(t, filepath.Join(dir, rel))
	}
	return dir
}

func writeTask(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("# Task\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestProgressLocationGranularity_SubdirUsescwd(t *testing.T) {
	root := t.TempDir()

	festivalPath := filepath.Join(root, "festivals", "active", "my-fest-MF0001")
	phaseDir := filepath.Join(festivalPath, "001_IMPL")
	seqDir := filepath.Join(phaseDir, "01_core")

	for _, d := range []string{seqDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(festivalPath, "FESTIVAL_GOAL.md"), []byte("goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte("# Phase\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seqDir, "SEQUENCE_GOAL.md"), []byte("# Seq\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	locFromPhase, err := show.DetectCurrentLocation(ctx, phaseDir)
	if err != nil {
		t.Fatalf("DetectCurrentLocation(phaseDir) error = %v", err)
	}
	if locFromPhase.Type != "phase" {
		t.Errorf("from phase subdir: loc.Type = %q, want %q (regression: #204 used festival root, always producing 'festival')", locFromPhase.Type, "phase")
	}

	locFromSeq, err := show.DetectCurrentLocation(ctx, seqDir)
	if err != nil {
		t.Fatalf("DetectCurrentLocation(seqDir) error = %v", err)
	}
	if locFromSeq.Type != "sequence" {
		t.Errorf("from sequence subdir: loc.Type = %q, want %q (regression: #204 used festival root, always producing 'festival')", locFromSeq.Type, "sequence")
	}

	locFromRoot, err := show.DetectCurrentLocation(ctx, festivalPath)
	if err != nil {
		t.Fatalf("DetectCurrentLocation(festivalPath) error = %v", err)
	}
	if locFromRoot.Type != "festival" {
		t.Errorf("from festival root: loc.Type = %q, want %q", locFromRoot.Type, "festival")
	}
}
