// Package list implements the fest list command for listing festivals by status.
package list

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

// validStatuses includes all festival directories plus "dungeon" as shorthand alias.
var validStatuses = func() []string {
	s := make([]string, len(id.StatusDirectories), len(id.StatusDirectories)+1)
	copy(s, id.StatusDirectories)
	return append(s, "dungeon")
}()

// defaultStatuses shown without --all flag.
var defaultStatuses = id.PrimaryStatusDirs

// statusCompletionDescriptions annotates the list status vocabulary for shell
// completion. The vocabulary itself is derived from id.PrimaryStatusDirs and
// id.StatusDirectories (see listStatusCompletions) so the two cannot drift; this
// map only supplies human-readable descriptions.
var statusCompletionDescriptions = map[string]string{
	"active":            "Festivals currently in progress",
	"ready":             "Festivals prepared and awaiting execution",
	"planning":          "Festivals being designed",
	"ritual":            "Recurring or special festivals",
	"completed":         "Successfully finished festivals",
	"dungeon":           "All shelved festivals (completed, archived, someday)",
	"dungeon/completed": "Completed festivals bucketed by date",
	"dungeon/archived":  "Festivals shelved but preserved",
	"dungeon/someday":   "Festivals deprioritized for later",
	"all":               "Every festival grouped by status",
}

// listStatusCompletions returns the ordered status vocabulary offered by tab
// completion. Values are sourced from id.PrimaryStatusDirs and
// id.StatusDirectories plus the "completed"/"dungeon" shorthands and the "all"
// alias, so no second hardcoded status list can drift from the id package.
func listStatusCompletions() []string {
	values := make([]string, 0, len(id.PrimaryStatusDirs)+len(id.StatusDirectories)+3)
	values = append(values, id.PrimaryStatusDirs...)
	values = append(values, "completed", "dungeon")
	for _, s := range id.StatusDirectories {
		if strings.HasPrefix(s, "dungeon/") {
			values = append(values, s)
		}
	}
	return append(values, "all")
}

// completeListStatus provides tab completion for the first positional status
// argument, offering the status vocabulary with descriptions.
func completeListStatus(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	out := make([]string, 0, len(statusCompletionDescriptions))
	for _, v := range listStatusCompletions() {
		if toComplete != "" && !strings.HasPrefix(v, toComplete) {
			continue
		}
		if desc := statusCompletionDescriptions[v]; desc != "" {
			out = append(out, v+"\t"+desc)
		} else {
			out = append(out, v)
		}
	}
	return out, cobra.ShellCompDirectiveNoFileComp
}

type listOptions struct {
	json          bool
	all           bool
	progress      bool
	alpha         bool
	status        string
	sortBy        string
	filterProject string
	since         string
	until         string
}

