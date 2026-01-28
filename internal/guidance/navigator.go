package guidance

import (
	"context"
	"errors"
)

// Navigator extends Guidance with additional navigation capabilities.
// All phase-type-specific navigators implement this interface.
//
// While Guidance defines the core operations (GetNext, MarkComplete, etc.),
// Navigator adds lifecycle management, state access, and configuration.
//
// Usage:
//
//	nav, err := guidance.NewNavigator(ctx, gctx)
//	if err != nil {
//	    return err
//	}
//
//	if err := nav.Initialize(ctx); err != nil {
//	    return err
//	}
//
//	for {
//	    step, err := nav.GetNext(ctx)
//	    if err != nil {
//	        return err
//	    }
//	    if step == nil {
//	        break // All complete
//	    }
//	    // Process step...
//	    nav.MarkComplete(ctx, step.ID)
//	}
type Navigator interface {
	Guidance

	// Initialize loads state and prepares the navigator for use.
	// Must be called before other methods.
	//
	// This loads persisted state from disk, scans the festival structure,
	// and prepares internal caches. If state exists, navigation resumes
	// from the saved position.
	Initialize(ctx context.Context) error

	// Advance moves to the next position (phase, sequence, step).
	// Returns an error if already at the end or if the current step
	// is not complete.
	//
	// This is typically called internally by MarkComplete, but can
	// be used directly for manual navigation.
	Advance(ctx context.Context) error

	// Reset clears state and returns to the beginning.
	// All task statuses are cleared and position is reset to the first task.
	//
	// This is useful for re-running a festival from scratch.
	Reset(ctx context.Context) error

	// GetState returns the current navigation state.
	// The state includes position, task statuses, timestamps, and counters.
	//
	// Note: Returns the internal state pointer. Modifications will affect
	// the navigator's state. Use with care.
	GetState() *GuidanceState

	// GetConfig returns the guidance configuration.
	// Configuration controls navigation behavior like auto-demotion,
	// gate review, and output format.
	GetConfig() *GuidanceConfig

	// GetContext returns the guidance context.
	// Context includes festival paths, current position names, mode,
	// and other navigation metadata.
	GetContext() *GuidanceContext

	// ShouldDemote returns true if the navigator recommends switching
	// to human-guided mode for the given step.
	//
	// This is based on the step's autonomy level, the configuration's
	// auto-demotion setting, and other factors like gate review requirements.
	ShouldDemote(step *NextStep) bool
}

// Sentinel errors for common navigation failures.
var (
	// ErrMissingGuidanceContext is returned when a nil or invalid guidance context is provided.
	ErrMissingGuidanceContext = &guidanceError{msg: "guidance context is required"}

	// ErrNotInitialized is returned when navigator methods are called before Initialize().
	ErrNotInitialized = &guidanceError{msg: "navigator not initialized, call Initialize() first"}

	// ErrNoNextStep is returned when there are no more steps to execute.
	ErrNoNextStep = &guidanceError{msg: "no next step available"}

	// ErrAlreadyComplete is returned when trying to advance past the end.
	ErrAlreadyComplete = &guidanceError{msg: "all steps are complete"}

	// ErrStepNotFound is returned when a step ID is not recognized.
	ErrStepNotFound = &guidanceError{msg: "step not found"}
)

// guidanceError is the error type for navigation-specific errors.
// It's a private type to prevent external creation of sentinel errors.
type guidanceError struct {
	msg string
}

// Error implements the error interface.
func (e *guidanceError) Error() string {
	return e.msg
}

// IsNoNextStep returns true if the error indicates no more steps are available.
// This is not an error condition but rather indicates successful completion
// of all navigation work.
func IsNoNextStep(err error) bool {
	if err == nil {
		return false
	}
	var ge *guidanceError
	if errors.As(err, &ge) {
		return ge.msg == ErrNoNextStep.msg
	}
	return false
}

// IsNotInitialized returns true if the error indicates the navigator
// has not been initialized. Call Initialize() before using other methods.
func IsNotInitialized(err error) bool {
	if err == nil {
		return false
	}
	var ge *guidanceError
	if errors.As(err, &ge) {
		return ge.msg == ErrNotInitialized.msg
	}
	return false
}

