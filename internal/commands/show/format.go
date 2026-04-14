package show

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/pathutil"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

// FormatNodeReference creates a node reference string from festival ID and location.
// Format: ID:P###.S##.T## (e.g., GU0001:P002.S01.T03)
// Returns empty string if festivalID is empty.
func FormatNodeReference(festivalID string, phase, sequence, task int) string {
	if festivalID == "" {
		return ""
	}
	return fmt.Sprintf("%s:P%03d.S%02d.T%02d", festivalID, phase, sequence, task)
}

// FormatFestivalDetails formats a single festival with full details.
func FormatFestivalDetails(festival *FestivalInfo, verbose bool, campaignRoot string) string {
	var sb strings.Builder

	// Header
	sb.WriteString(ui.H1("Festival"))
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "%s %s\n", ui.Label("Name"), ui.Value(festival.Name, ui.FestivalColor))

	// Display festival ID prominently
	if festival.MetadataID != "" {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("ID"), ui.Value(festival.MetadataID))
	} else {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("ID"), ui.Dim("No ID (run fest migrate to add)"))
	}

	fmt.Fprintf(&sb, "%s %s\n", ui.Label("Status"), ui.GetStatusStyle(festival.Status).Render(festival.Status))

	// Display campaign-relative paths
	displayFestPath := festival.Path
	displayProjectPath := festival.ProjectPath
	if campaignRoot != "" {
		displayFestPath = pathutil.DisplayPath(festival.Path, campaignRoot)
		displayProjectPath = pathutil.DisplayPath(festival.ProjectPath, campaignRoot)
	}

	fmt.Fprintf(&sb, "%s %s\n", ui.Label("Path"), ui.Dim(displayFestPath))

	if festival.ProjectPath != "" {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("Project"), ui.Value(displayProjectPath, ui.SequenceColor))
	}

	if festival.Priority != "" {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("Priority"), ui.Value(festival.Priority))
	}

	if !festival.CreatedAt.IsZero() {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("Created"), ui.Dim(formatTimestamp(festival.CreatedAt)))
	}
	if !festival.UpdatedAt.IsZero() {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("Updated"), ui.Dim(formatTimestamp(festival.UpdatedAt)))
	}

	// Statistics
	if festival.Stats != nil {
		sb.WriteString("\n")
		sb.WriteString(ui.H2("Progress"))
		sb.WriteString("\n")
		fmt.Fprintf(&sb, "%s %s %s\n",
			ui.Label("Overall"),
			renderPercentBar(festival.Stats.Progress),
			ui.Value(fmt.Sprintf("%.1f%%", festival.Stats.Progress)))

		sb.WriteString("\n")
		sb.WriteString(ui.H3("Phases"))
		sb.WriteString("\n")
		sb.WriteString(formatStatusCounts("  ", festival.Stats.Phases))

		sb.WriteString("\n")
		sb.WriteString(ui.H3("Sequences"))
		sb.WriteString("\n")
		sb.WriteString(formatStatusCounts("  ", festival.Stats.Sequences))

		sb.WriteString("\n")
		sb.WriteString(ui.H3("Tasks"))
		sb.WriteString("\n")
		sb.WriteString(formatStatusCounts("  ", festival.Stats.Tasks))

		if festival.Stats.Gates.Total > 0 {
			sb.WriteString("\n")
			sb.WriteString(ui.H3("Gates"))
			sb.WriteString("\n")
			fmt.Fprintf(&sb, "  %s %s\n", ui.Label("Total"), ui.Value(fmt.Sprintf("%d", festival.Stats.Gates.Total)))
			fmt.Fprintf(&sb, "  %s %s\n", ui.Label("Passed"), ui.GetStateStyle("completed").Render(fmt.Sprintf("%d", festival.Stats.Gates.Passed)))
			fmt.Fprintf(&sb, "  %s %s\n", ui.Label("Failed"), ui.GetStateStyle("blocked").Render(fmt.Sprintf("%d", festival.Stats.Gates.Failed)))
		}
	}

	return sb.String()
}

