package next

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Obedience-Corp/fest/internal/commands/shared"
	festerrors "github.com/Obedience-Corp/fest/internal/errors"
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

func findFailedRemediationGate(ctx context.Context, festivalPath string) (*failedGate, error) {
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
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
		Reason:           c.stepState.Feedback,
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
	if remediationPhase == "" {
		return false, nil
	}
	phasePath := filepath.Join(festivalPath, remediationPhase)
	if _, err := os.Stat(phasePath); err != nil {
		return false, nil
	}

	store := progress.NewStore(festivalPath)
	storeLoaded := store.Load(ctx) == nil
	return shared.ArePhaseTasksAndWorkflowComplete(ctx, storeLoaded, store, phasePath, remediationPhase), nil
}

// routeFailedRemediationGate produces the appropriate `fest next` output for
// an outstanding failed gate. If the remediation phase is incomplete, it
// renders into the remediation phase (workflow or task). If complete, it
// auto-rechecks the gate (transitions the step out of failed_remediation)
// and renders the gate step for re-evaluation. Returns (true, nil) when it
// fully handled the request.
func routeFailedRemediationGate(ctx context.Context, festivalPath string, gate *failedGate) (bool, error) {
	if gate == nil {
		return false, nil
	}

	complete, err := isRemediationPhaseComplete(ctx, festivalPath, gate.RemediationPhase)
	if err != nil {
		return false, err
	}

	if !complete {
		return true, renderRemediationRoute(ctx, festivalPath, gate)
	}
	return true, renderRecheckRoute(ctx, festivalPath, gate)
}

func renderRemediationRoute(ctx context.Context, festivalPath string, gate *failedGate) error {
	remPath := filepath.Join(festivalPath, gate.RemediationPhase)
	if _, err := os.Stat(remPath); err != nil {
		return festerrors.NotFound("remediation phase directory").
			WithField("phase", gate.RemediationPhase).
			WithField("path", remPath)
	}

	printFailedGateBanner(gate)

	if _, err := os.Stat(filepath.Join(remPath, "WORKFLOW.md")); err == nil {
		return runWorkflowMode(ctx, festivalPath, remPath)
	}
	return renderFirstIncompleteTask(ctx, festivalPath, remPath)
}

func renderFirstIncompleteTask(ctx context.Context, festivalPath, phasePath string) error {
	selector := selection.NewSelector(festivalPath)
	result, err := selector.FindNext(ctx, phasePath)
	if err != nil {
		return festerrors.Wrap(err, "finding next remediation task")
	}
	fmt.Print(selection.FormatText(result, true))
	return nil
}

func renderRecheckRoute(ctx context.Context, festivalPath string, gate *failedGate) error {
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
		return festerrors.Wrap(err, "creating gate navigator for recheck")
	}
	if wfNav, ok := nav.(*wf.Navigator); ok {
		wfNav.SetDocFilename("GATES.md")
		wfNav.SetStateKeyPrefix(gateStateKeyPrefix)
	}
	store := progress.NewStore(festivalPath)
	if err := store.Load(ctx); err != nil {
		return festerrors.Wrap(err, "loading progress store for recheck")
	}
	if wfNav, ok := nav.(*wf.Navigator); ok {
		wfNav.SetStateStore(store)
	}
	if err := nav.Initialize(ctx); err != nil {
		return festerrors.Wrap(err, "initializing gate navigator for recheck")
	}
	if wfNav, ok := nav.(*wf.Navigator); ok {
		if err := wfNav.Recheck(ctx); err != nil {
			return festerrors.Wrap(err, "recording gate recheck event")
		}
	}

	instructions, err := nav.FormatInstructions(ctx)
	if err != nil {
		return festerrors.Wrap(err, "formatting gate instructions for recheck")
	}
	fmt.Print(instructions)
	fmt.Println("\nRun 'fest workflow approve' to approve, or 'fest workflow reject' to record another failure.")
	return nil
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
