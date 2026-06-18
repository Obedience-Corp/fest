package show

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	"github.com/Obedience-Corp/fest/internal/errors"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/ui"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

const (
	standaloneModeAnonymous = "standalone-anonymous"
	standaloneModeTracked   = "standalone-tracked"

	renderModeNoRun      = "no_run"
	renderModeNextUp     = "next_up"
	renderModeInProgress = "in_progress"
	renderModeComplete   = "complete"
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

	mode := standaloneModeAnonymous
	runStatus := "not-started"
	var state *localstore.RunState
	if res.Mode == standalone.ModeTracked {
		mode = standaloneModeTracked
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
		if position <= completed || renderMode == renderModeComplete {
			status = wf.StepStatusCompleted
		} else if position == current {
			switch {
			case state != nil && state.Blocked:
				status = wf.StepStatusBlocked
			case renderMode == renderModeInProgress:
				status = wf.StepStatusInProgress
			default:
				status = wf.StepStatusPending
			}
		}

		views[i] = shared.WorkflowStepView{
			Number:        step.Number,
			Name:          step.Name,
			Status:        status,
			IsCurrent:     position == current && renderMode != renderModeComplete,
			HasCheckpoint: step.HasCheckpoint(),
			Goal:          step.Goal,
		}
	}
	return views, current, completed, renderMode
}

func standaloneRenderPosition(state *localstore.RunState, totalSteps int) (int, string) {
	if totalSteps == 0 {
		return 0, renderModeNoRun
	}
	if state == nil {
		return 1, renderModeNextUp
	}
	if state.Status == "completed" || state.CompletedSteps >= totalSteps {
		return totalSteps, renderModeComplete
	}
	if state.CurrentStep > state.CompletedSteps {
		return clampStepCount(state.CurrentStep, totalSteps), renderModeInProgress
	}
	next := state.CompletedSteps + 1
	if next > totalSteps {
		return totalSteps, renderModeComplete
	}
	if next < 1 {
		return 1, renderModeNextUp
	}
	return next, renderModeNextUp
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
	renderOpts := standaloneRenderOptionsFromShow(opts)
	if opts != nil && opts.json {
		return emitStandaloneWorkflowJSON(info)
	}
	if renderOpts.Summary {
		fmt.Print(FormatStandaloneWorkflowSummary(info))
		return nil
	}
	fmt.Print(formatStandaloneWorkflowProgress(info, renderOpts))
	return nil
}

func runShowStandaloneCurrent(ctx context.Context, cwd string, opts *showOptions) (bool, error) {
	standaloneWorkflow, err := ResolveStandaloneWorkflow(ctx, cwd)
	if err != nil {
		return false, err
	}
	if standaloneWorkflow == nil {
		return false, nil
	}
	if opts != nil && opts.watch {
		return true, WatchStandaloneWorkflow(ctx, standaloneWorkflow, WatchOptions{
			Summary:    opts.summary,
			Goals:      opts.goals,
			Collapsed:  opts.collapsed,
			InProgress: opts.inProgress,
		})
	}
	return true, emitStandaloneWorkflow(standaloneWorkflow, opts)
}

type standaloneRenderOptions struct {
	Summary    bool
	ShowGoals  bool
	Collapsed  bool
	InProgress bool
}

func standaloneRenderOptionsFromShow(opts *showOptions) standaloneRenderOptions {
	if opts == nil {
		return standaloneRenderOptions{}
	}
	return standaloneRenderOptions{
		Summary:    opts.summary,
		ShowGoals:  opts.goals,
		Collapsed:  opts.collapsed,
		InProgress: opts.inProgress,
	}
}

func standaloneRenderOptionsFromWatch(opts WatchOptions) standaloneRenderOptions {
	return standaloneRenderOptions{
		Summary:    opts.Summary,
		ShowGoals:  opts.Goals,
		Collapsed:  opts.Collapsed,
		InProgress: opts.InProgress,
	}
}

// FormatStandaloneWorkflowSummary renders aggregate standalone workflow state.
func FormatStandaloneWorkflowSummary(info *StandaloneWorkflowInfo) string {
	var sb strings.Builder
	writeStandaloneHeader(&sb, info)
	return sb.String()
}