func formatStatusCounts(prefix string, counts StatusCounts) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%s%s %s\n", prefix, ui.Label("Total"), ui.Value(fmt.Sprintf("%d", counts.Total)))
	fmt.Fprintf(&sb, "%s%s %s\n", prefix, ui.Label("Completed"), ui.GetStateStyle("completed").Render(fmt.Sprintf("%d", counts.Completed)))
	fmt.Fprintf(&sb, "%s%s %s\n", prefix, ui.Label("In progress"), ui.GetStateStyle("in_progress").Render(fmt.Sprintf("%d", counts.InProgress)))
	fmt.Fprintf(&sb, "%s%s %s\n", prefix, ui.Label("Pending"), ui.GetStateStyle("pending").Render(fmt.Sprintf("%d", counts.Pending)))
	if counts.Blocked > 0 {
		fmt.Fprintf(&sb, "%s%s %s\n", prefix, ui.Label("Blocked"), ui.GetStateStyle("blocked").Render(fmt.Sprintf("%d", counts.Blocked)))
	}
	return sb.String()
}

// FormatFestivalList formats a list of festivals for a single status.
func FormatFestivalList(status string, festivals []*FestivalInfo) string {
	var sb strings.Builder

	header := fmt.Sprintf("%s Festivals (%d)", strings.ToUpper(status), len(festivals))
	sb.WriteString(ui.GetStatusStyle(status).Render(header))
	sb.WriteString("\n")
	sb.WriteString(ui.Dim(strings.Repeat("─", 40)))
	sb.WriteString("\n")

	if len(festivals) == 0 {
		sb.WriteString(ui.Dim("  (none)\n"))
		return sb.String()
	}

	isDungeon := strings.HasPrefix(status, "dungeon/")
	for _, f := range festivals {
		progress := ""
		if f.Stats != nil {
			progress = ui.Dim(fmt.Sprintf(" [%.0f%%]", f.Stats.Progress))
		}
		moved := ""
		if isDungeon && f.StatusDate != "" {
			moved = ui.Dim(fmt.Sprintf("  moved %s", f.StatusDate))
		}
		styledName := ui.GetStatusStyle(status).Render(f.Name)
		fmt.Fprintf(&sb, "  %s%s%s\n", styledName, moved, progress)
	}

	return sb.String()
}

