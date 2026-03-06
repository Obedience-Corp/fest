package explore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/festival"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/watch"
	"github.com/Obedience-Corp/fest/internal/workspace"
	tea "github.com/charmbracelet/bubbletea"
)

// buildStatusItems creates status overview items from constants (zero I/O).
func buildStatusItems() []FestivalItem {
	items := make([]FestivalItem, 0, len(id.StatusDirectories))
	for _, s := range id.StatusDirectories {
		items = append(items, FestivalItem{
			Name:   statusDisplayName(s),
			Status: s,
			Path:   s,
			Type:   ItemStatus,
			Count:  -1, // loading
		})
	}
	return items
}

// sortFestivalsByCreated sorts festival items by CreatedAt descending (newest first).
// Items without dates sort last; ties fall back to alphabetical name.
func sortFestivalsByCreated(items []FestivalItem) {
	sort.SliceStable(items, func(i, j int) bool {
		ti := items[i].CreatedAt
		tj := items[j].CreatedAt
		if ti.IsZero() && tj.IsZero() {
			return items[i].Name < items[j].Name
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.After(tj)
	})
}

// loadStatusCounts does lightweight directory counting (ReadDir only, no stat validation).
func loadStatusCounts(ctx context.Context) tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return statusCountsMsg{}
	}
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return statusCountsMsg{}
	}

	counts := make(map[string]int, len(id.StatusDirectories))
	for _, s := range id.StatusDirectories {
		if ctx.Err() != nil {
			break
		}
		statusDir := filepath.Join(festivalsDir, s)
		entries, readErr := os.ReadDir(statusDir)
		if readErr != nil {
			counts[s] = 0
			continue
		}
		count := 0
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
				count++
			}
		}
		counts[s] = count
	}
	return statusCountsMsg{counts: counts}
}

// loadFestivalsForStatus loads festivals for a specific status directory.
func loadFestivalsForStatus(ctx context.Context, status string) tea.Msg {
	cwd, err := os.Getwd()
	if err != nil {
		return childrenLoadedMsg{err: err}
	}
	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return childrenLoadedMsg{err: fmt.Errorf("not in a festivals workspace")}
	}

	festivals, loadErr := show.ListFestivalsByStatusLight(ctx, festivalsDir, status)
	if loadErr != nil {
		return childrenLoadedMsg{err: loadErr}
	}

	var items []FestivalItem
	for _, f := range festivals {
		items = append(items, FestivalItem{
			Name:      f.Name,
			Status:    f.Status,
			CreatedAt: f.ModTime,
			Path:      f.Path,
			Type:      ItemFestival,
		})
	}

	sortFestivalsByCreated(items)
	return childrenLoadedMsg{
		items:      items,
		parentPath: status,
		breadcrumb: statusDisplayName(status),
	}
}

// statusDisplayName maps a status directory path to a user-friendly name.
func statusDisplayName(status string) string {
	switch status {
	case "dungeon/completed":
		return "Completed"
	case "dungeon/archived":
		return "Archived"
	case "dungeon/someday":
		return "Someday"
	case "planning":
		return "Planning"
	case "ready":
		return "Ready"
	case "active":
		return "Active"
	case "ritual":
		return "Ritual"
	default:
		return status
	}
}

// loadChildren loads the child items for a given hierarchy item.
func loadChildren(ctx context.Context, item FestivalItem) ([]FestivalItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	parser := festival.NewParser()

	switch item.Type {
	case ItemFestival:
		phases, err := parser.ParsePhases(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		if len(phases) > 0 {
			return elementsToItems(phases, ItemPhase), nil
		}
		return loadGenericChildren(item.Path)

	case ItemPhase:
		sequences, err := parser.ParseSequences(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		if len(sequences) > 0 {
			return elementsToItems(sequences, ItemSequence), nil
		}
		return loadGenericChildren(item.Path)

	case ItemSequence:
		tasks, err := parser.ParseTasks(ctx, item.Path)
		if err != nil {
			return nil, err
		}
		if len(tasks) > 0 {
			return elementsToItems(tasks, ItemTask), nil
		}
		return loadGenericChildren(item.Path)

	default:
		return nil, nil
	}
}

// loadGenericChildren lists non-numbered subdirectories and .md files as browsable items.
func loadGenericChildren(dir string) ([]FestivalItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var items []FestivalItem
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		// Skip known goal files
		switch name {
		case "FESTIVAL_GOAL.md", "FESTIVAL_OVERVIEW.md", "PHASE_GOAL.md", "SEQUENCE_GOAL.md":
			continue
		}

		if e.IsDir() {
			items = append(items, FestivalItem{
				Name: name,
				Path: filepath.Join(dir, name),
				Type: ItemSequence, // Navigable directory
			})
		} else if strings.HasSuffix(name, ".md") {
			items = append(items, FestivalItem{
				Name: name,
				Path: filepath.Join(dir, name),
				Type: ItemTask, // Leaf node, preview-able
			})
		}
	}
	return items, nil
}

// elementsToItems converts festival parser elements to FestivalItems.
func elementsToItems(elements []festival.FestivalElement, itemType ItemType) []FestivalItem {
	items := make([]FestivalItem, 0, len(elements))
	for _, el := range elements {
		items = append(items, FestivalItem{
			Name: el.FullName,
			Path: el.Path,
			Type: itemType,
		})
	}
	return items
}

// refreshCurrentView reloads the current view's data while preserving tree state.
func (m Model) refreshCurrentView() tea.Cmd {
	ctx := m.ctx

	// At status overview root: reload status counts
	if m.status == "" {
		return func() tea.Msg { return loadStatusCounts(ctx) }
	}

	// At festival list root: reload festivals for this status
	status := m.status
	return func() tea.Msg {
		cwd, err := os.Getwd()
		if err != nil {
			return refreshItemsMsg{}
		}
		festivalsDir, err := workspace.FindFestivals(cwd)
		if err != nil || festivalsDir == "" {
			return refreshItemsMsg{}
		}
		festivals, loadErr := show.ListFestivalsByStatusLight(ctx, festivalsDir, status)
		if loadErr != nil {
			return refreshItemsMsg{}
		}
		var items []FestivalItem
		for _, f := range festivals {
			items = append(items, FestivalItem{
				Name:      f.Name,
				Status:    f.Status,
				CreatedAt: f.ModTime,
				Path:      f.Path,
				Type:      ItemFestival,
			})
		}
		sortFestivalsByCreated(items)
		return refreshItemsMsg{items: items}
	}
}

// watchCmd returns a tea.Cmd that monitors the festivals directory for changes
// and sends refreshMsg to the TUI when files are modified.
func watchCmd(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		cwd, err := os.Getwd()
		if err != nil {
			return nil
		}
		festivalsDir, err := workspace.FindFestivals(cwd)
		if err != nil || festivalsDir == "" {
			return nil
		}

		changed := make(chan struct{}, 1)

		w, err := watch.New(watch.Config{
			Paths:    []string{festivalsDir},
			Debounce: 300 * time.Millisecond,
		}, func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		})
		if err != nil {
			return nil
		}

		go func() {
			_ = w.Watch(ctx)
		}()

		select {
		case <-ctx.Done():
			_ = w.Close()
			return nil
		case <-changed:
			_ = w.Close()
			return refreshMsg{}
		}
	}
}
