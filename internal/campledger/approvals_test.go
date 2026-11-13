package campledger

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Obedience-Corp/camp/pkg/approvalkit"
)

func tempCamp(t *testing.T) string {
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
	return camp
}

func TestSetWaiting_RecordsAndResolves(t *testing.T) {
	ctx := context.Background()
	camp := tempCamp(t)
	dir := filepath.Join(camp, "workflow", "explore", "demo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var warned []error
	warn := func(err error) { warned = append(warned, err) }

	w := &Waiting{
		Kind: approvalkit.SourceStandaloneCheckpoint, Step: 2, StepName: "REVIEW",
		Title: "demo: REVIEW", Body: "body", Dir: dir, Doc: filepath.Join(dir, "WORKFLOW.md"),
	}
	SetWaiting(ctx, dir, "", w, approvalkit.OutcomeWithdrawn, "", warn)

	pending, err := approvalkit.Load(ctx, camp)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 || pending[0].Cwd != "workflow/explore/demo" || pending[0].Source != "workflow/explore/demo" ||
		pending[0].Doc != "workflow/explore/demo/WORKFLOW.md" || pending[0].Scope.Campaign != "test-camp" {
		t.Fatalf("pending = %+v", pending)
	}

	SetWaiting(ctx, dir, "", nil, approvalkit.OutcomeApproved, "ok", warn)
	if pending, err = approvalkit.Load(ctx, camp); err != nil || len(pending) != 0 {
		t.Fatalf("pending after resolve = %+v, %v", pending, err)
	}
	if len(warned) != 0 {
		t.Fatalf("unexpected warnings: %v", warned)
	}
}

func TestSetWaiting_IdentitySeparatesSourcesInOneDirectory(t *testing.T) {
	ctx := context.Background()
	camp := tempCamp(t)
	dir := filepath.Join(camp, "festivals", "active", "f", "001_P")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := func(kind string) *Waiting {
		return &Waiting{Kind: kind, Step: 1, Title: "t", Dir: dir}
	}
	SetWaiting(ctx, dir, "001_P", at(approvalkit.SourceFestivalCheckpoint), approvalkit.OutcomeWithdrawn, "", nil)
	SetWaiting(ctx, dir, "gate:001_P", at(approvalkit.SourcePhaseGate), approvalkit.OutcomeWithdrawn, "", nil)
	pending, err := approvalkit.Load(ctx, camp)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
	// Clearing one source leaves the other.
	SetWaiting(ctx, dir, "001_P", nil, approvalkit.OutcomeApproved, "", nil)
	if pending, err = approvalkit.Load(ctx, camp); err != nil || len(pending) != 1 || pending[0].SourceKind != approvalkit.SourcePhaseGate {
		t.Fatalf("pending = %+v, %v", pending, err)
	}
}

func TestSetWaiting_OutsideACampIsSilent(t *testing.T) {
	t.Setenv("CAMP_ROOT", "")
	dir := t.TempDir()
	warned := 0
	SetWaiting(context.Background(), dir, "", &Waiting{Kind: "k", Step: 1, Title: "t", Dir: dir},
		approvalkit.OutcomeWithdrawn, "", func(error) { warned++ })
	if warned != 0 {
		t.Fatalf("outside a camp must be silent, got %d warnings", warned)
	}
	if _, err := os.Stat(filepath.Join(dir, ".campaign")); !os.IsNotExist(err) {
		t.Fatalf("nothing may be created: %v", err)
	}
}

func TestSetWaiting_SourceOutsideTheDetectedCampIsIgnored(t *testing.T) {
	camp := tempCamp(t)
	t.Setenv("CAMP_ROOT", camp)
	elsewhere := t.TempDir()
	SetWaiting(context.Background(), elsewhere, "", &Waiting{Kind: "k", Step: 1, Title: "t", Dir: elsewhere},
		approvalkit.OutcomeWithdrawn, "", nil)
	if _, err := os.Stat(filepath.Join(camp, filepath.FromSlash(approvalkit.Dir))); !os.IsNotExist(err) {
		t.Fatalf("a source outside the camp must not be recorded in it: %v", err)
	}
}
