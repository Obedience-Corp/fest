package guidance

import (
	"context"
	"os"
	"path/filepath"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
)

// InstructionHeader is the framing header prepended to actionable fest next output.
// It explicitly tells agents to treat the content as executable instructions.
const InstructionHeader = "\n" +
	"═══════════════════════════════════════════════════════════════════════════════\n" +
	"  EXECUTE THE FOLLOWING INSTRUCTIONS\n" +
	"  These are your direct instructions. Do not interpret, summarize, or skip steps.\n" +
	"  Execute each action exactly as written.\n" +
	"═══════════════════════════════════════════════════════════════════════════════\n"

// Guidance is the central interface for phase-type-aware navigation.
// It provides unified behavior for determining next steps, marking completion,
// and tracking progress across all phase types.
//
// All navigators (execution, planning, ingest, research, review, action)
// implement this interface, providing a consistent API regardless of the
// specific phase type being navigated.
//
// The interface is intentionally small and focused on core navigation operations.
// Additional lifecycle methods are provided by the Navigator interface which
// embeds Guidance.
type Guidance interface {
	// GetNext returns the next step/task based on current mode and position.
	// Returns nil (with no error) if all work is complete.
	//
	// The returned NextStep contains all information needed to execute the step,
	// including instructions, context files, and completion commands.
	GetNext(ctx context.Context) (*NextStep, error)

	// MarkComplete marks the specified step/task as complete.
	// The stepID should match the ID returned from GetNext().
	//
	// This updates the internal state and may trigger advancement
	// to the next step, sequence, or phase.
	MarkComplete(ctx context.Context, stepID string) error

	// MarkSkipped marks the specified step/task as skipped.
	// Skipped tasks are excluded from completion calculations but
	// do not count as failures.
	//
	// Use this when a task is not applicable or intentionally bypassed.
	MarkSkipped(ctx context.Context, stepID string) error

	// MarkFailed marks the specified step/task as failed.
	// Failed tasks may block progression depending on the mode's
	// failure handling policy.
	//
	// The navigator may require intervention before continuing
	// after a failure.
	MarkFailed(ctx context.Context, stepID string) error

	// GetProgress returns current progress information.
	// This includes counts, percentages, and phase-level breakdowns.
	//
	// Progress is calculated based on task statuses and the total
	// number of tasks in the current scope.
	GetProgress(ctx context.Context) (*Progress, error)

	// FormatInstructions generates agent-friendly instructions for the current state.
	// The output format is optimized for AI agent consumption, with clear
	// structure and actionable directives.
	//
	// The instructions include:
	// - Current position in the festival structure
	// - The next step to execute
	// - Context files to reference
	// - Completion commands to run when done
	FormatInstructions(ctx context.Context) (string, error)

	// GetContextFiles returns relevant context files for the current position.
	// These typically include goal files, rules, and other documents that
	// provide context for the current work.
	//
	// Files are returned as absolute paths.
	GetContextFiles(ctx context.Context) ([]string, error)
}

// NavigatorFactory is a function that creates a Navigator for a given context.
// Navigator packages register their factory functions during init().
type NavigatorFactory func(*GuidanceContext) (Navigator, error)

// navigatorFactories is the registry of navigator factories by mode.
// Each navigator package registers itself here during init().
var navigatorFactories = make(map[Mode]NavigatorFactory)

// RegisterNavigator registers a navigator factory for a mode.
// Called by navigator packages in their init() functions.
//
// Example:
//
//	func init() {
//	    guidance.RegisterNavigator(guidance.ModeImplementation, func(gctx *guidance.GuidanceContext) (guidance.Navigator, error) {
//	        return NewNavigator(gctx)
//	    })
//	}
func RegisterNavigator(mode Mode, factory NavigatorFactory) {
	navigatorFactories[mode] = factory
}

// MustRegisterNavigator registers a factory, panicking on error.
// Use this when registration failure is a programming error.
func MustRegisterNavigator(mode Mode, factory NavigatorFactory) {
	if factory == nil {
		panic("guidance: nil factory for mode " + string(mode))
	}
	if _, exists := navigatorFactories[mode]; exists {
		panic("guidance: duplicate factory for mode " + string(mode))
	}
	navigatorFactories[mode] = factory
}