// IsAlreadyComplete returns true if the error indicates all steps
// have been completed and there is no more work to do.
func IsAlreadyComplete(err error) bool {
	if err == nil {
		return false
	}
	var ge *guidanceError
	if errors.As(err, &ge) {
		return ge.msg == ErrAlreadyComplete.msg
	}
	return false
}

// IsStepNotFound returns true if the error indicates a step ID
// was not found in the navigation structure.
func IsStepNotFound(err error) bool {
	if err == nil {
		return false
	}
	var ge *guidanceError
	if errors.As(err, &ge) {
		return ge.msg == ErrStepNotFound.msg
	}
	return false
}

// BaseNavigator provides common functionality for all navigators.
// Embed this in navigator implementations to get default behavior.
//
// BaseNavigator handles:
//   - State management integration
//   - Configuration access
//   - Initialization tracking
//   - Auto-demotion logic
//
// Specific navigator implementations (ExecutionNavigator, PlanningNavigator, etc.)
// embed BaseNavigator and add their mode-specific behaviors.
type BaseNavigator struct {
	// Ctx is the guidance context with festival paths, mode, and config.
	Ctx *GuidanceContext

	// StateManager handles state persistence.
	StateManager StateManager

	// initialized tracks whether Initialize() has been called.
	initialized bool
}

// NewBaseNavigator creates a base navigator with common setup.
// All dependencies are injected via the constructor.
func NewBaseNavigator(gctx *GuidanceContext, mode Mode) (*BaseNavigator, error) {
	if gctx == nil {
		return nil, ErrMissingGuidanceContext
	}

	// Ensure config exists
	if gctx.Config == nil {
		gctx.Config = DefaultConfig()
	}

	// Set mode if not already set
	if gctx.Mode == "" {
		gctx.Mode = mode
	}

	return &BaseNavigator{
		Ctx:          gctx,
		StateManager: NewStateManager(gctx.FestivalPath, mode),
	}, nil
}

// Initialize loads state and prepares the navigator for use.
// Must be called before other methods.
func (b *BaseNavigator) Initialize(ctx context.Context) error {
	if _, err := b.StateManager.Load(ctx); err != nil {
		return err
	}
	b.initialized = true
	return nil
}

// Reset clears state and returns to the beginning.
func (b *BaseNavigator) Reset(ctx context.Context) error {
	if err := b.StateManager.Clear(ctx); err != nil {
		return err
	}
	b.initialized = false
	return nil
}

// GetState returns the current navigation state.
func (b *BaseNavigator) GetState() *GuidanceState {
	return b.StateManager.State()
}

// GetConfig returns the guidance configuration.
func (b *BaseNavigator) GetConfig() *GuidanceConfig {
	return b.Ctx.Config
}

// GetContext returns the guidance context.
func (b *BaseNavigator) GetContext() *GuidanceContext {
	return b.Ctx
}

// ShouldDemote returns true if human-guided mode is recommended for the step.
// This is based on:
//   - The step's autonomy level (low autonomy triggers demotion)
//   - Gate review configuration (gates may require human review)
//   - Whether auto-demotion is enabled in config
func (b *BaseNavigator) ShouldDemote(step *NextStep) bool {
	if step == nil {
		return false
	}

	if b.Ctx.Config == nil || !b.Ctx.Config.AutoDemote {
		return false
	}

	// Demote for low autonomy tasks
	if step.AutonomyLevel == AutonomyLow {
		return true
	}

	// Demote for gates if configured to review at gates
	if step.IsGate() && b.Ctx.Config.ReviewAtGates {
		return true
	}

	return false
}

// Save persists state changes to disk.
func (b *BaseNavigator) Save(ctx context.Context) error {
	return b.StateManager.Save(ctx)
}

// MarkTaskStatus updates a task's status and saves state.
func (b *BaseNavigator) MarkTaskStatus(ctx context.Context, taskID, status string) error {
	b.StateManager.SetTaskStatus(taskID, status)
	return b.StateManager.Save(ctx)
}

// IsInitialized returns true if Initialize() has been called.
func (b *BaseNavigator) IsInitialized() bool {
	return b.initialized
}

// EnsureInitialized returns ErrNotInitialized if Initialize() has not been called.
// Use this at the start of methods that require initialization.
func (b *BaseNavigator) EnsureInitialized() error {
	if !b.initialized {
		return ErrNotInitialized
	}
	return nil
}