// FormatFestivalListWithProgress formats a list of festivals with detailed progress info.
func FormatFestivalListWithProgress(status string, festivals []*FestivalInfo, progressMap map[string]*progress.FestivalProgress) string {
	var sb strings.Builder

	header := fmt.Sprintf("%s Festivals (%d)", strings.ToUpper(status), len(festivals))
	sb.WriteString(ui.GetStatusStyle(status).Render(header))
	sb.WriteString("\n")
	sb.WriteString(ui.Dim(strings.Repeat("─", 40)))
	sb.WriteString("\n")

	if len(festivals) == 0 {
		sb.WriteString(ui.Dim("  (none)\n"))
		return sb.String()
	}

	isDungeon := strings.HasPrefix(status, "dungeon/")
	for _, f := range festivals {
		styledName := ui.GetStatusStyle(status).Render(f.Name)
		fmt.Fprintf(&sb, "  %s\n", styledName)

		// Show moved-to-status date for dungeon festivals.
		if isDungeon && f.StatusDate != "" {
			fmt.Fprintf(&sb, "    %s %s\n", ui.Label("Moved"), ui.Dim(f.StatusDate))
		}
		// Show timestamps
		if !f.CreatedAt.IsZero() {
			fmt.Fprintf(&sb, "    %s %s\n", ui.Label("Created"), ui.Dim(formatTimestamp(f.CreatedAt)))
		}
		if !f.UpdatedAt.IsZero() {
			fmt.Fprintf(&sb, "    %s %s\n", ui.Label("Updated"), ui.Dim(formatTimestamp(f.UpdatedAt)))
		}

		// Show detailed progress if available
		if prog, ok := progressMap[f.Path]; ok && prog != nil && prog.Overall != nil {
			overall := prog.Overall
			// Progress bar with percentage and task counts
			bar := renderPercentBar(float64(overall.Percentage))
			fmt.Fprintf(&sb, "    %s %s %s %s\n",
				ui.Label("Overall"),
				bar,
				ui.Value(fmt.Sprintf("%d%%", overall.Percentage)),
				ui.Dim(fmt.Sprintf("(%d/%d tasks)", overall.Completed, overall.Total)))

			// Total time if available
			if overall.TimeSpentMin > 0 {
				fmt.Fprintf(&sb, "    %s %s\n",
					ui.Label("Total time"),
					ui.Value(ui.FormatDuration(overall.TimeSpentMin)))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// FormatAllFestivalsWithProgress formats all festivals grouped by status with detailed progress.
func FormatAllFestivalsWithProgress(allFestivals map[string][]*FestivalInfo, statusOrder []string, progressMap map[string]*progress.FestivalProgress) string {
	var sb strings.Builder

	total := 0
	for _, festivals := range allFestivals {
		total += len(festivals)
	}

	sb.WriteString(ui.H1("All Festivals"))
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "%s %s\n", ui.Label("Total"), ui.Value(fmt.Sprintf("%d", total)))
	sb.WriteString(ui.Dim(strings.Repeat("─", 40)))
	sb.WriteString("\n\n")

	for _, status := range statusOrder {
		festivals := allFestivals[status]
		sb.WriteString(FormatFestivalListWithProgress(status, festivals, progressMap))
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatAllFestivals formats all festivals grouped by status.
func FormatAllFestivals(allFestivals map[string][]*FestivalInfo, statusOrder []string) string {
	var sb strings.Builder

	total := 0
	for _, festivals := range allFestivals {
		total += len(festivals)
	}

	sb.WriteString(ui.H1("All Festivals"))
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "%s %s\n", ui.Label("Total"), ui.Value(fmt.Sprintf("%d", total)))
	sb.WriteString(ui.Dim(strings.Repeat("─", 40)))
	sb.WriteString("\n\n")

	for _, status := range statusOrder {
		festivals := allFestivals[status]
		sb.WriteString(FormatFestivalList(status, festivals))
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatLocation formats the current location within a festival.
func FormatLocation(loc *LocationInfo) string {
	var sb strings.Builder

	if loc.Festival == nil {
		return "Not in a festival directory\n"
	}

	sb.WriteString(ui.H1("Location"))
	sb.WriteString("\n")
	fmt.Fprintf(&sb, "%s %s\n", ui.Label("Festival"), ui.Value(loc.Festival.Name, ui.FestivalColor))
	fmt.Fprintf(&sb, "%s %s\n", ui.Label("Location"), ui.Value(loc.Type))

	if loc.Phase != "" {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("Phase"), ui.Value(loc.Phase, ui.PhaseColor))
	}
	if loc.Sequence != "" {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("Sequence"), ui.Value(loc.Sequence, ui.SequenceColor))
	}
	if loc.Task != "" {
		fmt.Fprintf(&sb, "%s %s\n", ui.Label("Task"), ui.Value(loc.Task, ui.TaskColor))
	}

	return sb.String()
}

// formatTimestamp formats a time for display.
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.Format("2006-01-02 15:04")
}

func renderPercentBar(progress float64) string {
	opts := ui.DefaultProgressBarOptions()
	opts.Current = int(math.Round(progress))
	opts.Total = 100
	opts.Width = 24
	opts.ShowPercentage = false
	opts.ShowFraction = false
	opts.FilledColor = ui.SuccessColor
	opts.EmptyColor = ui.BorderColor
	return ui.RenderProgressBar(opts)
}
