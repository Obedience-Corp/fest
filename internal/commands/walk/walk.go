package walk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/config"
	"github.com/Obedience-Corp/fest/internal/errors"
	gatescore "github.com/Obedience-Corp/fest/internal/gates"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/scope"
	tpl "github.com/Obedience-Corp/fest/internal/template"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workspace"
	"github.com/spf13/cobra"
)

type walkOptions struct {
	json bool
	from string
}

// WalkView is the structured view returned by fest walk.
type WalkView struct {
	Kind     string                       `json:"kind"`
	Name     string                       `json:"name"`
	Status   string                       `json:"status"`
	Path     string                       `json:"path"`
	Goal     string                       `json:"goal,omitempty"`
	Progress *walkProgress                `json:"progress,omitempty"`
	Next     string                       `json:"next,omitempty"`
	Blocked  []walkBlocker                `json:"blocked,omitempty"`
	Gates    []string                     `json:"gates,omitempty"`
	Warnings []string                     `json:"warnings,omitempty"`
	Workflow *show.StandaloneWorkflowJSON `json:"workflow,omitempty"`
}

type walkProgress struct {
	Completed  int `json:"completed"`
	Total      int `json:"total"`
	Percentage int `json:"percentage"`
}

type walkBlocker struct {
	Task   string `json:"task"`
	Reason string `json:"reason"`
}

// NewWalkCommand creates the walk/inspect command.
func NewWalkCommand() *cobra.Command {
	opts := &walkOptions{}

	cmd := &cobra.Command{
		Use:     "walk [path]",
		Aliases: []string{"inspect"},
		Short:   "Guided overview of a festival or workflow",
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		Long: `Display a guided orientation overview of a festival or workflow.

For festivals, shows what the festival is, where it is, its current status
and progress, the next task, blocked tasks, active quality gates, and any
warnings. For standalone WORKFLOW.md files, shows workflow mode, run status,
step progress, and the current step. This is a read-only orientation command;
it never mutates festival or workflow state.

Useful for quickly orienting inside a festival or standalone workflow before
continuing work, especially for rituals where the template and active run are
distinct.

EXAMPLES:
  fest walk                      # Walk current festival or WORKFLOW.md from cwd
  fest walk festivals/active/my-festival
  fest walk path/to/workflow-dir
  fest walk --json               # Machine-readable output`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if len(args) > 0 {
				opts.from = args[0]
			}
			return runWalk(ctx, opts)
		},
	}

	cmd.Flags().BoolVar(&opts.json, "json", false, "output in JSON format")
	cmd.Flags().StringVar(&opts.from, "from", "", "festival path (defaults to current directory)")

	return cmd
}

func runWalk(ctx context.Context, opts *walkOptions) error {
	if err := ctx.Err(); err != nil {
		return errors.Wrap(err, "context cancelled")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return errors.IO("getting current directory", err)
	}

	startDir := cwd
	if opts.from != "" {
		if filepath.IsAbs(opts.from) {
			startDir = opts.from
		} else {
			startDir = filepath.Join(cwd, opts.from)
		}
	}

	campaignRoot, _ := workspace.DetectCampaign(ctx, startDir)

	standaloneWorkflow, err := show.ResolveStandaloneWorkflow(ctx, startDir)
	if err != nil {
		return errors.Wrap(err, "resolving standalone workflow")
	}
	if standaloneWorkflow != nil {
		view := buildStandaloneWalkView(standaloneWorkflow)
		if opts.json {
			return emitJSON(view)
		}
		return emitStandaloneText(view, standaloneWorkflow, campaignRoot)
	}

	festival, err := resolveFestival(ctx, startDir, campaignRoot)
	if err != nil {
		return errors.Wrap(err, "not inside a festival or standalone workflow").
			WithHint("Run from within a festival directory, a directory containing WORKFLOW.md, or pass a path as an argument")
	}

	festivalPath := festival.Path

	view := &WalkView{
		Kind:   detectKind(festivalPath, festival.Status),
		Name:   festival.Name,
		Status: festival.Status,
		Path:   festivalPath,
	}

	view.Goal = loadGoal(festivalPath)
	view.Warnings = append(view.Warnings, kindWarnings(view.Kind, festival)...)

	mgr, mgrErr := progress.NewManagerReadOnly(ctx, festivalPath)
	if mgrErr == nil {
		festProgress, progressErr := mgr.GetFestivalProgress(ctx, festivalPath)
		if progressErr == nil && festProgress.Overall != nil {
			view.Progress = &walkProgress{
				Completed:  festProgress.Overall.Completed,
				Total:      festProgress.Overall.Total,
				Percentage: festProgress.Overall.Percentage,
			}
			for _, b := range festProgress.Overall.Blockers {
				view.Blocked = append(view.Blocked, walkBlocker{
					Task:   b.TaskID,
					Reason: b.BlockerMessage,
				})
			}
		}
	}

	view.Next = resolveNext(ctx, festivalPath, cwd)

	if len(view.Blocked) > 0 {
		for _, b := range view.Blocked {
			if b.Task == view.Next {
				view.Warnings = append(view.Warnings, fmt.Sprintf("next task is blocked: %s", b.Reason))
			}
		}
	}

	view.Gates = loadActiveGates(ctx, festivalPath)

	if opts.json {
		return emitJSON(view)
	}
	return emitText(view, festivalPath, campaignRoot)
}

