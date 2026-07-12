package list

import (
	"context"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/progress"
)

// multiStatusBoard is the shared collection model for dungeon and all-status
// boards. One-shot list, JSON, and --watch all consume this so sort/filter/
// progress behavior cannot drift between modes.
type multiStatusBoard struct {
	Festivals map[string][]*show.FestivalInfo
	Order     []string
	Total     int
	Progress  map[string]*progress.FestivalProgress
}

// statusBoard is the single-status collection model.
type statusBoard struct {
	Festivals []*show.FestivalInfo
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
		return formatStatusHuman(filterStatus, board.Festivals, board.Progress, opts.progress), nil
	}
	board, err := collectAllBoard(ctx, festivalsDir, opts, campaignRoot)
	if err != nil {
		return "", err
	}
	return formatAllHuman(board.Festivals, board.Order, board.Progress, board.Total, opts.progress), nil
}

func collectDungeonBoard(ctx context.Context, festivalsDir string, opts *listOptions, campaignRoot string) (multiStatusBoard, error) {
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
	return statusBoard{Festivals: festivals, Progress: progressMap}, nil
}

func collectAllBoard(ctx context.Context, festivalsDir string, opts *listOptions, campaignRoot string) (multiStatusBoard, error) {
	var totalCount int
	allFestivals := make(map[string][]*show.FestivalInfo)
	statuses := defaultStatuses
	if opts.all {
		statuses = validStatuses
	}
	statusOrder := make([]string, 0, len(statuses))
	var allFestivalsList []*show.FestivalInfo

	for _, status := range statuses {
		festivals, err := show.ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
		if err != nil {
			continue
		}
		festivals, err = applyFilters(festivals, opts)
		if err != nil {
			return multiStatusBoard{}, err
		}
		if len(festivals) > 0 {
			applySorting(festivals, opts.sortBy, opts.alpha)
			allFestivals[status] = festivals
			statusOrder = append(statusOrder, status)
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
		Order:     statusOrder,
		Total:     totalCount,
		Progress:  progressMap,
	}, nil
}
