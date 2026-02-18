package status

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/spf13/cobra"
)

// runStatusShow handles the status show command.
func runStatusShow(ctx context.Context, cmd *cobra.Command, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	// Resolve festival path (supports linked festivals via fest link)
	_, err = shared.ResolveFestivalPath(cwd, opts.path)
	if err != nil {
		if opts.json {
			return emitErrorJSON("not in a festival directory")
		}
		return errors.Wrap(err, "not inside a festival")
	}

	// Detect current location using cwd for accurate location detection
	loc, err := show.DetectCurrentLocation(ctx, cwd)
	if err != nil {
		if opts.json {
			return emitErrorJSON("not in a festival directory")
		}
		return err
	}

	if opts.json {
		return emitLocationJSON(loc)
	}
	return emitLocationText(ctx, loc)
}

// runStatusSet handles the status set command.
func runStatusSet(ctx context.Context, cmd *cobra.Command, newStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	display := ui.New(shared.IsNoColor(), shared.IsVerbose())

	// If no status provided, prompt user to select one
	if newStatus == "" {
		entityType := detectEntityTypeForStatusPrompt(cwd, opts)
		var validStatuses []string
		if entityType == EntityFestival {
			festivalsRoot := findFestivalsRoot(cwd)
			validStatuses = getValidFestivalStatuses(festivalsRoot)
		} else {
			validStatuses = ValidStatuses[entityType]
		}
		options := theme.ToOptions(validStatuses)

		title := fmt.Sprintf("Select %s status", entityType)
		cancelled, err := theme.QuickSelect(ctx, title, options, &newStatus)
		if err != nil {
			return errors.Wrap(err, "status selection failed")
		}
		if cancelled || newStatus == "" {
			display.Info("Selection cancelled.")
			return nil
		}
	}

	// Check if a level-specific flag was provided
	if opts.task != "" {
		return handleTaskStatusSet(ctx, display, cwd, newStatus, opts)
	}
	if opts.sequence != "" {
		return handleSequenceStatusSet(ctx, display, cwd, newStatus, opts)
	}
	if opts.phase != "" {
		return handlePhaseStatusSet(ctx, display, cwd, newStatus, opts)
	}

	// Handle --path flag: detect entity type and route accordingly
	if opts.path != "" {
		return handlePathBasedStatusSet(ctx, display, cwd, newStatus, opts)
	}

	// No level flag - use original logic (festival level or context-aware)
	// Resolve festival path (supports linked festivals via fest link)
	festivalPath, err := shared.ResolveFestivalPath(cwd, "")

	// Handle case when not inside a festival or interactive mode requested
	if err != nil || opts.interactive {
		// Interactive selection mode
		selectedFestival, selectErr := selectFestivalForStatus(ctx, cwd, newStatus)
		if selectErr != nil {
			return selectErr
		}
		if selectedFestival == nil {
			// User cancelled
			display.Info("Selection cancelled.")
			return nil
		}

		// Use selected festival
		return applyStatusToFestival(ctx, display, selectedFestival, newStatus, opts)
	}

	// Detect current location
	// Try cwd first (when inside festival), then fall back to festivalPath (when linked)
	var loc *show.LocationInfo
	loc, err = show.DetectCurrentLocation(ctx, cwd)
	if err != nil || loc.Festival == nil {
		// We might be in a linked project directory
		// Fall back to festival root detection
		loc, err = show.DetectCurrentLocation(ctx, festivalPath)
		if err != nil {
			return err
		}
	}

	if loc.Festival == nil {
		return errors.NotFound("festival")
	}

	// Context-aware routing based on detected location
	switch loc.Type {
	case "task":
		// In a task context - require explicit --task flag
		// Task status is too granular for auto-detect
		return showContextHint(display, opts, loc, newStatus, "task")

	case "sequence":
		// Auto-detect sequence status
		if !isValidStatus(EntitySequence, newStatus) {
			validOptions := ValidStatuses[EntitySequence]
			return errors.Validation("invalid status for sequence").
				WithField("status", newStatus).
				WithField("valid_options", strings.Join(validOptions, ", "))
		}
		// Route to sequence handler
		opts.sequence = loc.Sequence
		return handleSequenceStatusSet(ctx, display, cwd, newStatus, opts)

	case "phase":
		// Auto-detect phase status
		if !isValidStatus(EntityPhase, newStatus) {
			validOptions := ValidStatuses[EntityPhase]
			return errors.Validation("invalid status for phase").
				WithField("status", newStatus).
				WithField("valid_options", strings.Join(validOptions, ", "))
		}
		// Route to phase handler
		opts.phase = loc.Phase
		return handlePhaseStatusSet(ctx, display, cwd, newStatus, opts)

	case "festival":
		// At festival root - validate festival status (schema-aware)
		festivalsRoot := festivalsRootFromPath(loc.Festival.Path, loc.Festival.Status)
		if !isValidFestivalStatus(festivalsRoot, newStatus) {
			validOptions := getValidFestivalStatuses(festivalsRoot)
			return errors.Validation("invalid status for festival").
				WithField("status", newStatus).
				WithField("valid_options", strings.Join(validOptions, ", "))
		}
		return handleFestivalStatusChange(ctx, display, loc.Festival, newStatus, opts)

	default:
		// Unknown context - show help
		return errors.Validation("unknown context").
			WithField("type", loc.Type).
			WithField("hint", "use --phase, --sequence, or --task to specify level")
	}
}

// showContextHint shows a hint when in task context but no flag provided.
func showContextHint(display *ui.UI, opts *statusOptions, loc *show.LocationInfo, newStatus, contextType string) error {
	if opts.json {
		result := map[string]interface{}{
			"success":      false,
			"context_type": contextType,
			"hint":         "use --task flag to set task status explicitly",
			"current_task": loc.Task,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
		return nil
	}

	fmt.Println(ui.Warning("Context Detection"))
	fmt.Printf("%s %s\n", ui.Label("Current location"), ui.Dim(contextType))
	if loc.Task != "" {
		fmt.Printf("%s %s\n", ui.Label("Task"), ui.Dim(loc.Task))
	}
	fmt.Println()
	fmt.Println(ui.Info("Task status requires explicit targeting:"))
	fmt.Printf("  fest status set --task %s %s\n", loc.Task, newStatus)
	fmt.Println()
	fmt.Println(ui.Dim("Or to set a higher level:"))
	fmt.Printf("  fest status set --sequence %s %s  # sequence status\n", loc.Sequence, newStatus)
	fmt.Printf("  fest status set --phase %s %s       # phase status\n", loc.Phase, newStatus)

	return nil
}