// FormatStandaloneWorkflowProgress renders standalone workflow state plus steps.
func FormatStandaloneWorkflowProgress(info *StandaloneWorkflowInfo) string {
	return formatStandaloneWorkflowProgress(info, standaloneRenderOptions{})
}

func formatStandaloneWorkflowProgress(info *StandaloneWorkflowInfo, opts standaloneRenderOptions) string {
	var sb strings.Builder
	writeStandaloneHeader(&sb, info)
	current := currentStandaloneStep(info)
	if opts.Collapsed {
		if current != nil {
			fmt.Fprintf(&sb, "Current: Step %d - %s\n", current.Number, current.Name)
		}
		return sb.String()
	}

	sb.WriteString("\n")
	sb.WriteString(renderStandaloneSteps(info.Steps, opts))
	if current != nil {
		fmt.Fprintf(&sb, "\nCurrent: Step %d - %s\n", current.Number, current.Name)
	}
	if info.DocHashChanged {
		fmt.Fprintf(&sb, "%s WORKFLOW.md has changed since this run started\n", ui.Warning("!"))
	}
	if info.RenderMode != renderModeComplete {
		if info.Mode == standaloneModeAnonymous {
			sb.WriteString("First mutating command: ")
		} else {
			sb.WriteString("Next: ")
		}
		sb.WriteString(ui.Accent("fest workflow advance"))
		sb.WriteString("\n")
	}
	return sb.String()
}

func renderStandaloneSteps(steps []shared.WorkflowStepView, opts standaloneRenderOptions) string {
	filtered := steps
	if opts.InProgress {
		filtered = make([]shared.WorkflowStepView, 0, len(steps))
		for _, step := range steps {
			if step.Status == wf.StepStatusCompleted || step.Status == wf.StepStatusSkipped {
				continue
			}
			filtered = append(filtered, step)
		}
	}
	return shared.RenderWorkflowSteps(filtered, !opts.ShowGoals)
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
	if info == nil || info.RenderMode == renderModeComplete {
		return nil
	}
	for i := range info.Steps {
		if info.Steps[i].IsCurrent {
			return &info.Steps[i]
		}
	}
	return nil
}

// StandaloneWorkflowJSON is the machine-readable view of a standalone WORKFLOW.md.
type StandaloneWorkflowJSON struct {
	Mode           string                       `json:"mode"`
	WorkflowDoc    string                       `json:"workflow_doc"`
	RuntimeDir     string                       `json:"runtime_dir,omitempty"`
	RunID          string                       `json:"run_id,omitempty"`
	RunStatus      string                       `json:"run_status"`
	CurrentStep    int                          `json:"current_step"`
	TotalSteps     int                          `json:"total_steps"`
	CompletedSteps int                          `json:"completed_steps"`
	Blocked        bool                         `json:"blocked,omitempty"`
	DocHashChanged bool                         `json:"doc_hash_changed,omitempty"`
	Steps          []StandaloneWorkflowStepJSON `json:"steps"`
}

// StandaloneWorkflowStepJSON is a single step in a standalone workflow view.
type StandaloneWorkflowStepJSON struct {
	Number        int    `json:"number"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Current       bool   `json:"current,omitempty"`
	HasCheckpoint bool   `json:"has_checkpoint,omitempty"`
	Goal          string `json:"goal,omitempty"`
}

// NewStandaloneWorkflowJSON projects resolved info into the JSON view.
func NewStandaloneWorkflowJSON(info *StandaloneWorkflowInfo) StandaloneWorkflowJSON {
	out := StandaloneWorkflowJSON{
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
		Steps:          make([]StandaloneWorkflowStepJSON, 0, len(info.Steps)),
	}
	for _, step := range info.Steps {
		out.Steps = append(out.Steps, StandaloneWorkflowStepJSON{
			Number:        step.Number,
			Name:          step.Name,
			Status:        string(step.Status),
			Current:       step.IsCurrent,
			HasCheckpoint: step.HasCheckpoint,
			Goal:          step.Goal,
		})
	}
	return out
}

func emitStandaloneWorkflowJSON(info *StandaloneWorkflowInfo) error {
	if err := shared.EncodeJSON(os.Stdout, NewStandaloneWorkflowJSON(info)); err != nil {
		return errors.Wrap(err, "encoding JSON output")
	}
	return nil
}
