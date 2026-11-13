package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp/pkg/approvalkit"
	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/fest/internal/guidance"
)

// campNavigator returns a navigator for a phase inside a throwaway camp, with
// workflow persistence injected in memory. The approvals ledger it reports to
// is real, under the temp camp. HOME is redirected so resolving the ledger
// writer id never reads or writes the developer's own config.
func campNavigator(t *testing.T, steps []WorkflowStep) (*Navigator, string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CAMP_ROOT", "")
	camp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(camp, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(camp, ".campaign", "campaign.yaml"), []byte("id: test-camp\nname: test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	festival := filepath.Join(camp, "festivals", "active", "demo-fest-DF0001")
	phase := filepath.Join(festival, "001_REVIEW")
	if err := os.MkdirAll(phase, 0o755); err != nil {
		t.Fatal(err)
	}
	n, err := NewNavigator(&guidance.GuidanceContext{FestivalPath: festival, PhasePath: phase}, guidance.ModeWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	if err = n.BaseNavigator.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	n.steps = steps
	n.festivalPath = festival
	n.phaseName = "001_REVIEW"
	n.workflowState = NewWorkflowState(len(steps))
	n.workflowState.Initialize(steps)
	n.SetStateStore(&notificationStore{})
	// No approval judge configured: blocking checkpoints route to a human.
	n.notificationHooks = func(context.Context, string) (string, error) { return "", nil }
	return n, camp
}

func pendingIn(t *testing.T, camp string) []approvalkit.Pending {
	t.Helper()
	pending, err := approvalkit.Load(context.Background(), camp)
	if err != nil {
		t.Fatal(err)
	}
	return pending
}

func ledgerKinds(t *testing.T, camp string) string {
	t.Helper()
	reader, err := ledgerkit.NewReaderIn(camp, approvalkit.Dir)
	if err != nil {
		t.Fatal(err)
	}
	events, _, err := reader.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, e := range events {
		kinds = append(kinds, string(e.Kind))
	}
	return strings.Join(kinds, ",")
}

func humanGate(number int, name string) WorkflowStep {
	return WorkflowStep{Number: number, Name: name, Goal: "Get sign-off.", Checkpoint: CheckpointUserApproval}
}

func TestReportApprovals_RequestThenApprove(t *testing.T) {
	ctx := context.Background()
	n, camp := campNavigator(t, []WorkflowStep{humanGate(1, "REVIEW"), {Number: 2, Name: "WRAP"}})

	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}
	pending := pendingIn(t, camp)
	if len(pending) != 1 {
		t.Fatalf("pending = %+v, want one request", pending)
	}
	p := pending[0]
	if p.SourceKind != approvalkit.SourceFestivalCheckpoint || p.Step != 1 || p.StepName != "REVIEW" ||
		p.Cwd != "festivals/active/demo-fest-DF0001/001_REVIEW" ||
		p.Doc != "festivals/active/demo-fest-DF0001/001_REVIEW/WORKFLOW.md" ||
		p.Scope.Campaign != "test-camp" || p.Scope.Festival != "DF0001" || p.Scope.Phase != "001_REVIEW" ||
		!strings.Contains(p.Body, "Get sign-off.") {
		t.Fatalf("unexpected request: %+v", p)
	}

	// Persisting again without a change must not open a second request.
	if err := n.saveTransition(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := ledgerKinds(t, camp); got != "approval_requested" {
		t.Fatalf("ledger = %s, want a single request", got)
	}

	if err := n.Approve(ctx); err != nil {
		t.Fatal(err)
	}
	if pending := pendingIn(t, camp); len(pending) != 0 {
		t.Fatalf("pending after approve = %+v", pending)
	}
	if got := ledgerKinds(t, camp); got != "approval_requested,approval_approved" {
		t.Fatalf("ledger = %s", got)
	}
}

func TestReportApprovals_ApproveIntoNextHumanGate(t *testing.T) {
	ctx := context.Background()
	n, camp := campNavigator(t, []WorkflowStep{humanGate(1, "REVIEW"), humanGate(2, "SIGNOFF")})
	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if err := n.Approve(ctx); err != nil {
		t.Fatal(err)
	}
	pending := pendingIn(t, camp)
	if len(pending) != 1 || pending[0].Step != 2 {
		t.Fatalf("pending = %+v, want the step 2 gate", pending)
	}
	if got := ledgerKinds(t, camp); got != "approval_requested,approval_approved,approval_requested" {
		t.Fatalf("ledger = %s", got)
	}
}

func TestReportApprovals_Reject(t *testing.T) {
	ctx := context.Background()
	n, camp := campNavigator(t, []WorkflowStep{humanGate(1, "REVIEW")})
	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if err := n.Reject(ctx, "missing evidence"); err != nil {
		t.Fatal(err)
	}
	// A rejected step waits on a revision, not on the human.
	if pending := pendingIn(t, camp); len(pending) != 0 {
		t.Fatalf("pending after reject = %+v", pending)
	}
	if got := ledgerKinds(t, camp); got != "approval_requested,approval_rejected" {
		t.Fatalf("ledger = %s", got)
	}
}

func TestReportApprovals_SkipWithdraws(t *testing.T) {
	ctx := context.Background()
	n, camp := campNavigator(t, []WorkflowStep{humanGate(1, "REVIEW"), {Number: 2, Name: "WRAP"}})
	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if err := n.SkipCurrentStep(ctx, StepStatusSkipped, "done elsewhere"); err != nil {
		t.Fatal(err)
	}
	if pending := pendingIn(t, camp); len(pending) != 0 {
		t.Fatalf("pending after skip = %+v", pending)
	}
	if got := ledgerKinds(t, camp); got != "approval_requested,approval_withdrawn" {
		t.Fatalf("ledger = %s", got)
	}
}

func TestReportApprovals_JudgeDelegatedIsNotReported(t *testing.T) {
	ctx := context.Background()
	// An artifact review is the class a configured judge may decide.
	step := humanGate(1, "REVIEW")
	step.CheckpointClass = CheckpointClassArtifactReview
	n, camp := campNavigator(t, []WorkflowStep{step})
	n.notificationHooks = func(context.Context, string) (string, error) { return "judge-agent", nil }
	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if pending := pendingIn(t, camp); len(pending) != 0 {
		t.Fatalf("a judge-delegated checkpoint was reported: %+v", pending)
	}
	if _, err := os.Stat(filepath.Join(camp, filepath.FromSlash(approvalkit.Dir))); !os.IsNotExist(err) {
		t.Fatalf("nothing should have been written: %v", err)
	}
}

func TestReportApprovals_HumanRequiredIsReportedDespiteJudge(t *testing.T) {
	ctx := context.Background()
	step := humanGate(1, "ATTEST")
	step.Approval = "human-required"
	n, camp := campNavigator(t, []WorkflowStep{step})
	n.notificationHooks = func(context.Context, string) (string, error) { return "judge-agent", nil }
	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if pending := pendingIn(t, camp); len(pending) != 1 {
		t.Fatalf("a human-required checkpoint must be reported: %+v", pending)
	}
}

func TestReportApprovals_GateIsItsOwnSource(t *testing.T) {
	ctx := context.Background()
	n, camp := campNavigator(t, []WorkflowStep{humanGate(1, "REVIEW")})
	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}

	gate, _ := campNavigator(t, []WorkflowStep{humanGate(1, "GATE")})
	// Point the gate navigator at the same phase of the same camp.
	gate.Ctx = n.Ctx
	gate.festivalPath = n.festivalPath
	gate.SetDocFilename("GATES.md")
	gate.SetStateKeyPrefix("gate:")
	if err := gate.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}

	pending := pendingIn(t, camp)
	if len(pending) != 2 {
		t.Fatalf("a phase's workflow and gate are separate sources, got %+v", pending)
	}
	kinds := map[string]bool{}
	for _, p := range pending {
		kinds[p.SourceKind] = true
	}
	if !kinds[approvalkit.SourceFestivalCheckpoint] || !kinds[approvalkit.SourcePhaseGate] {
		t.Fatalf("kinds = %v", kinds)
	}
}

func TestReportApprovals_FailedSaveReportsNothing(t *testing.T) {
	ctx := context.Background()
	n, camp := campNavigator(t, []WorkflowStep{humanGate(1, "REVIEW")})
	n.store.(*notificationStore).err = os.ErrPermission
	if err := n.saveTransition(ctx, 0); err == nil {
		t.Fatal("expected the save to fail")
	}
	if pending := pendingIn(t, camp); len(pending) != 0 {
		t.Fatalf("a failed save must not report a request: %+v", pending)
	}
}

// TestReportApprovals_FailedSaveDoesNotLeakItsOutcome pins that the reason set
// for a transition that failed to persist is not attributed to a later report.
func TestReportApprovals_FailedSaveDoesNotLeakItsOutcome(t *testing.T) {
	ctx := context.Background()
	n, camp := campNavigator(t, []WorkflowStep{humanGate(1, "REVIEW"), {Number: 2, Name: "WRAP"}})
	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}
	store := n.store.(*notificationStore)
	store.err = os.ErrPermission
	if err := n.Approve(ctx); err == nil {
		t.Fatal("expected the approval to fail to save")
	}
	if n.nextApprovalReport != nil {
		t.Fatal("a failed save left its outcome behind")
	}
	// The same navigator then moves on by another route.
	store.err = nil
	n.reportApprovals(ctx)
	if got := ledgerKinds(t, camp); strings.Contains(got, "approval_approved") {
		t.Fatalf("a failed approval was recorded as approved: %s", got)
	}
}

func TestReportApprovals_OutsideACampIsSilent(t *testing.T) {
	ctx := context.Background()
	t.Setenv("CAMP_ROOT", "")
	n, _ := notificationNavigator(t, []WorkflowStep{humanGate(1, "REVIEW")})
	phase := t.TempDir()
	n.Ctx.PhasePath = phase
	n.festivalPath = phase
	if err := n.saveTransition(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(phase, ".campaign")); !os.IsNotExist(err) {
		t.Fatalf("no camp, so nothing may be created: %v", err)
	}
}
