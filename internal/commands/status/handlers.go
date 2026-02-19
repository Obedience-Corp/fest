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
			WithField("hint", "use --phase, --sequence, or --task to specify level")
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

// runFestivalListing handles listing festivals.
func runFestivalListing(ctx context.Context, festivalsRoot, filterStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if filterStatus != "" {
		festivals, err := show.ListFestivalsByStatus(ctx, festivalsRoot, filterStatus)
		if err != nil {
			return err
		}

		if opts.json {
			result := map[string]any{
				"status":    filterStatus,
				"count":     len(festivals),
				"festivals": festivals,
			}
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			fmt.Println(show.FormatFestivalList(filterStatus, festivals))
		}
	} else {
		fmt.Println("Use 'fest show all' to see all festivals grouped by status")
	}

	return nil
}

// runPhaseListing handles listing phases in a festival.
func runPhaseListing(ctx context.Context, loc *show.LocationInfo, filterStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if loc.Festival == nil {
		return errors.NotFound("festival")
	}

	phases, err := collectPhasesForListing(ctx, loc)
	if err != nil {
		return err
	}

	// Filter by status
	phases = filterPhasesByStatus(phases, filterStatus)

	// Handle empty results
	if len(phases) == 0 {
		return emitEmptyPhasesResult(opts, filterStatus)
	}

	// Emit output
	if opts.json {
		return emitPhasesJSON(phases, filterStatus)
	}
	return emitPhasesText(phases, filterStatus)
}

// collectPhasesForListing collects phases based on the current location context.
func collectPhasesForListing(ctx context.Context, loc *show.LocationInfo) ([]*PhaseInfo, error) {
	if loc.Type == "festival" {
		// List all phases in festival
		return collectPhases(ctx, loc.Festival.Path)
	}

	// In phase/sequence/task - list just current phase
	phasePath := filepath.Join(loc.Festival.Path, loc.Phase)
	phase, err := collectPhaseInfo(ctx, phasePath, loc.Phase)
	if err != nil {
		return nil, err
	}
	return []*PhaseInfo{phase}, nil
}

// emitEmptyPhasesResult outputs a message when no phases are found.
func emitEmptyPhasesResult(opts *statusOptions, filterStatus string) error {
	if opts.json {
		return emitEmptyJSON("phase", filterStatus)
	}
	message := "No phases found"
	if filterStatus != "" {
		message = fmt.Sprintf("No phases found with status '%s'", filterStatus)
	}
	fmt.Println(ui.Info(message))
	return nil
}

// runSequenceListing handles listing sequences in a festival or phase.
func runSequenceListing(ctx context.Context, loc *show.LocationInfo, filterStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if loc.Festival == nil {
		return errors.NotFound("festival")
	}

	sequences, err := collectSequencesForListing(ctx, loc)
	if err != nil {
		return err
	}

	// Filter by status
	sequences = filterSequencesByStatus(sequences, filterStatus)

	// Handle empty results
	if len(sequences) == 0 {
		return emitEmptySequencesResult(opts, filterStatus)
	}

	// Emit output
	if opts.json {
		return emitSequencesJSON(sequences, filterStatus)
	}
	return emitSequencesText(sequences, filterStatus)
}

// collectSequencesForListing collects sequences based on the current location context.
func collectSequencesForListing(ctx context.Context, loc *show.LocationInfo) ([]*SequenceInfo, error) {
	store := progressStoreForFestival(ctx, loc.Festival.Path)
	switch loc.Type {
	case "festival":
		// List all sequences in festival
		return collectSequencesFromFestival(ctx, loc.Festival.Path)
	case "phase":
		// List sequences in current phase
		phasePath := filepath.Join(loc.Festival.Path, loc.Phase)
		return collectSequences(ctx, phasePath, loc.Phase, store, loc.Festival.Path)
	default:
		// In sequence or task - list current sequence
		phasePath := filepath.Join(loc.Festival.Path, loc.Phase)
		seqPath := filepath.Join(phasePath, loc.Sequence)
		seq, err := collectSequenceInfo(ctx, seqPath, loc.Phase, loc.Sequence, store, loc.Festival.Path)
		if err != nil {
			return nil, err
		}
		return []*SequenceInfo{seq}, nil
	}
}

// emitEmptySequencesResult outputs a message when no sequences are found.
func emitEmptySequencesResult(opts *statusOptions, filterStatus string) error {
	if opts.json {
		return emitEmptyJSON("sequence", filterStatus)
	}
	message := "No sequences found"
	if filterStatus != "" {
		message = fmt.Sprintf("No sequences found with status '%s'", filterStatus)
	}
	fmt.Println(ui.Info(message))
	return nil
}

