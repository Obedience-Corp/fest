//go:build container_fs

package chaining

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/festival"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/dirlink"
	"gopkg.in/yaml.v3"
)

func TestCheckChaining(t *testing.T) {
	tests := []struct {
		name           string
		setupConfig    *config.FestivalConfig
		setupPhaseGoal bool
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

			writeFestConfig(t, festivalDir, tt.setupConfig)

			if tt.setupPhaseGoal {
				writePhaseGoal(t, phaseDir, "ingest")
			}

			ctx := context.Background()
			ch := NewChainer(festivalDir)
			target, err := ch.CheckChaining(ctx, phaseDir)

			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantTarget && target == nil {
				t.Error("expected chain target, got nil")
			}
			if !tt.wantTarget && target != nil {
				t.Errorf("expected no chain target, got %+v", target)
			}
		})
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
			got, err := linkPhaseInput(ctx, target)
			if err != nil {
				t.Fatalf("linkPhaseInput() error = %v", err)
			}

			if got != tt.wantLinked {
				t.Errorf("linkPhaseInput() = %v, want %v", got, tt.wantLinked)
			}

			if tt.wantLinked {
				linkPath := filepath.Join(planPhaseDir, "inputs", "ingest_output")
				linkTarget, err := dirlink.ReadTarget(linkPath)
				if err != nil || linkTarget == "" {
					t.Fatalf("expected link at %s: %v", linkPath, err)
				}
				linkedInfo, err := os.Stat(linkPath)
				if err != nil {
					t.Fatal(err)
				}
				sourceInfo, err := os.Stat(filepath.Join(ingestPhaseDir, "plan"))
				if err != nil || !os.SameFile(linkedInfo, sourceInfo) {
					t.Fatalf("input link does not resolve to source plan: %v", err)
				}
				readPath := filepath.Join(linkPath, "requirements.md")
				exe, err := os.Executable()
				if err != nil {
					t.Fatal(err)
				}
				child := exec.Command(exe, "-test.run=^TestPhaseInputChildReadback$")
				child.Env = append(os.Environ(), "FEST_CHAIN_INPUT_READBACK="+readPath)
				if output, err := child.CombinedOutput(); err != nil {
					t.Fatalf("separate-process phase input readback: %v\n%s", err, output)
				}

				readmePath := filepath.Join(planPhaseDir, "inputs", "README.md")
				if _, err := os.Stat(readmePath); os.IsNotExist(err) {
					t.Error("expected README.md in inputs/ directory")
				}
			}
		})
	}
}

