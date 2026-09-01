package status

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/ui/theme"
	"github.com/Obedience-Corp/fest/internal/workspace"
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

	_, resolveErr := shared.ResolveFestivalPath(cwd, opts.path)
	var detectPath string
	if resolveErr != nil {
		if opts.json {
			return emitErrorJSON("not in a festival directory")
		}
		picked, pickErr := pickFestivalForDisplay(ctx, cwd)
		if pickErr != nil {
			return pickErr
		}
		if picked == "" {
			return nil
		}
		detectPath = picked
	} else {
		detectPath = cwd
	}

	loc, err := show.DetectCurrentLocation(ctx, detectPath)
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

func pickFestivalForDisplay(ctx context.Context, cwd string) (string, error) {
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return "", errors.NotFound("festival").
			WithHint("navigate to a festival directory or a camp")
	}
	return shared.PickFestivalPath(ctx, festivalsDir, shared.FestivalPickerOptions{
		IncludeStatusDirectories: false,
		PreferredStatuses:        shared.WorkingFestivalPickerStatuses,
		FallbackStatuses:         shared.WorkingFestivalPickerStatuses,
		OrderByStatusThenRecency: true,
	})
}

// statusHandler is the signature for status set handler functions.
type statusHandler func(ctx context.Context, display *ui.UI, cwd, newStatus string, opts *statusOptions) error

// resolveExplicitHandler checks if any level-specific flag (--task, --sequence,
// --phase, --path) was provided and returns the corresponding handler function.
// Returns nil if no explicit flag was set.
// Flag priority: --task > --sequence > --phase > --path
func resolveExplicitHandler(opts *statusOptions) statusHandler {
	if opts.task != "" {
		return handleTaskStatusSet
	}
	if opts.sequence != "" {
		return handleSequenceStatusSet
	}
	if opts.phase != "" {
		return handlePhaseStatusSet
	}
	if opts.path != "" {
		return handlePathBasedStatusSet
	}
	return nil
}

// promptForStatus presents an interactive status selection prompt when no status
// argument was provided. It detects the entity type from context and presents
// valid status options for that entity type.
func promptForStatus(ctx context.Context, cwd string, opts *statusOptions) (string, bool, error) {
	entityType := detectEntityTypeForStatusPrompt(cwd, opts)
	var validStatuses []string
	if entityType == EntityFestival {
		validStatuses = getValidFestivalStatuses()
	} else {
		validStatuses = ValidStatuses[entityType]
	}
	options := theme.ToOptions(validStatuses)

	var selected string
	title := fmt.Sprintf("Select %s status", entityType)
	cancelled, err := theme.QuickSelect(ctx, title, options, &selected)
	if err != nil {
		return "", false, errors.Wrap(err, "status selection failed")
	}
	if cancelled || selected == "" {
		return "", true, nil
	}
	return selected, false, nil
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

	// Resolve status interactively if not provided
	if newStatus == "" {
		var cancelled bool
		newStatus, cancelled, err = promptForStatus(ctx, cwd, opts)
		if err != nil {
			return err
		}
		if cancelled {
			display.Info("Selection cancelled.")
			return nil
		}
	}

	// Route to explicit flag handlers first
	if handler := resolveExplicitHandler(opts); handler != nil {
		return handler(ctx, display, cwd, newStatus, opts)
	}

	// Fall back to context-aware routing
	return routeByContext(ctx, display, cwd, newStatus, opts)
}

// routeByContext handles status set when no explicit flag was provided.
// It resolves the festival path, detects the current location context,
// and routes to the appropriate handler.
func routeByContext(ctx context.Context, display *ui.UI, cwd, newStatus string, opts *statusOptions) error {
	festivalPath, err := shared.ResolveFestivalPath(cwd, "")

	if err != nil || opts.interactive {
		return handleInteractiveFallback(ctx, display, cwd, newStatus, opts)
	}

	loc, err := detectStatusLocation(ctx, cwd, festivalPath)
	if err != nil {
		return err
	}

	if loc.Festival == nil {
		return errors.NotFound("festival")
	}

	return dispatchByLocationType(ctx, display, cwd, newStatus, opts, loc)
}

// handleInteractiveFallback handles status set when not inside a festival
// or when --interactive was explicitly requested.
func handleInteractiveFallback(ctx context.Context, display *ui.UI, cwd, newStatus string, opts *statusOptions) error {
	selectedFestival, selectErr := selectFestivalForStatus(ctx, cwd, newStatus)
	if selectErr != nil {
		return selectErr
	}
	if selectedFestival == nil {
		display.Info("Selection cancelled.")
		return nil
	}
	return applyStatusToFestival(ctx, display, selectedFestival, newStatus, opts)
}