func resolveFestival(ctx context.Context, startDir, campaignRoot string) (*show.FestivalInfo, error) {
	festivalPath, resolveErr := shared.ResolveFestivalPath(startDir, "")
	if resolveErr == nil && festivalPath != "" {
		return show.DetectCurrentFestival(ctx, festivalPath, campaignRoot)
	}
	return show.DetectCurrentFestival(ctx, startDir, campaignRoot)
}

func buildStandaloneWalkView(info *show.StandaloneWorkflowInfo) *WalkView {
	workflow := show.NewStandaloneWorkflowJSON(info)
	view := &WalkView{
		Kind:   "workflow",
		Name:   filepath.Base(info.StartDir),
		Status: info.RunStatus,
		Path:   info.WorkflowDoc,
		Progress: &walkProgress{
			Completed:  info.CompletedSteps,
			Total:      info.TotalSteps,
			Percentage: progressPercentage(info.CompletedSteps, info.TotalSteps),
		},
		Workflow: &workflow,
	}

	for _, step := range info.Steps {
		if step.IsCurrent {
			view.Next = fmt.Sprintf("Step %d: %s", step.Number, step.Name)
		}
	}

	if info.Blocked && view.Next != "" {
		view.Blocked = append(view.Blocked, walkBlocker{
			Task:   view.Next,
			Reason: "workflow run is blocked",
		})
	}
	if info.DocHashChanged {
		view.Warnings = append(view.Warnings, "WORKFLOW.md has changed since this run started")
	}
	return view
}

func progressPercentage(completed, total int) int {
	if total <= 0 {
		return 0
	}
	if completed < 0 {
		completed = 0
	}
	if completed > total {
		completed = total
	}
	return completed * 100 / total
}

func detectKind(festivalPath, status string) string {
	if strings.Contains(status, "dungeon") {
		return "dungeon-run"
	}

	festivalsRoot := filepath.Dir(filepath.Dir(festivalPath))
	ritualDir := filepath.Join(festivalsRoot, "ritual")
	parentDir := filepath.Dir(festivalPath)

	if parentDir == ritualDir {
		return "ritual-template"
	}

	return "festival"
}

func loadGoal(festivalPath string) string {
	for _, name := range []string{"FESTIVAL_GOAL.md", "FESTIVAL_OVERVIEW.md"} {
		goalPath := filepath.Join(festivalPath, name)
		data, err := os.ReadFile(goalPath)
		if err != nil {
			continue
		}
		lines := strings.Split(string(data), "\n")
		inFrontmatter := false
		frontmatterDone := false
		seenOpenDelim := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if !frontmatterDone {
				if trimmed == "---" {
					if !seenOpenDelim {
						seenOpenDelim = true
						inFrontmatter = true
						continue
					}
					inFrontmatter = false
					frontmatterDone = true
					continue
				}
				if inFrontmatter {
					continue
				}
			}
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if len([]rune(trimmed)) > 300 {
				trimmed = string([]rune(trimmed)[:300]) + "..."
			}
			return trimmed
		}
	}
	return ""
}

