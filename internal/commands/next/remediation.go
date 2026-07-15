package next

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/frontmatter"
	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/guidance/selection"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
	"github.com/Obedience-Corp/fest/internal/progress"
)

const gateStateKeyPrefix = "gate:"

type failedGate struct {
	PhasePath        string
	PhaseName        string
	Step             int
	StepName         string
	Reason           string
	RemediationPhase string
}

func loadProgressStore(ctx context.Context, festivalPath string) (*progress.Store, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		return nil, err
	}
	return store, nil
}

func findFailedRemediationGate(ctx context.Context, festivalPath string) (*failedGate, error) {
	store, err := loadProgressStore(ctx, festivalPath)
	if err != nil {
		return nil, err
	}
	return findFailedRemediationGateInStore(ctx, festivalPath, store)
}

func findFailedRemediationGateInStore(ctx context.Context, festivalPath string, store *progress.Store) (*failedGate, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	state := store.WorkflowState()
	if state == nil {
		return nil, nil
	}

	type candidate struct {
		phaseName string
		stepNum   int
		stepState *wf.StepState
	}
	var candidates []candidate
	for stateKey, phaseState := range state.Phases {
		if !strings.HasPrefix(stateKey, gateStateKeyPrefix) {
			continue
		}
		if phaseState == nil {
			continue
		}
		phaseName := strings.TrimPrefix(stateKey, gateStateKeyPrefix)
		for stepNum, ss := range phaseState.Steps {
			if ss != nil && ss.Status == wf.StepStatusFailedRemediation {
				candidates = append(candidates, candidate{phaseName: phaseName, stepNum: stepNum, stepState: ss})
			}
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].phaseName != candidates[j].phaseName {
			return candidates[i].phaseName < candidates[j].phaseName
		}
		return candidates[i].stepNum < candidates[j].stepNum
	})
	c := candidates[0]

	phasePath := filepath.Join(festivalPath, c.phaseName)
	gate := &failedGate{
		PhasePath:        phasePath,
		PhaseName:        c.phaseName,
		Step:             c.stepNum,
		Reason:           wf.DisplayFeedback(c.stepState.Feedback),
		RemediationPhase: c.stepState.RemediationPhase,
	}
	if name, err := lookupGateStepName(ctx, phasePath, c.stepNum); err == nil {
		gate.StepName = name
	}
	return gate, nil
}

func lookupGateStepName(ctx context.Context, phasePath string, stepNum int) (string, error) {
	gatesPath := filepath.Join(phasePath, "GATES.md")
	if _, err := os.Stat(gatesPath); err != nil {
		return "", err
	}
	steps, err := wf.NewParser().Parse(ctx, gatesPath)
	if err != nil {
		return "", err
	}
	if stepNum < 1 || stepNum > len(steps) {
		return "", festerrors.NotFound("gate step")
	}
	return steps[stepNum-1].Name, nil
}

// isRemediationPhaseComplete reports whether the remediation phase has all
// task-based and workflow work complete. A remediation phase is driven by its
// WORKFLOW.md and/or numbered sequences; it is validated to contain at least
// one of these at link time (see validateRemediationPhase). A GATES.md on the
// remediation phase is not part of the remediation loop: the original failed
// gate is the verification step and is rechecked once remediation work is done.
func isRemediationPhaseComplete(ctx context.Context, festivalPath, remediationPhase string) (bool, error) {
	store, err := loadProgressStore(ctx, festivalPath)
	if err != nil {
		return false, err
	}
	phasePath, err := resolveRemediationPhasePath(festivalPath, &failedGate{RemediationPhase: remediationPhase})
	if err != nil {
		return false, err
	}
	return isRemediationPhaseCompleteWithStore(ctx, store, phasePath, remediationPhase)
}

func isRemediationPhaseCompleteWithStore(ctx context.Context, store *progress.Store, phasePath, remediationPhase string) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if remediationPhase == "" {
		return false, nil
	}
	return shared.ArePhaseTasksAndWorkflowComplete(ctx, store != nil, store, phasePath, remediationPhase), nil
}