func TestPhaseInputChildReadback(t *testing.T) {
	path := os.Getenv("FEST_CHAIN_INPUT_READBACK")
	if path == "" {
		return
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "test content" {
		t.Fatalf("phase input readback after process restart = %q, %v", got, err)
	}
}

func TestLinkPhaseInputFailureSurfaces(t *testing.T) {
	festivalDir := t.TempDir()
	ingestPhaseDir := filepath.Join(festivalDir, "001_INGEST")
	planPhaseDir := filepath.Join(festivalDir, "002_PLAN")
	planDir := filepath.Join(ingestPhaseDir, "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "x.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(planPhaseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePhaseGoal(t, planPhaseDir, "planning")

	// Block link creation by placing a plain file at the link path.
	inputsDir := filepath.Join(planPhaseDir, "inputs")
	if err := os.MkdirAll(inputsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(inputsDir, "ingest_output")
	if err := os.WriteFile(linkPath, []byte("not-a-link"), 0o644); err != nil {
		t.Fatal(err)
	}

	target := &ChainTarget{
		PendingPhase:       config.PendingPhase{Name: "PLAN", Type: "planning"},
		CompletedPhasePath: ingestPhaseDir,
		CompletedPhaseType: "ingest",
		FestivalPath:       festivalDir,
	}

	linked, err := linkPhaseInput(context.Background(), target)
	if err == nil {
		t.Fatal("expected link creation failure")
	}
	if linked {
		t.Fatal("expected linked=false on failure")
	}
}

func TestFindCreatedPhaseDir(t *testing.T) {
	festivalDir := t.TempDir()

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
		{name: "finds existing phase", phaseName: "PLAN", wantDir: "002_PLAN"},
		{name: "case insensitive", phaseName: "plan", wantDir: "002_PLAN"},
		{name: "not found", phaseName: "REVIEW", wantErr: true},
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

func TestIsWithinFestival(t *testing.T) {
	festivalDir := t.TempDir()
	unrelatedDir := filepath.Join(t.TempDir(), "unrelated")
	phaseDir := filepath.Join(festivalDir, "001_INGEST")
	if err := os.MkdirAll(phaseDir, 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		targetPath string
		want       bool
	}{
		{name: "path within festival", targetPath: filepath.Join(festivalDir, "001_INGEST", "plan"), want: true},
		{name: "festival root itself", targetPath: festivalDir, want: true},
		{name: "path outside festival", targetPath: filepath.Dir(festivalDir), want: false},
		{name: "completely unrelated path", targetPath: unrelatedDir, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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

func configWithChaining(trigger, phaseName, phaseType string) *config.FestivalConfig {
	cfg := config.DefaultFestivalConfig()
	cfg.TypeConfig = &config.TypeConfigMetadata{
		PendingPhases: []config.PendingPhase{{
			Name: phaseName, Type: phaseType, Trigger: trigger,
		}},
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

func installPlanningTemplate(t *testing.T, festivalsRoot string) {
	t.Helper()
	dst := filepath.Join(festivalsRoot, ".festival", "templates", "phases", "planning")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	// The native Windows proof copies only test executables into its container.
	// Keep this fixture independent of the source tree's working directory.
	const goal = `---
id: phase-goal-planning
phase_type: planning
fest_type: phase
fest_id: [REPLACE: PHASE_ID]
fest_name: [REPLACE: Phase Name]
fest_parent: [REPLACE: FESTIVAL_ID]
fest_order: [REPLACE: N]
fest_phase_type: planning
fest_status: pending
fest_tracking: true
---
# Phase Goal: [REPLACE: Phase Name]
`
	if err := os.WriteFile(filepath.Join(dst, "GOAL.md"), []byte(goal), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setupChainExecuteFixture(t *testing.T) (festivalDir, ingestDir string, target *ChainTarget) {
	t.Helper()
	root := t.TempDir()
	festivalsRoot := filepath.Join(root, "festivals")
	festivalDir = filepath.Join(festivalsRoot, "active", "chain-test")
	installPlanningTemplate(t, festivalsRoot)

	ingestDir = filepath.Join(festivalDir, "001_INGEST")
	if err := os.MkdirAll(ingestDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writePhaseGoal(t, ingestDir, "ingest")

	planDir := filepath.Join(ingestDir, "plan")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(planDir, "requirements.md"), []byte("req"), 0o644); err != nil {
		t.Fatal(err)
	}

	writeFestConfig(t, festivalDir, configWithChaining("phase_complete:ingest", "PLAN", "planning"))

	target = &ChainTarget{
		PendingPhase:       config.PendingPhase{Name: "PLAN", Type: "planning", Trigger: "phase_complete:ingest"},
		CompletedPhasePath: ingestDir,
		CompletedPhaseType: "ingest",
		FestivalPath:       festivalDir,
	}
	return festivalDir, ingestDir, target
}

func countPhaseDirsNamed(t *testing.T, festivalDir, phaseName string) int {
	t.Helper()
	entries, err := os.ReadDir(festivalDir)
	if err != nil {
		t.Fatal(err)
	}
	upper := strings.ToUpper(phaseName)
	n := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.Contains(strings.ToUpper(e.Name()), "_"+upper) {
			n++
		}
	}
	return n
}

func pendingIncludes(t *testing.T, festivalDir, name string) bool {
	t.Helper()
	cfg, err := config.LoadFestivalConfig(festivalDir, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range cfg.TypeConfig.PendingPhases {
		if strings.EqualFold(p.Name, name) {
			return true
		}
	}
	return false
}

func TestExecuteChainLinkFailureThenRetry(t *testing.T) {
	festivalDir, _, target := setupChainExecuteFixture(t)
	ch := NewChainer(festivalDir)
	ctx := context.Background()

	if err := festival.RunCreatePhase(ctx, &festival.CreatePhaseOptions{
		Name: "PLAN", PhaseType: "planning", Path: festivalDir, After: -1, SkipMarkers: true,
	}); err != nil {
		t.Fatalf("create phase for partial chain fixture: %v", err)
	}
	planDir, err := findCreatedPhaseDir(ctx, festivalDir, "PLAN")
	if err != nil {
		t.Fatal(err)
	}
	journal, err := newChainJournal(target)
	if err != nil {
		t.Fatal(err)
	}
	journal.PhaseDir = planDir
	if err := writeChainJournal(festivalDir, journal); err != nil {
		t.Fatal(err)
	}

	inputsDir := filepath.Join(planDir, "inputs")
	if err := os.MkdirAll(inputsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(inputsDir, "ingest_output")
	if err := os.WriteFile(linkPath, []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err = ch.ExecuteChain(ctx, target)
	if err == nil {
		t.Fatal("expected link failure")
	}
	if countPhaseDirsNamed(t, festivalDir, "PLAN") != 1 {
		t.Fatal("expected exactly one PLAN phase directory after partial chain")
	}
	if !pendingIncludes(t, festivalDir, "PLAN") {
		t.Fatal("pending PLAN should remain after link failure")
	}
	if _, err := loadChainJournal(festivalDir, "PLAN"); err != nil {
		t.Fatalf("expected chain journal: %v", err)
	}

	if err := os.Remove(linkPath); err != nil {
		t.Fatal(err)
	}

	result, err := ch.ExecuteChain(ctx, target)
	if err != nil {
		t.Fatalf("retry ExecuteChain: %v", err)
	}
	if !result.InputLinked {
		t.Fatal("expected input linked on retry")
	}
	if countPhaseDirsNamed(t, festivalDir, "PLAN") != 1 {
		t.Fatal("retry must not create a duplicate PLAN phase")
	}
	if pendingIncludes(t, festivalDir, "PLAN") {
		t.Fatal("pending PLAN should be cleared after successful chain")
	}
	if j, _ := loadChainJournal(festivalDir, "PLAN"); j != nil {
		t.Fatal("chain journal should be cleared after success")
	}
}

func TestExecuteChainUnrelatedSameNamePhaseCollision(t *testing.T) {
	festivalDir, _, target := setupChainExecuteFixture(t)
	unrelated := filepath.Join(festivalDir, "002_PLAN")
	if err := os.MkdirAll(unrelated, 0o755); err != nil {
		t.Fatal(err)
	}
	writePhaseGoal(t, unrelated, "planning")

	ch := NewChainer(festivalDir)
	_, err := ch.ExecuteChain(context.Background(), target)
	if err == nil {
		t.Fatal("expected collision error")
	}
	if !strings.Contains(err.Error(), "without a chain recovery journal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExecuteChainConfigSaveFailureThenRetry(t *testing.T) {
	festivalDir, _, target := setupChainExecuteFixture(t)
	ch := NewChainer(festivalDir)
	ctx := context.Background()
	ch.saveConfig = func(string, string, *config.FestivalConfig) error {
		return errors.New("injected config save failure")
	}

	_, err := ch.ExecuteChain(ctx, target)
	if err == nil {
		t.Fatal("expected config save failure")
	}
	if !strings.Contains(err.Error(), "saving festival config") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pendingIncludes(t, festivalDir, "PLAN") {
		t.Fatal("pending should remain when config save fails")
	}
	phaseDir, err := findCreatedPhaseDir(ctx, festivalDir, "PLAN")
	if err != nil {
		t.Fatalf("phase should exist for retry: %v", err)
	}
	if journal, err := loadChainJournal(festivalDir, "PLAN"); err != nil || journal == nil || journal.PhaseDir != phaseDir {
		t.Fatalf("expected recovery journal for %s: journal=%+v err=%v", phaseDir, journal, err)
	}

	ch.saveConfig = config.SaveFestivalConfig
	result, err := ch.ExecuteChain(ctx, target)
	if err != nil {
		t.Fatalf("retry after config save fix: %v", err)
	}
	if !result.InputLinked {
		t.Fatal("expected linked input on retry")
	}
	if pendingIncludes(t, festivalDir, "PLAN") {
		t.Fatal("pending should be cleared after successful retry")
	}
	if j, _ := loadChainJournal(festivalDir, "PLAN"); j != nil {
		t.Fatal("journal should be cleared")
	}
}
