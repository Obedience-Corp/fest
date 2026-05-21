package show

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/ui"
	filewatch "github.com/Obedience-Corp/fest/internal/watch"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

// StandaloneWorkflowInfo is the display-ready view of a standalone WORKFLOW.md.
type StandaloneWorkflowInfo struct {
	Mode           string
	StartDir       string
	WorkflowDoc    string
	RuntimeDir     string
	RunID          string
	RunStatus      string
	CurrentStep    int
	TotalSteps     int
	CompletedSteps int
	Blocked        bool
	DocHashChanged bool
	RenderMode     string
	Steps          []shared.WorkflowStepView
}

// ResolveStandaloneWorkflow resolves and loads a standalone WORKFLOW.md view.
// Festival contexts return nil so callers can continue with festival handling.
func ResolveStandaloneWorkflow(ctx context.Context, startDir string) (*StandaloneWorkflowInfo, error) {
	res, err := standalone.Resolve(ctx, startDir)
	if err != nil {
		return nil, err
	}
	switch res.Mode {
	case standalone.ModeAnonymous, standalone.ModeTracked:
		return loadStandaloneWorkflow(ctx, res)
	default:
		return nil, nil
	}
}

func loadStandaloneWorkflow(ctx context.Context, res *standalone.Result) (*StandaloneWorkflowInfo, error) {
	steps, err := wf.NewParser().Parse(ctx, res.WorkflowDoc)
	if err != nil {
		return nil, errors.Wrap(err, "parsing WORKFLOW.md").WithField("path", res.WorkflowDoc)
	}
	if len(steps) == 0 {
		return nil, errors.Validation("WORKFLOW.md has no parseable steps").
			WithField("path", res.WorkflowDoc).
			WithHint("add at least one '## Step 1: NAME' header")
	}

	mode := "standalone-anonymous"
	runStatus := "not-started"
	var state *localstore.RunState
	if res.Mode == standalone.ModeTracked {
		mode = "standalone-tracked"
		store := localstore.Open(res.RuntimeDir, res.WorkflowDoc)
		var err error
		state, err = store.LoadActive(ctx)
		if err != nil {
			return nil, err
		}
		if state != nil {
			runStatus = state.Status
		}
	}

	views, current, completed, renderMode := buildStandaloneStepViews(steps, state)
	info := &StandaloneWorkflowInfo{
		Mode:           mode,
		StartDir:       res.StartDir,
		WorkflowDoc:    res.WorkflowDoc,
		RuntimeDir:     res.RuntimeDir,
		RunStatus:      runStatus,
		CurrentStep:    current,
		TotalSteps:     len(steps),
		CompletedSteps: completed,
		RenderMode:     renderMode,
		Steps:          views,
	}
	if state != nil {
		info.RunID = state.RunID
		info.Blocked = state.Blocked
		info.DocHashChanged = state.DocHashChanged
	}
	return info, nil
}

func buildStandaloneStepViews(steps []wf.WorkflowStep, state *localstore.RunState) ([]shared.WorkflowStepView, int, int, string) {
	total := len(steps)
	current, renderMode := standaloneRenderPosition(state, total)
	completed := 0
	if state != nil {
		completed = clampStepCount(state.CompletedSteps, total)
		if state.Status == "completed" {
			completed = total
		}
	}

	views := make([]shared.WorkflowStepView, len(steps))
	for i, step := range steps {
		position := i + 1
		status := wf.StepStatusPending
		if position <= completed || renderMode == "complete" {
			status = wf.StepStatusCompleted
		} else if position == current {
			switch {
			case state != nil && state.Blocked:
				status = wf.StepStatusBlocked
			case renderMode == "in_progress":
				status = wf.StepStatusInProgress
			default:
				status = wf.StepStatusPending
			}
		}

		views[i] = shared.WorkflowStepView{
			Number:        step.Number,
			Name:          step.Name,
			Status:        status,
			IsCurrent:     position == current && renderMode != "complete",
			HasCheckpoint: step.HasCheckpoint(),
			Goal:          step.Goal,
		}
	}
	return views, current, completed, renderMode
}

func standaloneRenderPosition(state *localstore.RunState, totalSteps int) (int, string) {
	if totalSteps == 0 {
		return 0, "no_run"
	}
	if state == nil {
		return 1, "next_up"
	}
	if state.Status == "completed" || state.CompletedSteps >= totalSteps {
		return totalSteps, "complete"
	}
	if state.CurrentStep > state.CompletedSteps {
		return clampStepCount(state.CurrentStep, totalSteps), "in_progress"
	}
	next := state.CompletedSteps + 1
	if next > totalSteps {
		return totalSteps, "complete"
	}
	if next < 1 {
		return 1, "next_up"
	}
	return next, "next_up"
}