func resolveNext(ctx context.Context, festivalPath, cwd string) string {
	if err := ctx.Err(); err != nil {
		return ""
	}

	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return ""
	}

	mgr, mgrErr := progress.NewManagerReadOnly(ctx, festivalPath)

	for _, entry := range entries {
		if !entry.IsDir() || !hasNumericPrefix(entry.Name()) {
			continue
		}
		phasePath := filepath.Join(festivalPath, entry.Name())
		seqEntries, err := os.ReadDir(phasePath)
		if err != nil {
			continue
		}
		for _, seqEntry := range seqEntries {
			if !seqEntry.IsDir() || !hasNumericPrefix(seqEntry.Name()) {
				continue
			}
			seqPath := filepath.Join(phasePath, seqEntry.Name())
			taskEntries, err := os.ReadDir(seqPath)
			if err != nil {
				continue
			}
			for _, taskEntry := range taskEntries {
				if taskEntry.IsDir() || !strings.HasSuffix(taskEntry.Name(), ".md") {
					continue
				}
				if !hasNumericPrefix(taskEntry.Name()) {
					continue
				}
				taskPath := filepath.Join(seqPath, taskEntry.Name())
				if mgrErr != nil {
					rel, relErr := filepath.Rel(festivalPath, taskPath)
					if relErr == nil {
						return rel
					}
					return taskPath
				}
				store := mgr.Store()
				status := progress.ResolveTaskStatus(store, festivalPath, taskPath)
				if status == progress.StatusCompleted {
					continue
				}
				rel, relErr := filepath.Rel(festivalPath, taskPath)
				if relErr == nil {
					return rel
				}
				return taskPath
			}
		}
	}
	return ""
}

func loadActiveGates(ctx context.Context, festivalPath string) []string {
	if err := ctx.Err(); err != nil {
		return nil
	}

	festivalsRoot, err := tpl.FindFestivalsRoot(festivalPath)
	if err != nil {
		festivalsRoot = filepath.Dir(filepath.Dir(festivalPath))
	}

	merger, err := gatescore.NewConfigMerger(festivalsRoot, nil)
	if err != nil {
		return nil
	}

	entries, err := os.ReadDir(festivalPath)
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var names []string

	for _, entry := range entries {
		if !entry.IsDir() || !hasNumericPrefix(entry.Name()) {
			continue
		}
		phasePath := filepath.Join(festivalPath, entry.Name())
		seqEntries, err := os.ReadDir(phasePath)
		if err != nil {
			continue
		}
		for _, seqEntry := range seqEntries {
			if !seqEntry.IsDir() || !hasNumericPrefix(seqEntry.Name()) {
				continue
			}
			seqPath := filepath.Join(phasePath, seqEntry.Name())
			policy, mergeErr := merger.MergeForSequence(ctx, festivalPath, phasePath, seqPath, gatescore.DefaultMergeOptions())
			if mergeErr != nil {
				continue
			}
			for _, gate := range policy.GetActiveGates() {
				if _, ok := seen[gate.Template]; !ok {
					seen[gate.Template] = struct{}{}
					names = append(names, gate.Template)
				}
			}
		}
	}

	return names
}

func kindWarnings(kind string, festival *show.FestivalInfo) []string {
	if kind != "ritual-template" {
		return nil
	}
	cfg, err := config.LoadFestivalConfig(festival.Path, "")
	if err != nil || cfg == nil || cfg.Metadata.ID == "" {
		return []string{"this is a ritual template, not an execution run; use 'fest ritual run <id>' to create a run"}
	}
	return []string{fmt.Sprintf("this is a ritual template, not an execution run; use 'fest ritual run %s' to create a run", cfg.Metadata.ID)}
}

func hasNumericPrefix(name string) bool {
	if len(name) < 2 {
		return false
	}
	digitCount := 0
	for i := 0; i < len(name) && i < 4; i++ {
		if name[i] == '_' && digitCount > 0 {
			return true
		}
		if name[i] >= '0' && name[i] <= '9' {
			digitCount++
		} else {
			return false
		}
	}
	return false
}

func emitJSON(view *WalkView) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(view); err != nil {
		return errors.Wrap(err, "encoding JSON output")
	}
	return nil
}

