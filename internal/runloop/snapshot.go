package runloop

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	wfparser "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/workflow/localstore"
	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

// Inspect reads whatever fest next would show, without printing it.
func Inspect(ctx context.Context, cwd string) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	res, err := standalone.Resolve(ctx, cwd)
	if err != nil {
		return Snapshot{}, err
	}
	switch res.Mode {
	case standalone.ModeTracked:
		return inspectStandalone(ctx, res)
	case standalone.ModeAnonymous:
		return Snapshot{}, errors.New("anonymous WORKFLOW.md is not leaveable").
			WithHint("Run 'fest workflow start' or 'fest create workflow' so the run is tracked")
	case standalone.ModeFestival:
		return inspectFestival(ctx, res.FestivalPath, cwd)
	default:
		return Snapshot{}, errors.NotFound("no festival or tracked WORKFLOW.md").
			WithHint("Run from a festival directory or a tracked standalone workflow")
	}
}

func inspectStandalone(ctx context.Context, res *standalone.Result) (Snapshot, error) {
	store := localstore.Open(res.RuntimeDir, res.WorkflowDoc)
	state, err := store.LoadActive(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	if state == nil {
		runs, listErr := store.ListRuns(ctx)
		if listErr != nil {
			return Snapshot{}, listErr
		}
		if n := len(runs); n > 0 && runs[n-1].Status == "completed" {
			last := runs[n-1]
			return Snapshot{
				Kind:              "standalone",
				Complete:          true,
				StandaloneRuntime: res.RuntimeDir,
				StandaloneDoc:     res.WorkflowDoc,
				Path:              res.WorkflowDoc,
				WorkingDir:        filepath.Dir(res.WorkflowDoc),
				RunID:             last.RunID,
			}, nil
		}
		return Snapshot{}, errors.NotFound("no active run").
			WithHint("Run 'fest workflow start' to begin")
	}
	steps, parseErr := wfparser.NewParser().Parse(ctx, res.WorkflowDoc)
	if parseErr != nil {
		return Snapshot{}, errors.Wrap(parseErr, "parsing WORKFLOW.md")
	}
	idx, mode := pickRenderStep(state, len(steps))
	snap := Snapshot{
		Kind:              "standalone",
		Complete:          mode == "complete" || state.Status == "completed",
		Blocked:           state.Blocked,
		Current:           idx,
		Total:             len(steps),
		StandaloneRuntime: res.RuntimeDir,
		StandaloneDoc:     res.WorkflowDoc,
		Path:              res.WorkflowDoc,
		WorkingDir:        filepath.Dir(res.WorkflowDoc),
		RunID:             state.RunID,
	}
	if idx >= 1 && idx <= len(steps) {
		s := steps[idx-1]
		snap.Label = s.Name
		snap.Goal = s.Goal
		snap.Actions = s.Actions
		snap.Checkpoint = string(s.Checkpoint)
		snap.CheckpointClass = string(s.CheckpointClass)
		snap.HumanApproval = strings.EqualFold(strings.TrimSpace(s.Approval), "human-required")
		snap.Content = s.Goal
	}
	return snap, nil
}

func inspectFestival(ctx context.Context, festivalPath, cwd string) (Snapshot, error) {
	selector := selection.NewSelector(festivalPath)
	result, err := selector.FindNext(ctx, cwd)
	if err != nil {
		return Snapshot{}, errors.Wrap(err, "finding next task")
	}
	snap := Snapshot{
		Kind:         "festival",
		Complete:     result.FestivalComplete,
		FestivalPath: festivalPath,
		WorkingDir:   result.WorkingDirAbsolute,
	}
	if result.Task != nil {
		snap.Label = result.Task.Name
		snap.Path = result.Task.Path
		snap.TaskStatus = result.Task.Status
		snap.AutonomyLevel = result.Task.AutonomyLevel
	}
	if result.Progress != nil {
		snap.Current = result.Progress.CompletedTasks
		snap.Total = result.Progress.TotalTasks
	}
	if result.TaskContent != "" {
		snap.Content = result.TaskContent
		snap.Goal = result.TaskContent
	}
	return snap, nil
}

func pickRenderStep(state *localstore.RunState, totalSteps int) (int, string) {
	if totalSteps == 0 {
		return 0, "no_run"
	}
	if state.Status == "completed" || state.CompletedSteps >= totalSteps {
		return totalSteps, "complete"
	}
	if state.CurrentStep > state.CompletedSteps {
		return state.CurrentStep, "in_progress"
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

func LedgerPath(snap Snapshot) (string, error) {
	if snap.Kind == "standalone" && snap.StandaloneRuntime != "" && snap.RunID != "" {
		return filepath.Join(snap.StandaloneRuntime, "runs", snap.RunID, "fest-run.jsonl"), nil
	}
	if snap.Kind == "festival" && snap.FestivalPath != "" {
		return filepath.Join(snap.FestivalPath, ".fest", "runs", "fest-run.jsonl"), nil
	}
	return "", errors.New("cannot resolve fest-run ledger path")
}