func clampStepCount(n, total int) int {
	if n < 0 {
		return 0
	}
	if total > 0 && n > total {
		return total
	}
	return n
}

func emitStandaloneWorkflow(info *StandaloneWorkflowInfo, opts *showOptions) error {
	if opts.json {
		return emitStandaloneWorkflowJSON(info)
	}
	if opts.summary {
		fmt.Print(FormatStandaloneWorkflowSummary(info))
		return nil
	}
	fmt.Print(FormatStandaloneWorkflowProgress(info))
	return nil
}

// FormatStandaloneWorkflowSummary renders aggregate standalone workflow state.
func FormatStandaloneWorkflowSummary(info *StandaloneWorkflowInfo) string {
	var sb strings.Builder
	writeStandaloneHeader(&sb, info)
	return sb.String()
}

// FormatStandaloneWorkflowProgress renders standalone workflow state plus steps.
func FormatStandaloneWorkflowProgress(info *StandaloneWorkflowInfo) string {
	var sb strings.Builder
	writeStandaloneHeader(&sb, info)
	sb.WriteString("\n")
	sb.WriteString(shared.RenderWorkflowSteps(info.Steps, false))
	if current := currentStandaloneStep(info); current != nil {
		fmt.Fprintf(&sb, "\nCurrent: Step %d - %s\n", current.Number, current.Name)
	}
	if info.DocHashChanged {
		fmt.Fprintf(&sb, "%s WORKFLOW.md has changed since this run started\n", ui.Warning("!"))
	}
	if info.RenderMode != "complete" {
		if info.Mode == "standalone-anonymous" {
			sb.WriteString("First mutating command: ")
		} else {
			sb.WriteString("Next: ")
		}
		sb.WriteString(ui.Accent("fest workflow advance"))
		sb.WriteString("\n")
	}
	return sb.String()
}

func writeStandaloneHeader(sb *strings.Builder, info *StandaloneWorkflowInfo) {
	sb.WriteString("Workflow Progress\n")
	sb.WriteString("-----------------\n")
	fmt.Fprintf(sb, "Workflow: %s\n", info.WorkflowDoc)
	fmt.Fprintf(sb, "Mode: %s\n", strings.TrimPrefix(info.Mode, "standalone-"))
	if info.RunID != "" {
		fmt.Fprintf(sb, "Run: %s (%s)\n", info.RunID, info.RunStatus)
	} else {
		fmt.Fprintf(sb, "Status: %s\n", info.RunStatus)
	}
	if info.Blocked {
		fmt.Fprintf(sb, "Blocked: true\n")
	}
	fmt.Fprintf(sb, "Progress: %s steps\n", shared.RenderWorkflowProgress(info.CompletedSteps, info.TotalSteps))
}

func currentStandaloneStep(info *StandaloneWorkflowInfo) *shared.WorkflowStepView {
	if info == nil || info.RenderMode == "complete" {
		return nil
	}
	for i := range info.Steps {
		if info.Steps[i].IsCurrent {
			return &info.Steps[i]
		}
	}
	return nil
}

type standaloneWorkflowJSON struct {
	Mode           string                   `json:"mode"`
	WorkflowDoc    string                   `json:"workflow_doc"`
	RuntimeDir     string                   `json:"runtime_dir,omitempty"`
	RunID          string                   `json:"run_id,omitempty"`
	RunStatus      string                   `json:"run_status"`
	CurrentStep    int                      `json:"current_step"`
	TotalSteps     int                      `json:"total_steps"`
	CompletedSteps int                      `json:"completed_steps"`
	Blocked        bool                     `json:"blocked,omitempty"`
	DocHashChanged bool                     `json:"doc_hash_changed,omitempty"`
	Steps          []standaloneWorkflowStep `json:"steps"`
}

