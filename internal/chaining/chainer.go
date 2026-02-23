// Package chaining provides automatic phase transition on workflow completion.
// When a workflow-based phase (ingest, planning, research) completes, the chainer
// checks fest.yaml for pending phases with matching triggers and creates them.
package chaining

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/festival"
	"github.com/Obedience-Corp/fest/internal/config"
	festivalPkg "github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/guidance"
)

const (
	// TriggerPrefix is the prefix for phase-complete triggers.
	TriggerPrefix = "phase_complete:"
)

// ChainTarget describes what phase should be created next.
type ChainTarget struct {
	// PendingPhase is the pending phase config from fest.yaml.
	PendingPhase config.PendingPhase

	// CompletedPhasePath is the filesystem path of the completed phase.
	CompletedPhasePath string

	// CompletedPhaseType is the type of the completed phase (e.g., "ingest").
	CompletedPhaseType string

	// FestivalPath is the festival root directory.
	FestivalPath string
}

// ChainResult describes what was created by chaining.
type ChainResult struct {
	// PhaseName is the name of the created phase (e.g., "PLAN").
	PhaseName string

	// PhaseType is the type of the created phase (e.g., "planning").
	PhaseType string

	// InputLinked indicates whether input was linked from the completed phase.
	InputLinked bool
}

// Chainer checks for and executes phase chaining on workflow completion.
type Chainer struct {
	festivalPath string
}

// NewChainer creates a Chainer for the given festival.
func NewChainer(festivalPath string) *Chainer {
	return &Chainer{festivalPath: festivalPath}
}

// CheckChaining determines if a pending phase should be created after the
// given phase completes. Returns nil if no chaining is configured.
func (c *Chainer) CheckChaining(ctx context.Context, completedPhasePath string) (*ChainTarget, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Detect the completed phase's type
	completedType := guidance.DetectPhaseType(completedPhasePath)

	// Load festival config
	cfg, err := config.LoadFestivalConfig(c.festivalPath, "")
	if err != nil {
		return nil, fmt.Errorf("loading festival config: %w", err)
	}

	if !cfg.HasPendingPhases() {
		return nil, nil
	}

	// Find a pending phase whose trigger matches this completion
	for _, pending := range cfg.TypeConfig.PendingPhases {
		if matchesTrigger(pending.Trigger, completedType) {
			return &ChainTarget{
				PendingPhase:       pending,
				CompletedPhasePath: completedPhasePath,
				CompletedPhaseType: completedType,
				FestivalPath:       c.festivalPath,
			}, nil
		}
	}

	return nil, nil
}

// ExecuteChain creates the next phase from the chain target.
func (c *Chainer) ExecuteChain(ctx context.Context, target *ChainTarget) (*ChainResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	opts := &festival.CreatePhaseOptions{
		Name:        target.PendingPhase.Name,
		PhaseType:   target.PendingPhase.Type,
		Path:        target.FestivalPath,
		After:       -1, // Auto-detect position (append at end)
		SkipMarkers: true,
	}

	if err := festival.RunCreatePhase(ctx, opts); err != nil {
		return nil, fmt.Errorf("creating chained phase %s: %w", target.PendingPhase.Name, err)
	}

	// Remove the pending phase from config now that it's been created
	cfg, err := config.LoadFestivalConfig(c.festivalPath, "")
	if err != nil {
		return nil, fmt.Errorf("updating festival config after phase creation: %w", err)
	}

	cfg.RemovePendingPhase(target.PendingPhase.Name)
	if err := config.SaveFestivalConfig(c.festivalPath, "", cfg); err != nil {
		return nil, fmt.Errorf("saving festival config: %w", err)
	}

	// Link input from the completed phase to the new phase
	inputLinked := linkPhaseInput(ctx, target)

	return &ChainResult{
		PhaseName:   target.PendingPhase.Name,
		PhaseType:   target.PendingPhase.Type,
		InputLinked: inputLinked,
	}, nil
}