// routeFailedRemediationGate produces the appropriate `fest next` output for
// an outstanding failed gate. If the remediation phase is incomplete, it
// renders into the remediation phase (workflow or task). If complete, it
// auto-rechecks the gate (transitions the step out of failed_remediation)
// and renders the gate step for re-evaluation. Returns (true, nil) when it
// fully handled the request. opts controls output formatting so machine-output
// modes are honored while remediation is active.
func routeFailedRemediationGate(ctx context.Context, festivalPath string, store *progress.Store, gate *failedGate, opts RenderOptions) (bool, error) {
	if gate == nil {
		return false, nil
	}
	if store == nil {
		var err error
		store, err = loadProgressStore(ctx, festivalPath)
		if err != nil {
			return true, err
		}
	}

	remPath, err := resolveRemediationPhasePath(festivalPath, gate)
	if err != nil {
		return true, err
	}

	complete, err := isRemediationPhaseCompleteWithStore(ctx, store, remPath, gate.RemediationPhase)
	if err != nil {
		return true, err
	}

	if complete {
		return true, renderRecheckRoute(ctx, festivalPath, store, gate, opts)
	}
	return true, renderRemediationRoute(ctx, festivalPath, store, gate, remPath, opts)
}

func resolveRemediationPhasePath(festivalPath string, gate *failedGate) (string, error) {
	if gate == nil || gate.RemediationPhase == "" {
		return "", festerrors.Validation("failed gate is missing a remediation phase").
			WithHint("Reject the gate again with --remediation-phase, or clear the failed-remediation state with 'fest workflow reject --reason \"...\"'")
	}

	remPath := filepath.Join(festivalPath, gate.RemediationPhase)
	info, err := os.Stat(remPath)
	if err != nil || !info.IsDir() {
		return "", festerrors.NotFound("remediation phase directory").
			WithField("phase", gate.RemediationPhase).
			WithField("path", remPath).
			WithHint(fmt.Sprintf("The failed gate still references %s, but that phase no longer exists. Restore or rename the phase, or clear the failed-remediation state with 'fest workflow reject --phase %s --reason \"...\"'.", gate.RemediationPhase, gate.PhaseName))
	}
	return remPath, nil
}

func renderRemediationRoute(ctx context.Context, festivalPath string, store *progress.Store, gate *failedGate, remPath string, opts RenderOptions) error {
	useWorkflow := shouldRouteToRemediationWorkflowWithStore(ctx, store, remPath, gate.RemediationPhase)

	if opts.isMachineOutput() {
		view, err := remediationRouteView(ctx, festivalPath, remPath, gate, useWorkflow)
		if err != nil {
			return err
		}
		return emitRemediationView(view, opts)
	}

	printFailedGateBanner(gate)
	if useWorkflow {
		return runWorkflowModeWithStore(ctx, festivalPath, remPath, store)
	}
	return renderFirstIncompleteTask(ctx, festivalPath, remPath, opts)
}

// shouldRouteToRemediationWorkflow reports whether `fest next` should render the
// remediation phase's WORKFLOW.md step rather than its next task. A WORKFLOW.md
// is surfaced only while it still has incomplete steps: a completed workflow
// always falls through to task routing so a remediation phase with a finished
// workflow but remaining task work does not dead-end on "WORKFLOW COMPLETE". An
// incomplete workflow is shown immediately when it is the phase's only work or
// runs before tasks (position=before); an after-position workflow waits until
// the phase's tasks are complete, matching normal phase routing.
func shouldRouteToRemediationWorkflow(ctx context.Context, festivalPath, remPath, remediationPhase string) bool {
	store, err := loadProgressStore(ctx, festivalPath)
	if err != nil {
		return true
	}
	return shouldRouteToRemediationWorkflowWithStore(ctx, store, remPath, remediationPhase)
}

