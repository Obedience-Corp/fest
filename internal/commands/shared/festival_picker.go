package shared

import (
	"context"
	"fmt"
	"os"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/tui/picker"
	"golang.org/x/term"
)

// PickFestivalPath opens the shared festival picker and returns the selected path.
func PickFestivalPath(ctx context.Context, festivalsDir string, opts FestivalPickerOptions) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stderr.Fd())) {
		return "", nil
	}

	items := FestivalPickerItems(festivalsDir, opts)
	if len(items) == 0 {
		return "", nil
	}

	selected, err := picker.Run(items, navigation.Score)
	if err != nil {
		return "", errors.Wrap(err, "running festival picker")
	}
	if selected == nil {
		return "", nil
	}

	return selected.Value, nil
}

// FestivalPickerItems returns picker items for festival candidates.
func FestivalPickerItems(festivalsDir string, opts FestivalPickerOptions) []picker.Item {
	candidates := CollectFestivalPickCandidates(festivalsDir, opts)
	if len(candidates) == 0 && len(opts.PreferredStatuses) > 0 {
		opts.PreferredStatuses = nil
		candidates = CollectFestivalPickCandidates(festivalsDir, opts)
	}

	return festivalPickerItemsFromCandidates(candidates)
}

func festivalPickerItemsFromCandidates(candidates []FestivalPickCandidate) []picker.Item {
	items := make([]picker.Item, 0, len(candidates))
	for _, candidate := range candidates {
		items = append(items, picker.Item{
			Name:  festivalPickerItemName(candidate),
			Value: candidate.Path,
		})
	}
	return items
}

func festivalPickerItemName(candidate FestivalPickCandidate) string {
	if candidate.StatusDirectory {
		return fmt.Sprintf("[%s]/", candidate.Status)
	}
	return fmt.Sprintf("[%s] %s", candidate.Status, candidate.Name)
}