// NewNavigator creates the appropriate navigator for the given context.
// It inspects the mode and returns a phase-type-aware navigator.
//
// The mode can be set explicitly in gctx.Mode, or will be derived from
// gctx.PhaseType using ModeFromPhaseType().
//
// Example:
//
//	gctx := guidance.NewGuidanceContext("/path/to/festival")
//	gctx = gctx.WithPhase("/path/to/phase", "Implementation", "implementation", 1)
//
//	nav, err := guidance.NewNavigator(ctx, gctx)
//	if err != nil {
//	    return err
//	}
//
//	if err := nav.Initialize(ctx); err != nil {
//	    return err
//	}
func NewNavigator(ctx context.Context, gctx *GuidanceContext) (Navigator, error) {
	if gctx == nil {
		return nil, errors.Validation("guidance context required")
	}

	// Validate context
	if err := gctx.Validate(); err != nil {
		return nil, errors.Wrap(err, "invalid guidance context")
	}

	// Default config if not provided
	if gctx.Config == nil {
		gctx.Config = DefaultConfig()
	}

	// Determine mode from phase type if not explicit
	mode := gctx.Mode
	if mode == "" {
		mode = ModeFromPhaseType(gctx.PhaseType)
		gctx.Mode = mode
	}

	// Create navigator based on mode
	factory, ok := navigatorFactories[mode]
	if !ok {
		return nil, errors.Validation("unknown guidance mode").
			WithField("mode", string(mode))
	}

	return factory(gctx)
}

// NewNavigatorForPath creates a navigator by detecting the phase type from path.
// This is a convenience wrapper that extracts phase type from the filesystem.
//
// The phase type is detected by reading PHASE_GOAL.md frontmatter in the phase directory.
// If no phase type is specified, it defaults to "implementation" (ModeImplementation).
func NewNavigatorForPath(ctx context.Context, festivalPath, phasePath string, config *GuidanceConfig) (Navigator, error) {
	// Detect phase type from path
	phaseType := DetectPhaseType(phasePath)

	// Extract phase name from path
	phaseName := filepath.Base(phasePath)

	gctx := &GuidanceContext{
		FestivalPath: festivalPath,
		FestivalName: filepath.Base(festivalPath),
		PhasePath:    phasePath,
		PhaseName:    phaseName,
		PhaseType:    phaseType,
		Mode:         ModeFromPhaseType(phaseType),
		Config:       config,
	}

	return NewNavigator(ctx, gctx)
}

// DetectPhaseType reads the phase type from a phase directory.
// It looks for PHASE_GOAL.md frontmatter for the fest_phase_type field.
//
// If the phase type cannot be determined, it defaults to "implementation".
func DetectPhaseType(phasePath string) string {
	// Try reading from PHASE_GOAL.md frontmatter
	goalPath := filepath.Join(phasePath, "PHASE_GOAL.md")

	content, err := os.ReadFile(goalPath)
	if err != nil {
		// File doesn't exist or can't be read, use default
		return "implementation"
	}

	fm, _, err := frontmatter.Parse(content)
	if err != nil || fm == nil {
		// Can't parse frontmatter or no frontmatter found, use default
		return "implementation"
	}

	if fm.PhaseType != "" {
		return string(fm.PhaseType)
	}

	// Default to implementation
	return "implementation"
}

// GetRegisteredModes returns a list of all modes that have registered navigators.
// This is useful for testing and debugging.
func GetRegisteredModes() []Mode {
	modes := make([]Mode, 0, len(navigatorFactories))
	for mode := range navigatorFactories {
		modes = append(modes, mode)
	}
	return modes
}

// ClearNavigatorRegistry removes all registered navigators.
// This is primarily for testing purposes.
func ClearNavigatorRegistry() {
	navigatorFactories = make(map[Mode]NavigatorFactory)
}
