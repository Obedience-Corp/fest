package workflow

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp/pkg/approvalkit"
	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/fest/internal/workflow/standalone"
)

// newStandaloneRunInCamp is newStandaloneRun inside a throwaway camp, so the
// run's checkpoint requests are reported to that camp's approvals ledger.
func newStandaloneRunInCamp(t *testing.T, doc string) (*standalone.Result, string) {
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
	dir := filepath.Join(camp, "workflow", "explore", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "WORKFLOW.md")
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatal(err)
	}
	return &standalone.Result{Mode: standalone.ModeAnonymous, StartDir: dir, WorkflowDoc: path}, camp
}

func campPending(t *testing.T, camp string) []approvalkit.Pending {
	t.Helper()
	pending, err := approvalkit.Load(context.Background(), camp)
	if err != nil {
		t.Fatal(err)
	}
	return pending
}

func campLedgerKinds(t *testing.T, camp string) string {
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

func TestStandaloneCheckpoint_ReportsToCampApprovals(t *testing.T) {
	ctx := context.Background()
	res, camp := newStandaloneRunInCamp(t, checkpointWorkflowDoc)

	if err := runAdvanceBootstrap(ctx, res); err != nil {
		t.Fatal(err)
	}
	if pending := campPending(t, camp); len(pending) != 0 {
		t.Fatalf("a step without a checkpoint must not be reported: %+v", pending)
	}

	tr := tracked(res)
	if err := runAdvanceTracked(ctx, tr); err != nil {
		t.Fatal(err)
	}
	pending := campPending(t, camp)
	if len(pending) != 1 {
		t.Fatalf("pending = %+v, want the step 2 checkpoint", pending)
	}
	p := pending[0]
	if p.SourceKind != approvalkit.SourceStandaloneCheckpoint || p.Step != 2 || p.StepName != "REVIEW" ||
		p.Cwd != "workflow/explore/demo" || p.Doc != "workflow/explore/demo/WORKFLOW.md" ||
		p.Title != "demo: REVIEW" || p.Scope.Campaign != "test-camp" {
		t.Fatalf("unexpected request: %+v", p)
	}

	// Advancing again at the checkpoint must not open a second request.
	if err := runAdvanceTracked(ctx, tr); err != nil {
		t.Fatal(err)
	}
	if got := campLedgerKinds(t, camp); got != "approval_requested" {
		t.Fatalf("ledger = %s, want a single request", got)
	}

	// Reject, re-submit, approve.
	if err := runStandaloneReject(ctx, tr, "missing evidence", 0); err != nil {
		t.Fatal(err)
	}
	if pending := campPending(t, camp); len(pending) != 0 {
		t.Fatalf("pending after reject = %+v", pending)
	}
	if err := runAdvanceTracked(ctx, tr); err != nil {
		t.Fatal(err)
	}
	if pending := campPending(t, camp); len(pending) != 1 {
		t.Fatalf("pending after re-submit = %+v", pending)
	}
	operator(t, true, operatorApproveToken)
	if err := runStandaloneApprove(ctx, tr, standaloneApproveOptions{Summary: "ok"}); err != nil {
		t.Fatal(err)
	}
	if pending := campPending(t, camp); len(pending) != 0 {
		t.Fatalf("pending after approve = %+v", pending)
	}
	want := "approval_requested,approval_rejected,approval_requested,approval_approved"
	if got := campLedgerKinds(t, camp); got != want {
		t.Fatalf("ledger = %s, want %s", got, want)
	}
}

func TestStandaloneCheckpoint_OutsideACampWritesNoLedger(t *testing.T) {
	t.Setenv("CAMP_ROOT", "")
	tr := toCheckpoint(t)
	if _, err := os.Stat(filepath.Join(tr.StartDir, ".campaign")); !os.IsNotExist(err) {
		t.Fatalf("no camp, so nothing may be created: %v", err)
	}
}
