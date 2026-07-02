package next

import (
	"context"
	"path/filepath"
	"testing"
)

func TestRouteEarlierIncompleteWork_HandlesEarlierGateFromLaterPhase(t *testing.T) {
	festDir := scaffoldWorkflowGateFestival(t)
	ctx := context.Background()

	completeWorkflow(t, festDir, "001_INGEST", 2)

	if got, _ := findEarlierIncompletePhaseGate(ctx, festDir, "002_PLAN"); filepath.Base(got) != "001_INGEST" {
		t.Fatalf("findEarlierIncompletePhaseGate = %q, want 001_INGEST", got)
	}

	handled, err := routeEarlierIncompleteWork(ctx, festDir, "002_PLAN")
	if err != nil {
		t.Fatalf("routeEarlierIncompleteWork: %v", err)
	}
	if !handled {
		t.Fatal("routeEarlierIncompleteWork handled = false, want true (earlier gate must run before a later phase)")
	}
}

func TestRouteEarlierIncompleteWork_NotHandledWhenEarlierComplete(t *testing.T) {
	festDir := scaffoldWorkflowGateFestival(t)
	ctx := context.Background()

	completeWorkflow(t, festDir, "001_INGEST", 2)
	completeWorkflow(t, festDir, "gate:001_INGEST", 1)

	handled, err := routeEarlierIncompleteWork(ctx, festDir, "002_PLAN")
	if err != nil {
		t.Fatalf("routeEarlierIncompleteWork: %v", err)
	}
	if handled {
		t.Fatal("routeEarlierIncompleteWork handled = true, want false when the earlier phase is complete")
	}
}

func TestRouteEarlierIncompleteWork_FailsClosedOnUnreadableFestival(t *testing.T) {
	ctx := context.Background()
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	handled, err := routeEarlierIncompleteWork(ctx, missing, "002_PLAN")
	if !handled {
		t.Fatal("routeEarlierIncompleteWork handled = false on a read error, want true (fail closed)")
	}
	if err == nil {
		t.Fatal("routeEarlierIncompleteWork err = nil on a read error, want a propagated error")
	}
}