func shouldRouteToRemediationWorkflowWithStore(ctx context.Context, store *progress.Store, remPath, remediationPhase string) bool {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(remPath, "WORKFLOW.md")); err != nil {
		return false
	}

	if store == nil {
		return true
	}

	state, ok := store.WorkflowPhaseState(remediationPhase)
	if ok && state.TotalSteps > 0 && state.IsComplete() {
		return false
	}

	if shared.WorkflowPositionForPhase(remPath) == frontmatter.WorkflowPositionBefore {
		return true
	}
	if !shared.HasSequenceDirs(remPath) {
		return true
	}
	return shared.ArePhaseTasksComplete(true, store, remPath, remediationPhase)
}

func renderFirstIncompleteTask(ctx context.Context, festivalPath, phasePath string, opts RenderOptions) error {
	selector := selection.NewSelector(festivalPath)
	result, err := selector.FindNext(ctx, phasePath)
	if err != nil {
		return festerrors.Wrap(err, "finding next remediation task")
	}
	if opts.Verbose {
		fmt.Print(selection.FormatVerbose(result, opts.showInlineContext()))
		return nil
	}
	fmt.Print(selection.FormatText(result, opts.showInlineContext()))
	return nil
}

func renderRecheckRoute(ctx context.Context, festivalPath string, store *progress.Store, gate *failedGate, opts RenderOptions) error {
	if opts.isMachineOutput() {
		return emitRemediationView(recheckRouteView(gate), opts)
	}

	// Human `fest next` is the explicit operator action that re-enters the
	// failed gate. Pipe-oriented modes must remain read-only.
	nav, err := newRecheckNavigator(ctx, festivalPath, store, gate)
	if err != nil {
		return err
	}
	if err := nav.Recheck(ctx); err != nil {
		return festerrors.Wrap(err, "recording gate recheck event")
	}

	printRecheckBanner(gate)
	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		return festerrors.Wrap(err, "formatting gate instructions for recheck")
	}
	fmt.Print(instructions)
	fmt.Println("\nRun 'fest workflow approve' to approve, or 'fest workflow reject' to record another failure.")
	return nil
}

// newRecheckNavigator builds the GATES.md-configured workflow navigator used to
// record the recheck transition and render the failed gate step.
func newRecheckNavigator(ctx context.Context, festivalPath string, store *progress.Store, gate *failedGate) (*wf.Navigator, error) {
	gctx := &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		FestivalName: filepath.Base(festivalPath),
		PhasePath:    gate.PhasePath,
		PhaseName:    gate.PhaseName,
		PhaseType:    guidance.DetectPhaseType(gate.PhasePath),
		Mode:         guidance.ModeWorkflow,
		Config:       guidance.DefaultConfig(),
	}
	nav, err := guidance.NewNavigator(ctx, gctx)
	if err != nil {
		return nil, festerrors.Wrap(err, "creating gate navigator for recheck")
	}
	wfNav, ok := nav.(*wf.Navigator)
	if !ok {
		return nil, festerrors.New("gate navigator is not a workflow navigator")
	}
	wfNav.SetDocFilename("GATES.md")
	wfNav.SetStateKeyPrefix(gateStateKeyPrefix)

	if store == nil {
		var err error
		store, err = loadProgressStore(ctx, festivalPath)
		if err != nil {
			return nil, festerrors.Wrap(err, "loading progress store for recheck")
		}
	}
	wfNav.SetStateStore(store)

	if err := wfNav.Initialize(ctx); err != nil {
		return nil, festerrors.Wrap(err, "initializing gate navigator for recheck")
	}
	return wfNav, nil
}

// remediationView is the machine-readable description of an active failed-gate
// remediation route. Paths in TargetDoc are relative to the festival root for
// piping; TargetDir is absolute for shell `cd`.
type remediationView struct {
	Mode             string `json:"mode"`
	Route            string `json:"route"`
	Complete         bool   `json:"remediation_complete"`
	FailedGatePhase  string `json:"failed_gate_phase"`
	FailedGateStep   int    `json:"failed_gate_step"`
	FailedGateName   string `json:"failed_gate_step_name,omitempty"`
	RemediationPhase string `json:"remediation_phase"`
	Reason           string `json:"reason,omitempty"`
	TargetDoc        string `json:"target_doc,omitempty"`
	TargetDir        string `json:"target_dir,omitempty"`
}