// runTaskListing handles listing tasks in a festival, phase, or sequence.
func runTaskListing(ctx context.Context, loc *show.LocationInfo, filterStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	if loc.Festival == nil {
		return errors.NotFound("festival")
	}

	tasks, err := collectTasksForListing(ctx, loc)
	if err != nil {
		return err
	}

	// Filter by status
	tasks = filterTasksByStatus(tasks, filterStatus)

	// Handle empty results
	if len(tasks) == 0 {
		return emitEmptyTasksResult(opts, filterStatus)
	}

	// Emit output
	if opts.json {
		return emitTasksJSON(tasks, filterStatus)
	}
	return emitTasksText(tasks, filterStatus)
}

// collectTasksForListing collects tasks based on the current location context.
func collectTasksForListing(ctx context.Context, loc *show.LocationInfo) ([]*TaskInfo, error) {
	store := progressStoreForFestival(ctx, loc.Festival.Path)
	switch loc.Type {
	case "festival":
		// List all tasks in festival
		return collectTasksFromFestival(ctx, loc.Festival.Path)
	case "phase":
		// List tasks in current phase
		phasePath := filepath.Join(loc.Festival.Path, loc.Phase)
		return collectTasksFromPhase(ctx, phasePath, loc.Phase, store, loc.Festival.Path)
	default:
		// In sequence or task - list tasks in current sequence
		phasePath := filepath.Join(loc.Festival.Path, loc.Phase)
		seqPath := filepath.Join(phasePath, loc.Sequence)
		return collectTasks(ctx, seqPath, loc.Phase, loc.Sequence, store, loc.Festival.Path)
	}
}

// emitEmptyTasksResult outputs a message when no tasks are found.
func emitEmptyTasksResult(opts *statusOptions, filterStatus string) error {
	if opts.json {
		return emitEmptyJSON("task", filterStatus)
	}
	message := "No tasks found"
	if filterStatus != "" {
		message = fmt.Sprintf("No tasks found with status '%s'", filterStatus)
	}
	fmt.Println(ui.Info(message))
	return nil
}

// runStatusList is the main handler for the status list command.
func runStatusList(ctx context.Context, cmd *cobra.Command, filterStatus string, opts *statusOptions) error {
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
		return handleStatusListOutsideFestival(ctx, cwd, filterStatus, opts)
	}

	// Detect current location
	loc, err := show.DetectCurrentLocation(ctx, festivalPath)
	if err != nil {
		return handleStatusListOutsideFestival(ctx, cwd, filterStatus, opts)
	}

	// Validate entity type and status
	entityType := EntityType(opts.entityType)
	if filterStatus != "" {
		var statusValid bool
		var validOptions []string
		if entityType == EntityFestival || entityType == "" {
			statusValid = isValidFestivalStatus(filterStatus)
			validOptions = getValidFestivalStatuses()
		} else {
			statusValid = isValidStatus(entityType, filterStatus)
			validOptions = ValidStatuses[entityType]
		}
		if !statusValid {
			return errors.Validation("invalid status for entity type").
				WithField("status", filterStatus).
				WithField("type", opts.entityType).
				WithField("valid_options", strings.Join(validOptions, ", "))
		}
	}

	// Route based on entity type
	return routeStatusListByType(ctx, loc, filterStatus, opts)
}

// handleStatusListOutsideFestival handles status list when not in a festival directory.
func handleStatusListOutsideFestival(ctx context.Context, cwd, filterStatus string, opts *statusOptions) error {
	festivalsDir := findFestivalsRoot(cwd)
	if festivalsDir == "" {
		return errors.NotFound("festival or festivals directory").
			WithField("hint", "navigate to a festival directory to list phases/sequences/tasks")
	}
	if opts.entityType == "festival" || opts.entityType == "" {
		return runFestivalListing(ctx, festivalsDir, filterStatus, opts)
	}
	return errors.NotFound("festival").
		WithField("hint", "navigate to a festival directory to list phases/sequences/tasks")
}

// routeStatusListByType routes the status list command based on entity type.
func routeStatusListByType(ctx context.Context, loc *show.LocationInfo, filterStatus string, opts *statusOptions) error {
	switch opts.entityType {
	case "festival", "":
		festivalsRoot := festivalsRootFromPath(loc.Festival.Path, loc.Festival.Status)
		return runFestivalListing(ctx, festivalsRoot, filterStatus, opts)
	case "phase":
		return runPhaseListing(ctx, loc, filterStatus, opts)
	case "sequence":
		return runSequenceListing(ctx, loc, filterStatus, opts)
	case "task":
		return runTaskListing(ctx, loc, filterStatus, opts)
	default:
		return errors.Validation("invalid entity type").
			WithField("type", opts.entityType).
			WithField("valid_types", "festival, phase, sequence, task")
	}
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