// detectStatusLocation detects the current location within a festival.
// Tries cwd first, then falls back to festivalPath for linked projects.
func detectStatusLocation(ctx context.Context, cwd, festivalPath string) (*show.LocationInfo, error) {
	loc, err := show.DetectCurrentLocation(ctx, cwd)
	if err != nil || loc.Festival == nil {
		loc, err = show.DetectCurrentLocation(ctx, festivalPath)
		if err != nil {
			return nil, err
		}
	}
	return loc, nil
}

// dispatchByLocationType routes status set based on the detected location type.
func dispatchByLocationType(ctx context.Context, display *ui.UI, cwd, newStatus string, opts *statusOptions, loc *show.LocationInfo) error {
	switch loc.Type {
	case "task":
		return showContextHint(display, opts, loc, newStatus, "task")

	case "sequence":
		if !isValidStatus(EntitySequence, newStatus) {
			validOptions := ValidStatuses[EntitySequence]
			return errors.Validation("invalid status for sequence").
				WithField("status", newStatus).
				WithField("valid_options", strings.Join(validOptions, ", "))
		}
		opts.sequence = loc.Sequence
		return handleSequenceStatusSet(ctx, display, cwd, newStatus, opts)

	case "phase":
		if !isValidStatus(EntityPhase, newStatus) {
			validOptions := ValidStatuses[EntityPhase]
			return errors.Validation("invalid status for phase").
				WithField("status", newStatus).
				WithField("valid_options", strings.Join(validOptions, ", "))
		}
		opts.phase = loc.Phase
		return handlePhaseStatusSet(ctx, display, cwd, newStatus, opts)

	case "festival":
		if !isValidFestivalStatus(newStatus) {
			validOptions := getValidFestivalStatuses()
			return errors.Validation("invalid status for festival").
				WithField("status", newStatus).
				WithField("valid_options", strings.Join(validOptions, ", "))
		}
		return handleFestivalStatusChange(ctx, display, loc.Festival, newStatus, opts)

	default:
		return errors.Validation("unknown context").
			WithField("type", loc.Type).
			WithHint("use --phase, --sequence, or --task to specify level")
	}
}

// showContextHint shows a hint when in task context but no flag provided.
func showContextHint(display *ui.UI, opts *statusOptions, loc *show.LocationInfo, newStatus, contextType string) error {
	if opts.json {
		result := map[string]any{
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

// runStatusHistory handles the status history command.
func runStatusHistory(ctx context.Context, cmd *cobra.Command, limit int, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	// Resolve festival path (supports linked festivals via fest link)
	festivalPath, err := shared.ResolveFestivalPath(cwd, "")
	if err != nil {
		return errors.Wrap(err, "not inside a festival")
	}

	loc, err := show.DetectCurrentLocation(ctx, festivalPath)
	if err != nil {
		return err
	}

	if loc.Festival == nil {
		return errors.NotFound("festival")
	}

	// Load and emit history
	history, err := loadStatusHistory(ctx, loc.Festival.Path)
	if err != nil {
		return err
	}

	return emitStatusHistory(opts, loc.Festival.Name, history, limit)
}

// loadStatusHistory loads the status history from a festival's history file.
func loadStatusHistory(ctx context.Context, festivalPath string) ([]map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, errors.Wrap(err, "context cancelled")
	}

	historyPath := filepath.Join(festivalPath, ".fest", "status_history.json")

	// Check if history exists
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		return nil, nil // No history file - not an error
	}

	// Read history
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return nil, errors.IO("reading history file", err)
	}

	var history []map[string]any
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, errors.Wrap(err, "parsing history file")
	}

	return history, nil
}

// emitStatusHistory outputs the status history in the appropriate format.
func emitStatusHistory(opts *statusOptions, festivalName string, history []map[string]any, limit int) error {
	// Handle no history case
	if history == nil {
		if opts.json {
			result := map[string]any{
				"history": []any{},
				"message": "no status history found",
			}
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			fmt.Println("No status history found for this festival.")
		}
		return nil
	}

	// Apply limit
	if limit > 0 && len(history) > limit {
		history = history[len(history)-limit:]
	}

	if opts.json {
		result := map[string]any{
			"festival": festivalName,
			"count":    len(history),
			"history":  history,
		}
		if err := shared.EncodeJSON(os.Stdout, result); err != nil {
			return errors.Wrap(err, "encoding JSON output")
		}
	} else {
		fmt.Printf("Status History for %s:\n", festivalName)
		fmt.Println(strings.Repeat("-", 50))
		for _, entry := range history {
			fmt.Printf("%s: %s -> %s\n",
				entry["timestamp"],
				entry["from_status"],
				entry["to_status"])
			if note, ok := entry["note"].(string); ok && note != "" {
				fmt.Printf("  Note: %s\n", note)
			}
		}
	}

	return nil
}