// linkPhaseInput creates a symlink from the new phase's inputs/ directory to the
// completed phase's plan/ output. Returns true if linking succeeded.
func linkPhaseInput(ctx context.Context, target *ChainTarget) bool {
	if err := ctx.Err(); err != nil {
		return false
	}

	// Check that the completed phase has a plan/ directory with output
	sourcePlanDir := filepath.Join(target.CompletedPhasePath, "plan")
	info, err := os.Stat(sourcePlanDir)
	if err != nil || !info.IsDir() {
		return false
	}

	// Check if plan/ has any files
	entries, err := os.ReadDir(sourcePlanDir)
	if err != nil || len(entries) == 0 {
		return false
	}

	// Validate source stays within festival boundary
	if !isWithinFestival(target.FestivalPath, sourcePlanDir) {
		return false
	}

	// Find the newly created phase directory
	newPhasePath, err := findCreatedPhaseDir(ctx, target.FestivalPath, target.PendingPhase.Name)
	if err != nil {
		return false
	}

	// Create inputs/ directory in the new phase
	inputsDir := filepath.Join(newPhasePath, "inputs")
	if err := os.MkdirAll(inputsDir, 0755); err != nil {
		return false
	}

	// Create relative symlink using phase type as link name
	completedPhaseName := filepath.Base(target.CompletedPhasePath)
	newPhaseName := filepath.Base(newPhasePath)
	relPath, err := filepath.Rel(
		filepath.Join(target.FestivalPath, newPhaseName, "inputs"),
		sourcePlanDir,
	)
	if err != nil {
		return false
	}

	linkName := fmt.Sprintf("%s_output", target.CompletedPhaseType)
	linkPath := filepath.Join(inputsDir, linkName)
	if err := os.Symlink(relPath, linkPath); err != nil {
		return false
	}

	// Verify the resolved symlink stays within the festival boundary
	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil || !isWithinFestival(target.FestivalPath, resolved) {
		// Remove the unsafe symlink
		_ = os.Remove(linkPath)
		return false
	}

	// Write inputs/README.md documenting the source
	readme := fmt.Sprintf("# Phase Inputs\n\nThis phase receives input from the completed %s phase.\n\n## Sources\n\n- `%s/` - Symlink to `%s/plan/` (auto-linked by phase chaining)\n",
		target.CompletedPhaseType, linkName, completedPhaseName)
	_ = os.WriteFile(filepath.Join(inputsDir, "README.md"), []byte(readme), 0644)

	return true
}

// isWithinFestival verifies that a path resolves to within the festival directory.
func isWithinFestival(festivalPath, targetPath string) bool {
	festAbs, err := filepath.Abs(festivalPath)
	if err != nil {
		return false
	}

	// Resolve symlinks in the festival path itself
	festResolved, err := filepath.EvalSymlinks(festAbs)
	if err != nil {
		return false
	}

	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		return false
	}

	// Resolve symlinks in the target path
	targetResolved, err := filepath.EvalSymlinks(targetAbs)
	if err != nil {
		// If EvalSymlinks fails (e.g. path doesn't exist yet), check the abs path
		targetResolved = targetAbs
	}

	return strings.HasPrefix(targetResolved, festResolved+string(filepath.Separator)) ||
		targetResolved == festResolved
}

// findCreatedPhaseDir locates the directory for a phase by name within the festival.
func findCreatedPhaseDir(ctx context.Context, festivalPath, phaseName string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	parser := festivalPkg.NewParser()
	phases, err := parser.ParsePhases(ctx, festivalPath)
	if err != nil {
		return "", err
	}

	upperName := strings.ToUpper(phaseName)
	for _, p := range phases {
		if strings.ToUpper(p.Name) == upperName {
			return filepath.Join(festivalPath, p.FullName), nil
		}
	}

	return "", fmt.Errorf("phase %s not found in festival", phaseName)
}

// matchesTrigger checks if a pending phase trigger matches the completed phase type.
func matchesTrigger(trigger, completedType string) bool {
	if trigger == "" {
		return false
	}

	// Handle "phase_complete:<type>" format
	if triggerType, ok := strings.CutPrefix(trigger, TriggerPrefix); ok {
		return strings.EqualFold(triggerType, completedType)
	}

	// Handle legacy format like "research_complete"
	if triggerType, ok := strings.CutSuffix(trigger, "_complete"); ok {
		return strings.EqualFold(triggerType, completedType)
	}

	return false
}