func remediationRouteView(ctx context.Context, festivalPath, remPath string, gate *failedGate, useWorkflow bool) (remediationView, error) {
	v := remediationView{
		Mode:             "failed-gate-remediation",
		FailedGatePhase:  gate.PhaseName,
		FailedGateStep:   gate.Step,
		FailedGateName:   gate.StepName,
		RemediationPhase: gate.RemediationPhase,
		Reason:           gate.Reason,
		TargetDir:        remPath,
	}
	if useWorkflow {
		v.Route = "remediation-workflow"
		v.TargetDoc = filepath.Join(gate.RemediationPhase, "WORKFLOW.md")
		return v, nil
	}

	v.Route = "remediation-task"
	selector := selection.NewSelector(festivalPath)
	result, err := selector.FindNext(ctx, remPath)
	if err != nil {
		return v, festerrors.Wrap(err, "finding next remediation task")
	}
	if result != nil && result.Task != nil {
		v.TargetDoc = filepath.Join(result.Task.PhaseName, result.Task.SequenceName, result.Task.Name+".md")
		v.TargetDir = filepath.Join(festivalPath, result.Task.PhaseName, result.Task.SequenceName)
	}
	return v, nil
}

func recheckRouteView(gate *failedGate) remediationView {
	return remediationView{
		Mode:             "failed-gate-remediation",
		Route:            "recheck-gate",
		Complete:         true,
		FailedGatePhase:  gate.PhaseName,
		FailedGateStep:   gate.Step,
		FailedGateName:   gate.StepName,
		RemediationPhase: gate.RemediationPhase,
		Reason:           gate.Reason,
		TargetDoc:        filepath.Join(gate.PhaseName, "GATES.md"),
		TargetDir:        gate.PhasePath,
	}
}

func emitRemediationView(v remediationView, opts RenderOptions) error {
	switch {
	case opts.Path:
		if v.TargetDoc == "" {
			return festerrors.NotFound("no remediation target document")
		}
		fmt.Println(v.TargetDoc)
	case opts.CD, opts.ProjectDir:
		if v.TargetDir == "" {
			return festerrors.NotFound("no remediation target directory")
		}
		fmt.Println(v.TargetDir)
	case opts.Short:
		fmt.Println(remediationShortLine(v))
	case opts.JSON:
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return festerrors.Parse("formatting remediation JSON", err)
		}
		fmt.Println(string(data))
	}
	return nil
}

func remediationShortLine(v remediationView) string {
	if v.Complete {
		return fmt.Sprintf("remediation complete: recheck gate %s step %d", v.FailedGatePhase, v.FailedGateStep)
	}
	return fmt.Sprintf("remediation active: %s (gate %s step %d failed)", v.RemediationPhase, v.FailedGatePhase, v.FailedGateStep)
}

func printFailedGateBanner(gate *failedGate) {
	fmt.Println("FAILED GATE. REMEDIATION ACTIVE.")
	fmt.Println("────────────────────────────────")
	fmt.Printf("Previous gate failed:\n  %s / step %d", gate.PhaseName, gate.Step)
	if gate.StepName != "" {
		fmt.Printf(" (%s)", gate.StepName)
	}
	fmt.Println()
	if gate.Reason != "" {
		fmt.Printf("Reason: %s\n", gate.Reason)
	}
	fmt.Printf("Remediation phase:\n  %s\n\n", gate.RemediationPhase)
}

func printRecheckBanner(gate *failedGate) {
	fmt.Println("REMEDIATION COMPLETE. RECHECK GATE.")
	fmt.Println("───────────────────────────────────")
	fmt.Printf("Previously failed gate:\n  %s / step %d", gate.PhaseName, gate.Step)
	if gate.StepName != "" {
		fmt.Printf(" (%s)", gate.StepName)
	}
	fmt.Println()
	if gate.Reason != "" {
		fmt.Printf("Original reason: %s\n", gate.Reason)
	}
	fmt.Printf("Remediation phase: %s (complete)\n\n", gate.RemediationPhase)
}
