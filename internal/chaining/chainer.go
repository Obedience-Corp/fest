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
	"github.com/Obedience-Corp/fest/internal/dirlink"
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
	saveConfig   func(string, string, *config.FestivalConfig) error
}

// NewChainer creates a Chainer for the given festival.
func NewChainer(festivalPath string) *Chainer {
	return &Chainer{festivalPath: festivalPath, saveConfig: config.SaveFestivalConfig}
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
// A chain journal under .fest/chaining/ records in-progress attempts so link or
// config save failures can be retried without creating a duplicate phase.
func (c *Chainer) ExecuteChain(ctx context.Context, target *ChainTarget) (*ChainResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if err := c.ensureChainedPhase(ctx, target); err != nil {
		return nil, err
	}

	inputLinked, linkErr := linkPhaseInput(ctx, target)
	if linkErr != nil {
		return nil, fmt.Errorf("linking phase input for %s: %w", target.PendingPhase.Name, linkErr)
	}

	if err := c.finalizeChain(ctx, target); err != nil {
		return nil, err
	}

	return &ChainResult{
		PhaseName:   target.PendingPhase.Name,
		PhaseType:   target.PendingPhase.Type,
		InputLinked: inputLinked,
	}, nil
}

func (c *Chainer) ensureChainedPhase(ctx context.Context, target *ChainTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	journal, err := loadChainJournal(c.festivalPath, target.PendingPhase.Name)
	if err != nil {
		return err
	}

	if journal != nil {
		matches, err := journalMatchesTarget(journal, target)
		if err != nil {
			return err
		}
		if !matches {
			return fmt.Errorf(
				"chain journal for %s does not match this completion (journal completed phase %q, got %q)",
				target.PendingPhase.Name,
				journal.CompletedPhasePath,
				target.CompletedPhasePath,
			)
		}
		if strings.TrimSpace(journal.PhaseDir) != "" {
			if _, statErr := os.Stat(journal.PhaseDir); statErr == nil {
				return nil
			}
			return fmt.Errorf("chain journal references missing phase directory %s", journal.PhaseDir)
		}
	} else if _, findErr := findCreatedPhaseDir(ctx, c.festivalPath, target.PendingPhase.Name); findErr == nil {
		return fmt.Errorf(
			"phase %s already exists without a chain recovery journal; resolve manually before chaining",
			target.PendingPhase.Name,
		)
	}

	if journal == nil {
		journal, err = newChainJournal(target)
		if err != nil {
			return err
		}
		if err := writeChainJournal(c.festivalPath, journal); err != nil {
			return fmt.Errorf("recording chain attempt for %s: %w", target.PendingPhase.Name, err)
		}
	}

	if dir, err := findCreatedPhaseDir(ctx, c.festivalPath, target.PendingPhase.Name); err == nil {
		journal.PhaseDir = dir
		if err := writeChainJournal(c.festivalPath, journal); err != nil {
			return fmt.Errorf("update chain journal for %s: %w", target.PendingPhase.Name, err)
		}
		return nil
	}

	opts := &festival.CreatePhaseOptions{
		Name:        target.PendingPhase.Name,
		PhaseType:   target.PendingPhase.Type,
		Path:        c.festivalPath,
		After:       -1,
		SkipMarkers: true,
	}
	if err := festival.RunCreatePhase(ctx, opts); err != nil {
		return fmt.Errorf("creating chained phase %s: %w", target.PendingPhase.Name, err)
	}

	phaseDir, err := findCreatedPhaseDir(ctx, c.festivalPath, target.PendingPhase.Name)
	if err != nil {
		return fmt.Errorf("locate chained phase %s after creation: %w", target.PendingPhase.Name, err)
	}
	journal.PhaseDir = phaseDir
	if err := writeChainJournal(c.festivalPath, journal); err != nil {
		return fmt.Errorf("update chain journal for %s: %w", target.PendingPhase.Name, err)
	}
	return nil
}

func (c *Chainer) finalizeChain(ctx context.Context, target *ChainTarget) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cfg, err := config.LoadFestivalConfig(c.festivalPath, "")
	if err != nil {
		return fmt.Errorf("updating festival config after phase creation: %w", err)
	}

	cfg.RemovePendingPhase(target.PendingPhase.Name)
	if err := c.saveConfig(c.festivalPath, "", cfg); err != nil {
		return fmt.Errorf("saving festival config: %w", err)
	}

	if err := clearChainJournal(c.festivalPath, target.PendingPhase.Name); err != nil {
		return fmt.Errorf("clearing chain journal for %s: %w", target.PendingPhase.Name, err)
	}
	return nil
}

