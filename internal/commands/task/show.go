package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/spf13/cobra"
)

var (
	showJSON      bool
	showNoContext bool
)

func newShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [task]",
		Short: "Show task details and status",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runShow,
	}

	cmd.Flags().BoolVar(&showJSON, "json", false, "output as JSON")
	cmd.Flags().BoolVar(&showNoContext, "no-context", false, "hide task markdown content")

	return cmd
}

func runShow(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	festivalPath, ok := scope.FestivalFrom(ctx)
	if !ok {
		return errors.Validation("no festival context")
	}

	var arg string
	if len(args) > 0 {
		arg = args[0]
	}

	taskID, taskFilePath, err := resolveTask(ctx, festivalPath, arg)
	if err != nil {
		return err
	}

	mgr, err := progress.NewManager(ctx, festivalPath)
	if err != nil {
		return errors.Wrap(err, "loading progress")
	}

	task, _ := mgr.GetTaskProgress(taskID)
	if task == nil {
		task = &progress.TaskProgress{
			TaskID:   taskID,
			Status:   progress.StatusPending,
			Progress: 0,
		}
	}

	if showJSON {
		result := map[string]any{
			"task_id":  taskID,
			"status":   task.Status,
			"progress": task.Progress,
		}
		if task.BlockerMessage != "" {
			result["blocker"] = task.BlockerMessage
		}
		if task.TimeSpentMinutes > 0 {
			result["time_spent_minutes"] = task.TimeSpentMinutes
		}
		if task.StartedAt != nil {
			result["started_at"] = task.StartedAt
		}
		if task.CompletedAt != nil {
			result["completed_at"] = task.CompletedAt
		}
		return shared.EncodeJSON(os.Stdout, result)
	}

	// Human-readable output
	fmt.Println(ui.H1("Task Details"))

	workType := frontmatter.InferWorkType(taskID)
	workTypeIndicator := ui.FormatWorkType(string(workType))
	taskDisplay := ui.Value(taskID, ui.TaskColor)
	if workTypeIndicator != "" {
		taskDisplay = workTypeIndicator + " " + taskDisplay
	}

	fmt.Printf("%s %s\n", ui.Label("Task"), taskDisplay)
	fmt.Printf("%s %s\n", ui.Label("Status"), ui.GetStateStyle(task.Status).Render(task.Status))
	fmt.Printf("%s %s\n", ui.Label("Progress"), ui.Value(fmt.Sprintf("%d%%", task.Progress)))

	if task.BlockerMessage != "" {
		fmt.Printf("%s %s\n", ui.Label("Blocker"), ui.Warning(task.BlockerMessage))
	}
	if task.TimeSpentMinutes > 0 {
		fmt.Printf("%s %s\n", ui.Label("Time"), ui.Value(ui.FormatDuration(task.TimeSpentMinutes)))
	}
	if task.StartedAt != nil {
		fmt.Printf("%s %s\n", ui.Label("Started"), ui.Value(task.StartedAt.Format("2006-01-02 15:04")))
	}
	if task.CompletedAt != nil {
		fmt.Printf("%s %s\n", ui.Label("Completed"), ui.Value(task.CompletedAt.Format("2006-01-02 15:04")))
	}

	// Show task file path
	fmt.Printf("%s %s\n", ui.Label("File"), ui.Dim(taskFilePath))

	// Inline content unless suppressed
	if !showNoContext {
		content, readErr := os.ReadFile(taskFilePath)
		if readErr == nil && len(content) > 0 {
			fmt.Println()
			fmt.Println(ui.Dim(strings.Repeat("─", 60)))
			fmt.Println()
			// Strip frontmatter if present
			text := string(content)
			if strings.HasPrefix(text, "---") {
				if end := strings.Index(text[3:], "---"); end >= 0 {
					text = strings.TrimLeft(text[3+end+3:], "\n")
				}
			}
			fmt.Print(text)
			if !strings.HasSuffix(text, "\n") {
				fmt.Println()
			}
		}
	}

	return nil
}

// resolveTaskLocationPaths extracts phase and sequence paths from a task ID.
func resolveTaskLocationPaths(festivalPath, taskID string) (phasePath, sequencePath string) {
	parts := strings.Split(taskID, "/")
	if len(parts) >= 1 {
		phasePath = filepath.Join(festivalPath, parts[0])
	}
	if len(parts) >= 2 {
		sequencePath = filepath.Join(festivalPath, parts[0], parts[1])
	}
	return phasePath, sequencePath
}
