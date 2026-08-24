package status

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

// runFestivalListing handles listing festivals.
func runFestivalListing(ctx context.Context, festivalsRoot, filterStatus string, opts *statusOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	if filterStatus != "" {
		if filterStatus == "dungeon" || strings.HasPrefix(filterStatus, "dungeon/") {
			if err := workspace.CheckDungeonConflict(festivalsRoot); err != nil {
				return err
			}
		}

		festivals, err := show.ListFestivalsByStatus(ctx, festivalsRoot, filterStatus, campaignRoot)
		if err != nil {
			return err
		}

		residents := show.ListResidentsByStatus(ctx, festivalsRoot, filterStatus)

		if opts.json {
			result := map[string]any{
				"status":    filterStatus,
				"count":     len(festivals),
				"festivals": festivals,
			}
			// Additive: absent when the stage holds no residents.
			if len(residents) > 0 {
				result["residents"] = residents
			}
			if err := shared.EncodeJSON(os.Stdout, result); err != nil {
				return errors.Wrap(err, "encoding JSON output")
			}
		} else {
			fmt.Println(show.FormatFestivalList(filterStatus, festivals, nil))
			if block := show.FormatResidentList(filterStatus, residents); block != "" {
				fmt.Println(block)
			}
		}
	} else {
		fmt.Println("Use 'fest list --all' to see all festivals grouped by status")
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
			WithHint("navigate to a festival directory to list phases/sequences/tasks")
	}
	if opts.entityType == "festival" || opts.entityType == "" {
		return runFestivalListing(ctx, festivalsDir, filterStatus, opts)
	}
	return errors.NotFound("festival").
		WithHint("navigate to a festival directory to list phases/sequences/tasks")
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