type standaloneWorkflowStep struct {
	Number        int    `json:"number"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Current       bool   `json:"current,omitempty"`
	HasCheckpoint bool   `json:"has_checkpoint,omitempty"`
	Goal          string `json:"goal,omitempty"`
}

func emitStandaloneWorkflowJSON(info *StandaloneWorkflowInfo) error {
	out := standaloneWorkflowJSON{
		Mode:           info.Mode,
		WorkflowDoc:    info.WorkflowDoc,
		RuntimeDir:     info.RuntimeDir,
		RunID:          info.RunID,
		RunStatus:      info.RunStatus,
		CurrentStep:    info.CurrentStep,
		TotalSteps:     info.TotalSteps,
		CompletedSteps: info.CompletedSteps,
		Blocked:        info.Blocked,
		DocHashChanged: info.DocHashChanged,
		Steps:          make([]standaloneWorkflowStep, 0, len(info.Steps)),
	}
	for _, step := range info.Steps {
		out.Steps = append(out.Steps, standaloneWorkflowStep{
			Number:        step.Number,
			Name:          step.Name,
			Status:        string(step.Status),
			Current:       step.IsCurrent,
			HasCheckpoint: step.HasCheckpoint,
			Goal:          step.Goal,
		})
	}
	if err := shared.EncodeJSON(os.Stdout, out); err != nil {
		return errors.Wrap(err, "encoding JSON output")
	}
	return nil
}

// WatchStandaloneWorkflow watches and refreshes a standalone WORKFLOW.md view.
func WatchStandaloneWorkflow(ctx context.Context, info *StandaloneWorkflowInfo, opts WatchOptions) error {
	if info == nil {
		return errors.Validation("standalone workflow is required")
	}
	startDir := info.StartDir
	if startDir == "" {
		startDir = filepath.Dir(info.WorkflowDoc)
	}
	return runStandaloneWatchMode(ctx, startDir, opts)
}

func runStandaloneWatchMode(ctx context.Context, startDir string, opts WatchOptions) error {
	render := func() (*StandaloneWorkflowInfo, error) {
		info, err := ResolveStandaloneWorkflow(ctx, startDir)
		if err != nil {
			return nil, err
		}
		if info == nil {
			return nil, errors.NotFound("standalone workflow").WithField("path", startDir)
		}
		return info, nil
	}

	info, err := render()
	if err != nil {
		return err
	}
	clearScreen()
	if err := renderStandaloneWorkflowView(info, opts); err != nil {
		return err
	}
	printWatchFooter(false)

	w, err := filewatch.New(filewatch.Config{
		Paths:    standaloneWatchPaths(info),
		Debounce: 100 * time.Millisecond,
	}, func() {
		refreshed, err := render()
		if err != nil {
			return
		}
		clearScreen()
		if err := renderStandaloneWorkflowView(refreshed, opts); err != nil {
			return
		}
		printWatchFooter(false)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "File watching unavailable (%v), using polling fallback\n", err)
		return runStandalonePollingMode(ctx, startDir, opts)
	}
	defer func() { _ = w.Close() }()

	return w.Watch(ctx)
}

func runStandalonePollingMode(ctx context.Context, startDir string, opts WatchOptions) error {
	clearScreen()
	if err := renderStandaloneWorkflowFromStartDir(ctx, startDir, opts); err != nil {
		return err
	}
	printWatchFooter(true)

	ticker := time.NewTicker(pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println()
			return nil
		case <-ticker.C:
			clearScreen()
			if err := renderStandaloneWorkflowFromStartDir(ctx, startDir, opts); err != nil {
				return err
			}
			printWatchFooter(true)
		}
	}
}

func renderStandaloneWorkflowFromStartDir(ctx context.Context, startDir string, opts WatchOptions) error {
	info, err := ResolveStandaloneWorkflow(ctx, startDir)
	if err != nil {
		return err
	}
	if info == nil {
		return errors.NotFound("standalone workflow").WithField("path", startDir)
	}
	return renderStandaloneWorkflowView(info, opts)
}

func renderStandaloneWorkflowView(info *StandaloneWorkflowInfo, opts WatchOptions) error {
	showOpts := &showOptions{summary: opts.Summary}
	return emitStandaloneWorkflow(info, showOpts)
}

func standaloneWatchPaths(info *StandaloneWorkflowInfo) []string {
	seen := map[string]bool{}
	paths := []string{}
	add := func(path string) {
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}

	add(filepath.Dir(info.WorkflowDoc))
	add(info.WorkflowDoc)
	add(info.RuntimeDir)
	if info.RuntimeDir != "" {
		add(filepath.Join(info.RuntimeDir, "runs"))
	}
	if info.RuntimeDir != "" && info.RunID != "" {
		add(filepath.Join(info.RuntimeDir, "runs", info.RunID))
	}
	return paths
}
