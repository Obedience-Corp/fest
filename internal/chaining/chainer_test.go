package chaining

import (
	"context"
	"path/filepath"
	"testing"
)

func TestMatchesTrigger(t *testing.T) {
	tests := []struct {
		name          string
		trigger       string
		completedType string
		want          bool
	}{
		{
			name:          "phase_complete prefix matches",
			trigger:       "phase_complete:ingest",
			completedType: "ingest",
			want:          true,
		},
		{
			name:          "phase_complete prefix case insensitive",
			trigger:       "phase_complete:INGEST",
			completedType: "ingest",
			want:          true,
		},
		{
			name:          "phase_complete prefix no match",
			trigger:       "phase_complete:research",
			completedType: "ingest",
			want:          false,
		},
		{
			name:          "legacy suffix matches",
			trigger:       "research_complete",
			completedType: "research",
			want:          true,
		},
		{
			name:          "legacy suffix case insensitive",
			trigger:       "INGEST_complete",
			completedType: "ingest",
			want:          true,
		},
		{
			name:          "legacy suffix no match",
			trigger:       "ingest_complete",
			completedType: "research",
			want:          false,
		},
		{
			name:          "empty trigger",
			trigger:       "",
			completedType: "ingest",
			want:          false,
		},
		{
			name:          "manual trigger no match",
			trigger:       "manual",
			completedType: "ingest",
			want:          false,
		},
		{
			name:          "auto trigger no match",
			trigger:       "auto",
			completedType: "ingest",
			want:          false,
		},
		{
			name:          "planning phase complete",
			trigger:       "phase_complete:planning",
			completedType: "planning",
			want:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesTrigger(tt.trigger, tt.completedType)
			if got != tt.want {
				t.Errorf("matchesTrigger(%q, %q) = %v, want %v", tt.trigger, tt.completedType, got, tt.want)
			}
		})
	}
}

func TestLinkPhaseInputContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	target := &ChainTarget{
		CompletedPhasePath: "/nonexistent",
		FestivalPath:       "/nonexistent",
	}

	got, err := linkPhaseInput(ctx, target)
	if err == nil {
		t.Fatal("expected context error")
	}
	if got {
		t.Error("expected false from cancelled context")
	}
}

func TestFindCreatedPhaseDirContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := findCreatedPhaseDir(ctx, "/nonexistent", "PLAN")
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestNewChainer(t *testing.T) {
	ch := NewChainer("/some/path")
	if ch == nil {
		t.Fatal("expected non-nil chainer")
	}
	if ch.festivalPath != "/some/path" {
		t.Errorf("festivalPath = %q, want %q", ch.festivalPath, "/some/path")
	}
}

func TestChainJournalPathConfinesPendingName(t *testing.T) {
	festival := "/festival"
	path := chainJournalPath(festival, "../../outside")
	if filepath.Dir(path) != chainJournalDir(festival) {
		t.Fatalf("journal path escaped chaining directory: %s", path)
	}
}
