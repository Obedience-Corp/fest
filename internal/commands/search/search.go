// Package search implements the fest search command for finding festivals by fuzzy query.
package search

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/id"
	"github.com/Obedience-Corp/fest/internal/navigation"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

type searchOptions struct {
	status string
	json   bool
	limit  int
}

// SearchResult represents a single search match.
type SearchResult struct {
	Name        string `json:"name"`
	ID          string `json:"id,omitempty"`
	Status      string `json:"status"`
	Path        string `json:"path"`
	Score       int    `json:"score"`
	GoalExcerpt string `json:"goal_excerpt,omitempty"`
}

// NewSearchCommand creates the search command.
func NewSearchCommand() *cobra.Command {
	opts := &searchOptions{}

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search festivals by name, ID, or goal text",
		Long: `Search across all festivals in the workspace using fuzzy matching.

Matches against:
  - Festival directory names
  - Festival IDs (from frontmatter)
  - Goal text (from FESTIVAL_OVERVIEW.md)

Results are ranked by relevance with exact and prefix matches scored highest.`,
		Example: `  fest search intents           # Find festivals matching "intents"
  fest search --status active   # Search only active festivals
  fest search RI0001            # Search by festival ID
  fest search --json deploy     # JSON output for scripting`,
		Annotations: map[string]string{
			"scope": string(scope.Workspace),
		},
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), args[0], opts)
		},
	}

	cmd.Flags().StringVar(&opts.status, "status", "", "limit search to status: active|planning|completed|dungeon")
	cmd.Flags().BoolVar(&opts.json, "json", false, "output results as JSON")
	cmd.Flags().IntVar(&opts.limit, "limit", 20, "maximum number of results")

	return cmd
}

func runSearch(ctx context.Context, query string, opts *searchOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled").WithOp("runSearch")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	festivalsDir, err := workspace.FindFestivals(cwd)
	if err != nil || festivalsDir == "" {
		return errors.NotFound("festivals workspace").
			WithField("hint", "run from a project with a festivals/ directory")
	}

	targets := collectSearchTargets(ctx, festivalsDir, opts.status)
	if len(targets) == 0 {
		if opts.json {
			fmt.Println("[]")
		} else {
			fmt.Println(ui.Warning("No festivals found to search."))
		}
		return nil
	}

	var results []SearchResult
	for _, target := range targets {
		bestScore := scoreTarget(query, target)
		if bestScore > 0 {
			results = append(results, SearchResult{
				Name:        target.Name,
				ID:          target.ID,
				Status:      target.Status,
				Path:        target.Path,
				Score:       bestScore,
				GoalExcerpt: truncate(target.GoalText, 80),
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if opts.limit > 0 && len(results) > opts.limit {
		results = results[:opts.limit]
	}

	if opts.json {
		return outputJSON(results)
	}

	return outputText(results, query)
}

// searchTarget holds data about a festival for search matching.
type searchTarget struct {
	Name     string
	ID       string
	Status   string
	Path     string
	GoalText string
}

// scoreTarget scores a search target against the query, returning the best score.
func scoreTarget(query string, target searchTarget) int {
	bestScore := 0
	nameScore, _ := navigation.Score(query, target.Name)
	if nameScore > bestScore {
		bestScore = nameScore
	}
	if target.ID != "" {
		idScore, _ := navigation.Score(query, target.ID)
		if idScore > bestScore {
			bestScore = idScore
		}
	}
	if target.GoalText != "" {
		goalScore, _ := navigation.Score(query, target.GoalText)
		goalScore = goalScore * 7 / 10
		if goalScore > bestScore {
			bestScore = goalScore
		}
	}
	return bestScore
}

// collectSearchTargets gathers all festivals with their searchable metadata.
func collectSearchTargets(ctx context.Context, festivalsDir, statusFilter string) []searchTarget {
	statuses := id.StatusDirectories
	if statusFilter != "" {
		statuses = []string{id.ResolveStatusPath(statusFilter)}
	}

	var targets []searchTarget
	for _, status := range statuses {
		if err := ctx.Err(); err != nil {
			return targets
		}
		statusPath := filepath.Join(festivalsDir, status)
		entries, err := os.ReadDir(statusPath)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			festPath := filepath.Join(statusPath, entry.Name())
			target := searchTarget{
				Name:   entry.Name(),
				Status: status,
				Path:   festPath,
			}
			target.ID = extractFestivalID(festPath)
			target.GoalText = extractGoalText(festPath)
			targets = append(targets, target)
		}
	}
	return targets
}

// extractFestivalID reads the festival ID from FESTIVAL_OVERVIEW.md or FESTIVAL_GOAL.md frontmatter.
func extractFestivalID(festPath string) string {
	for _, filename := range []string{"FESTIVAL_OVERVIEW.md", "FESTIVAL_GOAL.md"} {
		filePath := filepath.Join(festPath, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "fest_id:") {
				return strings.TrimSpace(strings.TrimPrefix(line, "fest_id:"))
			}
		}
	}
	return ""
}

// extractGoalText reads the goal text from FESTIVAL_OVERVIEW.md or FESTIVAL_GOAL.md.
func extractGoalText(festPath string) string {
	for _, filename := range []string{"FESTIVAL_OVERVIEW.md", "FESTIVAL_GOAL.md"} {
		filePath := filepath.Join(festPath, filename)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(content), "\n")
		inGoal := false
		for _, line := range lines {
			if strings.HasPrefix(line, "## Festival Goal") || strings.HasPrefix(line, "## Objective") || strings.HasPrefix(line, "# ") {
				inGoal = true
				continue
			}
			if inGoal {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if strings.HasPrefix(trimmed, "##") || strings.HasPrefix(trimmed, "---") {
					break
				}
				return trimmed
			}
		}
	}
	return ""
}

// truncate limits a string to maxLen characters, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func outputJSON(results []SearchResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(results)
}

func outputText(results []SearchResult, query string) error {
	if len(results) == 0 {
		fmt.Printf("No festivals matching '%s'\n", query)
		return nil
	}

	fmt.Printf("Search results for '%s' (%d matches):\n\n", query, len(results))
	for i, r := range results {
		statusStyle := ui.GetStatusStyle(r.Status)
		fmt.Printf("  %2d. %s %s (score: %d)\n",
			i+1,
			statusStyle.Render(fmt.Sprintf("[%s]", r.Status)),
			r.Name,
			r.Score,
		)
		if r.GoalExcerpt != "" {
			fmt.Printf("      %s\n", ui.Dim(r.GoalExcerpt))
		}
	}
	return nil
}