// NewListCommand creates the list command for listing festivals by status.
func NewListCommand() *cobra.Command {
	opts := &listOptions{}

	cmd := &cobra.Command{
		Use:   "list [status]",
		Short: "List festivals by status",
		Long: `List festivals filtered by status.

Works from anywhere - finds the festivals workspace automatically.

STATUS can be: active, ready, planning, ritual, completed, all,
dungeon, dungeon/completed, dungeon/archived, dungeon/someday

By default, shows active, ready, planning, and ritual festivals.
Use 'fest list all' (or --all) to include completed and dungeon festivals.`,
		Example: `  fest list                                        # Active, ready, planning, ritual festivals
  fest list active                                 # Only active festivals
  fest list all                                    # Every festival grouped by status
  fest list dungeon/completed                      # Completed festivals in the dungeon
  fest list --filter-project camp                  # Festivals linked to "camp" project
  fest list active --sort progress                 # Active festivals, most complete first
  fest list --since 2026-01-01 --until 2026-02-01  # Created in January 2026
  fest list --json                                 # Output in JSON format`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeListStatus,
		RunE: func(cmd *cobra.Command, args []string) error {
			status := opts.status
			if len(args) > 0 {
				status = strings.ToLower(args[0])
			}
			if status == "all" {
				// `fest list all` is an alias for `fest list --all`.
				opts.all = true
				status = ""
			}
			if status != "" {
				status = id.ResolveStatusPath(status)
			}
			if status != "" && !isValidStatus(status) {
				return errors.Validation("invalid status").
					WithField("status", status).
					WithField("valid", strings.Join(validStatuses, ", "))
			}
			if opts.sortBy != "" && !isValidSortBy(opts.sortBy) {
				return errors.Validation("invalid sort").
					WithField("sort", opts.sortBy).
					WithField("valid", "date, status, progress, name, created, updated")
			}
			if opts.since != "" {
				if _, err := parseFilterDate(opts.since); err != nil {
					return err
				}
			}
			if opts.until != "" {
				if _, err := parseFilterDate(opts.until); err != nil {
					return err
				}
			}
			return runList(cmd.Context(), status, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	cmd.Flags().BoolVar(&opts.all, "all", false, "include completed and dungeon festivals")
	cmd.Flags().BoolVar(&opts.progress, "progress", false, "show detailed progress for each festival")
	cmd.Flags().BoolVar(&opts.alpha, "alpha", false, "sort alphabetically by name instead of by date")
	cmd.Flags().StringVar(&opts.status, "status", "", "filter by status: active|planning|completed|dungeon")
	cmd.Flags().StringVar(&opts.sortBy, "sort", "", "sort by: date|status|progress|name|created|updated")
	cmd.Flags().StringVar(&opts.filterProject, "filter-project", "", "filter festivals linked to a project path (substring match)")
	cmd.Flags().StringVar(&opts.since, "since", "", "show festivals created on or after this date (YYYY-MM-DD or RFC3339)")
	cmd.Flags().StringVar(&opts.until, "until", "", "show festivals created on or before this date (YYYY-MM-DD or RFC3339)")

	return cmd
}

func isValidStatus(status string) bool {
	for _, v := range validStatuses {
		if v == status {
			return true
		}
	}
	return false
}

func runList(ctx context.Context, filterStatus string, opts *listOptions) error {
	// Find festivals workspace from anywhere
	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return errors.NotFound("festivals workspace").
			WithField("hint", "run from a project with a festivals/ directory or use 'fest init --register'")
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, "")

	if filterStatus != "" {
		// "dungeon" without substatus: list all dungeon children
		if filterStatus == "dungeon" {
			return listDungeon(ctx, festivalsDir, opts, campaignRoot)
		}
		// List single status
		return listByStatus(ctx, festivalsDir, filterStatus, opts, campaignRoot)
	}

	// List all statuses
	return listAll(ctx, festivalsDir, opts, campaignRoot)
}

// validSortValues defines the accepted sort field names.
var validSortValues = []string{"date", "status", "progress", "name", "created", "updated"}

func isValidSortBy(s string) bool {
	for _, v := range validSortValues {
		if v == s {
			return true
		}
	}
	return false
}

// statusOrder returns a numeric rank for sorting by status.
func statusOrder(s string) int {
	switch s {
	case "active":
		return 0
	case "ready":
		return 1
	case "planning":
		return 2
	case "completed":
		return 3
	case "dungeon":
		return 4
	case "dungeon/completed":
		return 5
	case "dungeon/archived":
		return 6
	case "dungeon/someday":
		return 7
	default:
		return 8
	}
}

func sortByDate(festivals []*show.FestivalInfo) {
	sort.Slice(festivals, func(i, j int) bool {
		return festivals[i].ModTime.After(festivals[j].ModTime)
	})
}

// sortByStatusDate orders festivals by their dungeon bucket date, newest
// first. ISO YYYY-MM-DD values compare lexicographically. Festivals with an
// empty StatusDate fall to the bottom and are further ordered by ModTime
// desc so non-bucketed dungeon entries still have a deterministic position.
func sortByStatusDate(festivals []*show.FestivalInfo) {
	sort.SliceStable(festivals, func(i, j int) bool {
		a, b := festivals[i].StatusDate, festivals[j].StatusDate
		if a == b {
			if festivals[i].ModTime.Equal(festivals[j].ModTime) {
				return festivals[i].Name < festivals[j].Name
			}
			return festivals[i].ModTime.After(festivals[j].ModTime)
		}
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		return a > b
	})
}

// applySorting applies the requested sort order to a festival list.
func applySorting(festivals []*show.FestivalInfo, sortBy string, alpha bool) {
	switch sortBy {
	case "status":
		sort.Slice(festivals, func(i, j int) bool {
			return statusOrder(festivals[i].Status) < statusOrder(festivals[j].Status)
		})
	case "progress":
		sort.Slice(festivals, func(i, j int) bool {
			pi, pj := 0.0, 0.0
			if festivals[i].Stats != nil {
				pi = festivals[i].Stats.Progress
			}
			if festivals[j].Stats != nil {
				pj = festivals[j].Stats.Progress
			}
			return pi > pj // descending
		})
	case "name":
		sort.Slice(festivals, func(i, j int) bool {
			return festivals[i].Name < festivals[j].Name
		})
	case "created":
		sort.Slice(festivals, func(i, j int) bool {
			return festivals[i].CreatedAt.After(festivals[j].CreatedAt)
		})
	case "updated":
		sort.Slice(festivals, func(i, j int) bool {
			return festivals[i].UpdatedAt.After(festivals[j].UpdatedAt)
		})
	case "date", "":
		if alpha {
			sort.Slice(festivals, func(i, j int) bool {
				return festivals[i].Name < festivals[j].Name
			})
		} else {
			sortByDate(festivals)
		}
	}
}

// parseFilterDate parses a date string in YYYY-MM-DD or RFC3339 format.
func parseFilterDate(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, errors.Validation("invalid date format").
		WithField("value", s).
		WithField("expected", "YYYY-MM-DD or RFC3339")
}

// applyFilters applies project and date range filters to a festival list.
func applyFilters(festivals []*show.FestivalInfo, opts *listOptions) ([]*show.FestivalInfo, error) {
	if opts.filterProject == "" && opts.since == "" && opts.until == "" {
		return festivals, nil
	}

	var sinceTime, untilTime time.Time
	var err error

	if opts.since != "" {
		sinceTime, err = parseFilterDate(opts.since)
		if err != nil {
			return nil, err
		}
	}
	if opts.until != "" {
		untilTime, err = parseFilterDate(opts.until)
		if err != nil {
			return nil, err
		}
		untilTime = untilTime.Add(24*time.Hour - time.Nanosecond)
	}

	result := make([]*show.FestivalInfo, 0, len(festivals))
	for _, f := range festivals {
		if opts.filterProject != "" {
			if !strings.Contains(strings.ToLower(f.ProjectPath), strings.ToLower(opts.filterProject)) {
				continue
			}
		}
		if !sinceTime.IsZero() && !f.CreatedAt.IsZero() && f.CreatedAt.Before(sinceTime) {
			continue
		}
		if !untilTime.IsZero() && !f.CreatedAt.IsZero() && f.CreatedAt.After(untilTime) {
			continue
		}
		result = append(result, f)
	}
	return result, nil
}

// dungeonSubstatuses defines the valid dungeon child statuses.
var dungeonSubstatuses = []string{"dungeon/completed", "dungeon/archived", "dungeon/someday"}

func listDungeon(ctx context.Context, festivalsDir string, opts *listOptions, campaignRoot string) error {
	result := make(map[string]interface{})
	var totalCount int
	allFestivals := make(map[string][]*show.FestivalInfo)
	var allFestivalsList []*show.FestivalInfo

	order := make([]string, 0, len(dungeonSubstatuses))
	for _, status := range dungeonSubstatuses {
		festivals, err := show.ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
		if err != nil {
			continue
		}
		festivals, err = applyFilters(festivals, opts)
		if err != nil {
			return err
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

	if opts.json {
		for status, festivals := range allFestivals {
			result[status] = festivalsToMapWithProgress(festivals, progressMap)
		}
		result["total"] = totalCount
		return outputJSON(result)
	}

	if totalCount == 0 {
		fmt.Println(ui.Warning("No festivals in dungeon."))
		return nil
	}

	if opts.progress {
		fmt.Print(show.FormatAllFestivalsWithProgress(allFestivals, order, progressMap))
	} else {
		fmt.Print(show.FormatAllFestivals(allFestivals, order))
	}
	return nil
}

func listByStatus(ctx context.Context, festivalsDir, status string, opts *listOptions, campaignRoot string) error {
	festivals, err := show.ListFestivalsByStatus(ctx, festivalsDir, status, campaignRoot)
	if err != nil {
		return err
	}

	festivals, err = applyFilters(festivals, opts)
	if err != nil {
		return err
	}

	// Dungeon listings default to newest bucket date first; explicit sort
	// flags still take precedence.
	if opts.sortBy == "" && !opts.alpha && strings.HasPrefix(status, "dungeon/") {
		sortByStatusDate(festivals)
	} else {
		applySorting(festivals, opts.sortBy, opts.alpha)
	}

	// Fetch detailed progress if requested
	var progressMap map[string]*progress.FestivalProgress
	if opts.progress {
		progressMap = fetchProgressForFestivals(ctx, festivals)
	}

	if opts.json {
		return outputJSON(map[string]interface{}{
			"status":    status,
			"count":     len(festivals),
			"festivals": festivalsToMapWithProgress(festivals, progressMap),
		})
	}

	if opts.progress {
		fmt.Print(show.FormatFestivalListWithProgress(status, festivals, progressMap))
	} else {
		fmt.Print(show.FormatFestivalList(status, festivals))
	}

	return nil
}

func listAll(ctx context.Context, festivalsDir string, opts *listOptions, campaignRoot string) error {
	result := make(map[string]interface{})
	var totalCount int
	allFestivals := make(map[string][]*show.FestivalInfo)

	// Use all statuses if --all flag, otherwise just active/planning
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
			return err
		}
		if len(festivals) > 0 {
			applySorting(festivals, opts.sortBy, opts.alpha)
			allFestivals[status] = festivals
			statusOrder = append(statusOrder, status)
			totalCount += len(festivals)
			allFestivalsList = append(allFestivalsList, festivals...)
		}
	}

	// Fetch detailed progress if requested
	var progressMap map[string]*progress.FestivalProgress
	if opts.progress {
		progressMap = fetchProgressForFestivals(ctx, allFestivalsList)
	}

	if opts.json {
		for status, festivals := range allFestivals {
			result[status] = festivalsToMapWithProgress(festivals, progressMap)
		}
		result["total"] = totalCount
		return outputJSON(result)
	}

	if totalCount == 0 {
		fmt.Println(ui.Warning("No festivals found."))
		fmt.Println(ui.Info("Create a festival with: fest create festival"))
		return nil
	}

	if opts.progress {
		fmt.Print(show.FormatAllFestivalsWithProgress(allFestivals, statusOrder, progressMap))
	} else {
		fmt.Print(show.FormatAllFestivals(allFestivals, statusOrder))
	}
	return nil
}

func festivalsToMap(festivals []*show.FestivalInfo) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(festivals))
	for _, f := range festivals {
		m := map[string]interface{}{
			"name":   f.Name,
			"path":   f.Path,
			"status": f.Status,
		}
		if f.StatusDate != "" {
			m["status_date"] = f.StatusDate
		}
		if f.Stats != nil {
			m["progress"] = f.Stats.Progress
		}
		if !f.CreatedAt.IsZero() {
			m["created_at"] = f.CreatedAt.Format(time.RFC3339)
		}
		if !f.UpdatedAt.IsZero() {
			m["updated_at"] = f.UpdatedAt.Format(time.RFC3339)
		}
		result = append(result, m)
	}
	return result
}

