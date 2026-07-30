package list

import (
	"context"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// multiStatusBoard is the shared collection model for dungeon and all-status
// boards. One-shot list, JSON, and --watch all consume this so sort/filter/
// progress behavior cannot drift between modes.
type multiStatusBoard struct {
	Festivals map[string][]*show.FestivalInfo
	Order     []string
	Total     int
	Progress  map[string]*progress.FestivalProgress
	Residents map[string][]*show.ResidentCard
}

// statusBoard is the single-status collection model.
type statusBoard struct {
	Festivals []*show.FestivalInfo
	Residents []*show.ResidentCard
	Progress  map[string]*progress.FestivalProgress
}

// formatListBoard builds the human-readable list board for the current filters.
// Used by one-shot list (via list*) and by --watch refresh frames.
func formatListBoard(ctx context.Context, festivalsDir, filterStatus string, opts *listOptions, campaignRoot string) (string, error) {
	if filterStatus != "" {
		if filterStatus == "dungeon" {
			board, err := collectDungeonBoard(ctx, festivalsDir, opts, campaignRoot)
			if err != nil {
				return "", err
			}
			return formatDungeonHuman(board.Festivals, board.Order, board.Progress, board.Total, opts.progress), nil
		}
		board, err := collectStatusBoard(ctx, festivalsDir, filterStatus, opts, campaignRoot)
		if err != nil {
			return "", err
		}
		return formatStatusHuman(filterStatus, board.Festivals, board.Residents, board.Progress, opts.progress), nil
	}
	board, err := collectAllBoard(ctx, festivalsDir, opts, campaignRoot)
	if err != nil {
		return "", err
	}
	return formatAllHuman(board.Festivals, board.Residents, board.Order, board.Progress, board.Total, opts.progress), nil
}

func collectDungeonBoard(ctx context.Context, festivalsDir string, opts *listOptions, campaignRoot string) (multiStatusBoard, error) {
	if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
		return multiStatusBoard{}, err
	}

	var totalCount int
	allFestivals := make(map[string][]*show.FestivalInfo)
	order := make([]string, 0, len(dungeonSubstatuses))
	var allFestivalsList []*show.FestivalInfo

	for _, status := range dungeonSubstatuses {
		festivals, err := show.ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
		if err != nil {
			continue
		}
		festivals, err = applyFilters(festivals, opts)
		if err != nil {
			return multiStatusBoard{}, err
		}
		if len(festivals) > 0 {
			// Default dungeon listings order by bucket date (newest first);
			// explicit --sort or --alpha still wins.
			if opts.sortBy == "" && !opts.alpha {
				sortByStatusDate(festivals)
			} else {
				applySorting(festivals, opts.sortBy, opts.alpha)
			}
			allFestivals[status] = festivals
			order = append(order, status)
			totalCount += len(festivals)
			allFestivalsList = append(allFestivalsList, festivals...)
		}
	}

	var progressMap map[string]*progress.FestivalProgress
	if opts.progress {
		progressMap = fetchProgressForFestivals(ctx, allFestivalsList)
	}
	return multiStatusBoard{
		Festivals: allFestivals,
		Order:     order,
		Total:     totalCount,
		Progress:  progressMap,
	}, nil
}

func collectStatusBoard(ctx context.Context, festivalsDir, status string, opts *listOptions, campaignRoot string) (statusBoard, error) {
	if strings.HasPrefix(status, "dungeon/") {
		if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
			return statusBoard{}, err
		}
	}

	festivals, err := show.ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
	if err != nil {
		return statusBoard{}, err
	}
	festivals, err = applyFilters(festivals, opts)
	if err != nil {
		return statusBoard{}, err
	}
	// Dungeon listings default to newest bucket date first; explicit sort
	// flags still take precedence.
	if opts.sortBy == "" && !opts.alpha && strings.HasPrefix(status, "dungeon/") {
		sortByStatusDate(festivals)
	} else {
		applySorting(festivals, opts.sortBy, opts.alpha)
	}
	var progressMap map[string]*progress.FestivalProgress
	if opts.progress {
		progressMap = fetchProgressForFestivals(ctx, festivals)
	}
	return statusBoard{
		Festivals: festivals,
		Residents: show.ListResidentsByStatus(ctx, festivalsDir, status),
		Progress:  progressMap,
	}, nil
}

func collectAllBoard(ctx context.Context, festivalsDir string, opts *listOptions, campaignRoot string) (multiStatusBoard, error) {
	statuses := defaultStatuses
	if opts.all {
		if err := workspace.CheckDungeonConflict(festivalsDir); err != nil {
			return multiStatusBoard{}, err
		}
		statuses = validStatuses
	}

	var totalCount int
	allFestivals := make(map[string][]*show.FestivalInfo)
	allResidents := make(map[string][]*show.ResidentCard)
	statusOrder := make([]string, 0, len(statuses))
	var allFestivalsList []*show.FestivalInfo

	for _, status := range statuses {
		if residents := show.ListResidentsByStatus(ctx, festivalsDir, status); len(residents) > 0 {
			allResidents[status] = residents
		}
		festivals, err := collectStatusFestivals(ctx, festivalsDir, status, opts, campaignRoot)
		if err != nil {
			return multiStatusBoard{}, err
		}
		if len(festivals) > 0 {
			allFestivals[status] = festivals
			totalCount += len(festivals)
			allFestivalsList = append(allFestivalsList, festivals...)
		}
		// A stage holding only residents still deserves a column.
		if len(festivals) > 0 || len(allResidents[status]) > 0 {
			statusOrder = append(statusOrder, status)
		}
	}

	var progressMap map[string]*progress.FestivalProgress
	if opts.progress {
		progressMap = fetchProgressForFestivals(ctx, allFestivalsList)
	}
	return multiStatusBoard{
		Festivals: allFestivals,
		Residents: allResidents,
		Order:     statusOrder,
		Total:     totalCount,
		Progress:  progressMap,
	}, nil
}

// collectStatusFestivals lists, filters, and sorts one status's festivals. An
// unreadable status directory yields none rather than failing the whole board,
// matching the previous inline behavior.
func collectStatusFestivals(ctx context.Context, festivalsDir, status string, opts *listOptions, campaignRoot string) ([]*show.FestivalInfo, error) {
	festivals, err := show.ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
	if err != nil {
		return nil, nil
	}
	festivals, err = applyFilters(festivals, opts)
	if err != nil {
		return nil, err
	}
	applySorting(festivals, opts.sortBy, opts.alpha)
	return festivals, nil
}
