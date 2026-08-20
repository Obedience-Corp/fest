package shared

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/tui/picker"
	"github.com/Obedience-Corp/fest/internal/ui"
	"golang.org/x/term"
)

const progressDetailWorkers = 8

// WorkingFestivalPickerStatuses is the default status priority for execution-oriented
// pickers (status, progress, watch): festivals you are likely working on now.
var WorkingFestivalPickerStatuses = []string{"active", "ready", "planning"}

// BrowseFestivalPickerStatuses extends the working set for browse-oriented pickers
// (show). Ritual festivals are inspectable via fest show but intentionally omitted
// from watch/progress pickers that target active execution context.
var BrowseFestivalPickerStatuses = []string{"active", "ready", "planning", "ritual"}

// DungeonFestivalPickerStatuses is the browse fallback for a campaign whose only
// festivals are dungeoned. A campaign that just completed its single festival
// still has something worth showing, so browse-oriented surfaces broaden to the
// dungeon rather than reporting that no festival exists.
var DungeonFestivalPickerStatuses = []string{"dungeon/completed", "dungeon/archived", "dungeon/someday"}

type FestivalPickOutcome int

const (
	FestivalPicked FestivalPickOutcome = iota
	FestivalPickCancelled
	FestivalPickUnavailable
)

func PickFestival(ctx context.Context, festivalsDir string, opts FestivalPickerOptions) (string, FestivalPickOutcome, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", FestivalPickUnavailable, err
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return "", FestivalPickUnavailable, nil
	}

	items := FestivalPickerItems(festivalsDir, opts)
	if len(items) == 0 {
		return "", FestivalPickUnavailable, nil
	}
	attachProgressDetails(ctx, items)

	selected, err := picker.Run(items, navigation.Score)
	if err != nil {
		return "", FestivalPickUnavailable, errors.Wrap(err, "running festival picker")
	}
	if selected == nil {
		return "", FestivalPickCancelled, nil
	}

	return selected.Value, FestivalPicked, nil
}

func PickFestivalPath(ctx context.Context, festivalsDir string, opts FestivalPickerOptions) (string, error) {
	path, _, err := PickFestival(ctx, festivalsDir, opts)
	return path, err
}

// FestivalPickerItems returns picker items for festival candidates, including the
// FallbackStatuses broadening applied by CollectFestivalPickCandidates.
func FestivalPickerItems(festivalsDir string, opts FestivalPickerOptions) []picker.Item {
	return festivalPickerItemsFromCandidates(CollectFestivalPickCandidates(festivalsDir, opts))
}

func festivalPickerItemsFromCandidates(candidates []FestivalPickCandidate) []picker.Item {
	items := make([]picker.Item, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, picker.Item{
			Prefix:      festivalStatusLabel(candidate),
			PrefixColor: ui.GetInteractiveStatusColor(candidate.Status),
			Name:        festivalPickerItemName(candidate),
			Value:       candidate.Path,
		})
	}
	return items
}

// festivalStatusLabel is the status prefix shown (and colorized) before the
// festival name, e.g. "[ready]" or "[active]/" for a status directory.
func festivalStatusLabel(candidate FestivalPickCandidate) string {
	if candidate.StatusDirectory {
		return fmt.Sprintf("[%s]/", candidate.Status)
	}
	return fmt.Sprintf("[%s]", candidate.Status)
}

// festivalPickerItemName is the matchable display name: the festival name, or
// empty for a status directory whose label lives entirely in the prefix.
func festivalPickerItemName(candidate FestivalPickCandidate) string {
	if candidate.StatusDirectory {
		return ""
	}
	return candidate.Name
}

// attachProgressDetails fills each item's Detail with a progress bar, computed
// read-only and concurrently. Items without tasks (status directories, empty
// festivals) get no bar.
func attachProgressDetails(ctx context.Context, items []picker.Item) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, progressDetailWorkers)
	for i := range items {
		if items[i].Value == "" {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			items[i].Detail = festivalProgressDetail(ctx, items[i].Value)
		}(i)
	}
	wg.Wait()
}

func festivalProgressDetail(ctx context.Context, festivalPath string) string {
	mgr, err := progress.NewManagerReadOnly(ctx, festivalPath)
	if err != nil {
		return ""
	}
	prog, err := mgr.GetFestivalProgress(ctx, festivalPath)
	if err != nil || prog == nil || prog.Overall == nil || prog.Overall.Total == 0 {
		return ""
	}
	return ui.RenderProgressBar(ui.ProgressBarOptions{
		Current:        prog.Overall.Percentage,
		Total:          100,
		Width:          10,
		FilledChar:     "█",
		EmptyChar:      "░",
		FilledColor:    ui.SuccessColor,
		EmptyColor:     ui.BorderColor,
		ShowPercentage: true,
	})
}