func outputJSON(data interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

// fetchProgressForFestivals fetches detailed progress for each festival.
// Returns a map from festival path to progress data.
// Silently skips festivals where progress cannot be fetched.
func fetchProgressForFestivals(ctx context.Context, festivals []*show.FestivalInfo) map[string]*progress.FestivalProgress {
	progressMap := make(map[string]*progress.FestivalProgress)
	for _, f := range festivals {
		mgr, err := progress.NewManager(ctx, f.Path)
		if err != nil {
			continue // Silently skip
		}
		prog, err := mgr.GetFestivalProgress(ctx, f.Path)
		if err != nil {
			continue // Silently skip
		}
		progressMap[f.Path] = prog
	}
	return progressMap
}

// festivalsToMapWithProgress converts festivals to map with optional detailed progress.
func festivalsToMapWithProgress(festivals []*show.FestivalInfo, progressMap map[string]*progress.FestivalProgress) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(festivals))
	for _, f := range festivals {
		m := map[string]interface{}{
			"name":   f.Name,
			"path":   f.Path,
			"status": f.Status,
		}
		if f.StatusDate != "" {
			m["status_date"] = f.StatusDate
		}
		if f.Stats != nil {
			m["progress"] = f.Stats.Progress
		}
		if !f.CreatedAt.IsZero() {
			m["created_at"] = f.CreatedAt.Format(time.RFC3339)
		}
		if !f.UpdatedAt.IsZero() {
			m["updated_at"] = f.UpdatedAt.Format(time.RFC3339)
		}
		// Add detailed progress if available
		if progressMap != nil {
			if prog, ok := progressMap[f.Path]; ok && prog != nil && prog.Overall != nil {
				m["tasks"] = map[string]interface{}{
					"total":       prog.Overall.Total,
					"completed":   prog.Overall.Completed,
					"in_progress": prog.Overall.InProgress,
					"blocked":     prog.Overall.Blocked,
					"pending":     prog.Overall.Pending,
				}
				m["time_spent_minutes"] = prog.Overall.TimeSpentMin
			}
		}
		result = append(result, m)
	}
	return result
}
