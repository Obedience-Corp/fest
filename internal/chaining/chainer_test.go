package chaining

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/fest/internal/config"
	"gopkg.in/yaml.v3"
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

func TestCheckChaining(t *testing.T) {
	tests := []struct {
		name           string
		setupConfig    *config.FestivalConfig
		setupPhaseGoal bool // create PHASE_GOAL.md with ingest type
		wantTarget     bool
		wantErr        bool
	}{
		{
			name:           "chain configured for ingest completion",
			setupConfig:    configWithChaining("phase_complete:ingest", "PLAN", "planning"),
			setupPhaseGoal: true,
			wantTarget:     true,
		},
		{
			name:           "no pending phases",
			setupConfig:    configWithoutChaining(),
			setupPhaseGoal: true,
			wantTarget:     false,
		},
		{
			name:           "pending phase with manual trigger",
			setupConfig:    configWithChaining("manual", "IMPLEMENT", "implementation"),
			setupPhaseGoal: true,
			wantTarget:     false,
		},
		{
			name:           "pending phase trigger doesnt match completed type",
			setupConfig:    configWithChaining("phase_complete:research", "SYNTHESIZE", "planning"),
			setupPhaseGoal: true,
			wantTarget:     false,
		},
		{
			name:           "legacy trigger format",
			setupConfig:    configWithChaining("ingest_complete", "PLAN", "planning"),
			setupPhaseGoal: true,
			wantTarget:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festivalDir := t.TempDir()
			phaseDir := filepath.Join(festivalDir, "001_INGEST")
			if err := os.MkdirAll(phaseDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Write fest.yaml
			writeFestConfig(t, festivalDir, tt.setupConfig)

			// Create PHASE_GOAL.md with ingest type
			if tt.setupPhaseGoal {
				writePhaseGoal(t, phaseDir, "ingest")
			}

			ctx := context.Background()
			ch := NewChainer(festivalDir)
			target, err := ch.CheckChaining(ctx, phaseDir)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tt.wantTarget && target == nil {
				t.Error("expected chain target, got nil")
			}
			if !tt.wantTarget && target != nil {
				t.Errorf("expected no chain target, got %+v", target)
			}

			if target != nil {
				if target.CompletedPhaseType != "ingest" {
					t.Errorf("CompletedPhaseType = %q, want %q", target.CompletedPhaseType, "ingest")
				}
				if target.FestivalPath != festivalDir {
					t.Errorf("FestivalPath = %q, want %q", target.FestivalPath, festivalDir)
				}
			}
		})
	}
}