// linkPhaseInput creates a directory link from the new phase's inputs/ directory to the
// completed phase's plan/ output. When plan output exists, linking failure is an error.
func linkPhaseInput(ctx context.Context, target *ChainTarget) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	sourcePlanDir := filepath.Join(target.CompletedPhasePath, "plan")
	info, err := os.Stat(sourcePlanDir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("inspect completed phase plan: %w", err)
	}
	if !info.IsDir() {
		return false, fmt.Errorf("completed phase plan is not a directory: %s", sourcePlanDir)
	}

	entries, err := os.ReadDir(sourcePlanDir)
	if err != nil {
		return false, fmt.Errorf("read completed phase plan: %w", err)
	}
	if len(entries) == 0 {
		return false, nil
	}

	if !isWithinFestival(target.FestivalPath, sourcePlanDir) {
		return false, fmt.Errorf("completed phase plan path escapes festival boundary")
	}

	newPhasePath, err := findCreatedPhaseDir(ctx, target.FestivalPath, target.PendingPhase.Name)
	if err != nil {
		return false, fmt.Errorf("locate chained phase directory: %w", err)
	}

	inputsDir := filepath.Join(newPhasePath, "inputs")
	if err := os.MkdirAll(inputsDir, 0755); err != nil {
		return false, fmt.Errorf("create inputs directory: %w", err)
	}

	completedPhaseName := filepath.Base(target.CompletedPhasePath)
	newPhaseName := filepath.Base(newPhasePath)
	relPath, err := filepath.Rel(
		filepath.Join(target.FestivalPath, newPhaseName, "inputs"),
		sourcePlanDir,
	)
	if err != nil {
		return false, fmt.Errorf("compute relative plan path: %w", err)
	}

	linkName := fmt.Sprintf("%s_output", target.CompletedPhaseType)
	linkPath := filepath.Join(inputsDir, linkName)
	createdLink := false
	kind, kindErr := dirlink.Kind(linkPath)
	switch {
	case kindErr == nil:
		if kind == dirlink.KindNone {
			return false, fmt.Errorf("inputs path %s already exists and is not a directory link", linkPath)
		}
		linkedInfo, linkedErr := os.Stat(linkPath)
		sourceInfo, sourceErr := os.Stat(sourcePlanDir)
		if linkedErr != nil || sourceErr != nil || !os.SameFile(linkedInfo, sourceInfo) {
			return false, fmt.Errorf("inputs link %s already exists with an unexpected or broken target", linkName)
		}
	case os.IsNotExist(kindErr):
		if err := dirlink.Create(linkPath, relPath); err != nil {
			return false, err
		}
		createdLink = true
	default:
		return false, fmt.Errorf("inspect inputs link %s: %w", linkName, kindErr)
	}

	resolved, err := filepath.EvalSymlinks(linkPath)
	if err != nil || !isWithinFestival(target.FestivalPath, resolved) {
		if createdLink {
			_ = dirlink.Remove(linkPath)
		}
		return false, fmt.Errorf("linked plan path escapes festival boundary")
	}

	readme := fmt.Sprintf("# Phase Inputs\n\nThis phase receives input from the completed %s phase.\n\n## Sources\n\n- `%s/` - Directory link to `%s/plan/` (auto-linked by phase chaining)\n",
		target.CompletedPhaseType, linkName, completedPhaseName)
	if err := os.WriteFile(filepath.Join(inputsDir, "README.md"), []byte(readme), 0644); err != nil {
		if createdLink {
			_ = dirlink.Remove(linkPath)
		}
		return false, fmt.Errorf("write inputs README: %w", err)
	}

	return true, nil
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