func emitStandaloneText(view *WalkView, info *show.StandaloneWorkflowInfo, campaignRoot string) error {
	displayPath := view.Path
	if campaignRoot != "" && strings.HasPrefix(view.Path, campaignRoot) {
		rel, err := filepath.Rel(campaignRoot, view.Path)
		if err == nil {
			displayPath = rel
		}
	}

	fmt.Println(ui.H1("Workflow Walk"))
	fmt.Printf("%s %s\n", ui.Label("Workflow"), ui.Value(view.Name, ui.FestivalColor))
	fmt.Printf("%s %s\n", ui.Label("Kind"), ui.Value(view.Kind))
	fmt.Printf("%s %s\n", ui.Label("Mode"), ui.Value(strings.TrimPrefix(info.Mode, "standalone-")))
	fmt.Printf("%s %s\n", ui.Label("Status"), ui.GetStateStyle(view.Status).Render(view.Status))
	fmt.Printf("%s %s\n", ui.Label("Path"), ui.Dim(displayPath))
	if info.RunID != "" {
		fmt.Printf("%s %s\n", ui.Label("Run"), ui.Value(info.RunID))
	}

	if view.Progress != nil {
		fmt.Println()
		fmt.Println(ui.H2("Progress"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		fmt.Printf("%s %d/%d steps (%d%%)\n",
			ui.Label("Steps"),
			view.Progress.Completed,
			view.Progress.Total,
			view.Progress.Percentage,
		)
	}

	if view.Next != "" {
		fmt.Println()
		fmt.Println(ui.H2("Current"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		fmt.Printf("%s\n", view.Next)
	}

	if len(view.Blocked) > 0 {
		fmt.Println()
		fmt.Println(ui.H2("Blocked"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, b := range view.Blocked {
			if b.Reason != "" {
				fmt.Printf("%s %s %s\n", ui.StateIcon("blocked"), ui.Value(b.Task, ui.TaskColor), ui.Dim(b.Reason))
			} else {
				fmt.Printf("%s %s\n", ui.StateIcon("blocked"), ui.Value(b.Task, ui.TaskColor))
			}
		}
	}

	if len(view.Warnings) > 0 {
		fmt.Println()
		fmt.Println(ui.H2("Warnings"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, w := range view.Warnings {
			fmt.Printf("%s %s\n", ui.StateIcon("blocked"), ui.Warning(w))
		}
	}

	return nil
}

func emitText(view *WalkView, festivalPath, campaignRoot string) error {
	displayPath := festivalPath
	if campaignRoot != "" && strings.HasPrefix(festivalPath, campaignRoot) {
		rel, err := filepath.Rel(campaignRoot, festivalPath)
		if err == nil {
			displayPath = rel
		}
	}

	fmt.Println(ui.H1("Festival Walk"))
	fmt.Printf("%s %s\n", ui.Label("Festival"), ui.Value(view.Name, ui.FestivalColor))
	fmt.Printf("%s %s\n", ui.Label("Kind"), ui.Value(view.Kind))
	fmt.Printf("%s %s\n", ui.Label("Status"), ui.GetStateStyle(view.Status).Render(view.Status))
	fmt.Printf("%s %s\n", ui.Label("Path"), ui.Dim(displayPath))

	if view.Goal != "" {
		fmt.Println()
		fmt.Println(ui.H2("Goal"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		fmt.Printf("%s\n", view.Goal)
	}

	if view.Progress != nil {
		fmt.Println()
		fmt.Println(ui.H2("Progress"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		fmt.Printf("%s %d/%d tasks (%d%%)\n",
			ui.Label("Tasks"),
			view.Progress.Completed,
			view.Progress.Total,
			view.Progress.Percentage,
		)
	}

	if view.Next != "" {
		fmt.Println()
		fmt.Println(ui.H2("Next"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		fmt.Printf("%s\n", view.Next)
	}

	if len(view.Blocked) > 0 {
		fmt.Println()
		fmt.Println(ui.H2("Blocked"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, b := range view.Blocked {
			if b.Reason != "" {
				fmt.Printf("%s %s %s\n", ui.StateIcon("blocked"), ui.Value(b.Task, ui.TaskColor), ui.Dim(b.Reason))
			} else {
				fmt.Printf("%s %s\n", ui.StateIcon("blocked"), ui.Value(b.Task, ui.TaskColor))
			}
		}
	}

	if len(view.Gates) > 0 {
		fmt.Println()
		fmt.Println(ui.H2("Active Gates"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, g := range view.Gates {
			fmt.Printf("  %s\n", g)
		}
	}

	if len(view.Warnings) > 0 {
		fmt.Println()
		fmt.Println(ui.H2("Warnings"))
		fmt.Println(ui.Dim(strings.Repeat("─", 60)))
		for _, w := range view.Warnings {
			fmt.Printf("%s %s\n", ui.StateIcon("blocked"), ui.Warning(w))
		}
	}

	return nil
}