func TestCheckChainingContextCancellation(t *testing.T) {
	festivalDir := t.TempDir()
	phaseDir := filepath.Join(festivalDir, "001_INGEST")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ch := NewChainer(festivalDir)
	_, err := ch.CheckChaining(ctx, phaseDir)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestLinkPhaseInput(t *testing.T) {
	tests := []struct {
		name         string
		setupPlanDir bool
		planFiles    []string
		wantLinked   bool
	}{
		{
			name:         "links when plan directory has files",
			setupPlanDir: true,
			planFiles:    []string{"requirements.md", "context.md"},
			wantLinked:   true,
		},
		{
			name:         "no link when plan directory missing",
			setupPlanDir: false,
			wantLinked:   false,
		},
		{
			name:         "no link when plan directory empty",
			setupPlanDir: true,
			planFiles:    nil,
			wantLinked:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			festivalDir := t.TempDir()
			ingestPhaseDir := filepath.Join(festivalDir, "001_INGEST")
			planPhaseDir := filepath.Join(festivalDir, "002_PLAN")

			if err := os.MkdirAll(ingestPhaseDir, 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(planPhaseDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Setup plan directory with files
			if tt.setupPlanDir {
				planDir := filepath.Join(ingestPhaseDir, "plan")
				if err := os.MkdirAll(planDir, 0755); err != nil {
					t.Fatal(err)
				}
				for _, f := range tt.planFiles {
					if err := os.WriteFile(filepath.Join(planDir, f), []byte("test content"), 0644); err != nil {
						t.Fatal(err)
					}
				}
			}

			// Write a PHASE_GOAL.md in planPhaseDir so findCreatedPhaseDir can locate it
			writePhaseGoal(t, planPhaseDir, "planning")

			target := &ChainTarget{
				PendingPhase: config.PendingPhase{
					Name: "PLAN",
					Type: "planning",
				},
				CompletedPhasePath: ingestPhaseDir,
				CompletedPhaseType: "ingest",
				FestivalPath:       festivalDir,
			}

			ctx := context.Background()
			got := linkPhaseInput(ctx, target)

			if got != tt.wantLinked {
				t.Errorf("linkPhaseInput() = %v, want %v", got, tt.wantLinked)
			}

			if tt.wantLinked {
				// Verify symlink exists and points to correct location
				linkPath := filepath.Join(planPhaseDir, "inputs", "ingest_output") // type is "ingest" so link is "ingest_output"
				linkTarget, err := os.Readlink(linkPath)
				if err != nil {
					t.Fatalf("expected symlink at %s: %v", linkPath, err)
				}

				// Resolve the symlink and check it points to the plan directory
				resolved := filepath.Join(planPhaseDir, "inputs", linkTarget)
				resolvedAbs, _ := filepath.Abs(resolved)
				expectedAbs, _ := filepath.Abs(filepath.Join(ingestPhaseDir, "plan"))
				if resolvedAbs != expectedAbs {
					t.Errorf("symlink resolves to %q, want %q", resolvedAbs, expectedAbs)
				}

				// Verify README.md was created
				readmePath := filepath.Join(planPhaseDir, "inputs", "README.md")
				if _, err := os.Stat(readmePath); os.IsNotExist(err) {
					t.Error("expected README.md in inputs/ directory")
				}
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

	got := linkPhaseInput(ctx, target)
	if got {
		t.Error("expected false from cancelled context")
	}
}

func TestFindCreatedPhaseDir(t *testing.T) {
	festivalDir := t.TempDir()

	// Create phase directories matching parser's expected format
	for _, dir := range []string{"001_INGEST", "002_PLAN", "003_IMPLEMENT"} {
		if err := os.MkdirAll(filepath.Join(festivalDir, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name      string
		phaseName string
		wantDir   string
		wantErr   bool
	}{
		{
			name:      "finds existing phase",
			phaseName: "PLAN",
			wantDir:   "002_PLAN",
		},
		{
			name:      "case insensitive",
			phaseName: "plan",
			wantDir:   "002_PLAN",
		},
		{
			name:      "not found",
			phaseName: "REVIEW",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := findCreatedPhaseDir(ctx, festivalDir, tt.phaseName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			expected := filepath.Join(festivalDir, tt.wantDir)
			if got != expected {
				t.Errorf("findCreatedPhaseDir() = %q, want %q", got, expected)
			}
		})
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

func TestIsWithinFestival(t *testing.T) {
	festivalDir := t.TempDir()
	phaseDir := filepath.Join(festivalDir, "001_INGEST")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		targetPath string
		want       bool
	}{
		{
			name:       "path within festival",
			targetPath: filepath.Join(festivalDir, "001_INGEST", "plan"),
			want:       true,
		},
		{
			name:       "festival root itself",
			targetPath: festivalDir,
			want:       true,
		},
		{
			name:       "path outside festival",
			targetPath: filepath.Dir(festivalDir),
			want:       false,
		},
		{
			name:       "completely unrelated path",
			targetPath: "/tmp",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure target exists for EvalSymlinks to work
			if _, err := os.Stat(tt.targetPath); os.IsNotExist(err) {
				if err := os.MkdirAll(tt.targetPath, 0755); err != nil {
					t.Skip("cannot create target path")
				}
			}

			got := isWithinFestival(festivalDir, tt.targetPath)
			if got != tt.want {
				t.Errorf("isWithinFestival(%q, %q) = %v, want %v", festivalDir, tt.targetPath, got, tt.want)
			}
		})
	}
}

// Helper functions

func configWithChaining(trigger, phaseName, phaseType string) *config.FestivalConfig {
	cfg := config.DefaultFestivalConfig()
	cfg.TypeConfig = &config.TypeConfigMetadata{
		PendingPhases: []config.PendingPhase{
			{
				Name:    phaseName,
				Type:    phaseType,
				Trigger: trigger,
			},
		},
	}
	return cfg
}

func configWithoutChaining() *config.FestivalConfig {
	return config.DefaultFestivalConfig()
}

func writeFestConfig(t *testing.T, festivalDir string, cfg *config.FestivalConfig) {
	t.Helper()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festivalDir, "fest.yaml"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func writePhaseGoal(t *testing.T, phaseDir, phaseType string) {
	t.Helper()
	content := "---\nfest_type: phase\nfest_phase_type: " + phaseType + "\n---\n# Phase Goal\n"
	if err := os.WriteFile(filepath.Join(phaseDir, "PHASE_GOAL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
