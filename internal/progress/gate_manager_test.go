package progress

import (
	"context"
	stderrors "errors"
	"testing"
)

// blockingGate is a Gate that always blocks with a sentinel error.
type blockingGate struct{}

var errBlocked = stderrors.New("blocked by gate")

func (blockingGate) EnforceForTask(_ context.Context, _ string) error {
	return errBlocked
}

// TestManager_GateBlocksMutations verifies every mutation method on Manager
// consults the gate before changing state. Without this wiring, a Manager
// constructed with a real lifecycle gate would still let mutations through
// when callers reach Manager directly (defense in depth).
func TestManager_GateBlocksMutations(t *testing.T) {
	ctx := context.Background()
	mgr, err := NewManagerWithGate(ctx, t.TempDir(), blockingGate{})
	if err != nil {
		t.Fatalf("NewManagerWithGate: %v", err)
	}

	cases := []struct {
		name string
		call func() error
	}{
		{"UpdateProgress", func() error { return mgr.UpdateProgress(ctx, "001/01/01_t.md", 50) }},
		{"MarkComplete", func() error { return mgr.MarkComplete(ctx, "001/01/01_t.md") }},
		{"MarkInProgress", func() error { return mgr.MarkInProgress(ctx, "001/01/01_t.md") }},
		{"ReportBlocker", func() error { return mgr.ReportBlocker(ctx, "001/01/01_t.md", "x") }},
		{"ResetTask", func() error { return mgr.ResetTask(ctx, "001/01/01_t.md") }},
		{"ClearBlocker", func() error { return mgr.ClearBlocker(ctx, "001/01/01_t.md") }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call()
			if !stderrors.Is(err, errBlocked) {
				t.Errorf("%s: expected gate to block (got %v)", tc.name, err)
			}
		})
	}
}

// TestManager_NoopGateAllowsMutations verifies plain NewManager (which
// installs NoopGate) does not gate mutations. This preserves the
// pre-existing behaviour of the 21+ NewManager call sites.
func TestManager_NoopGateAllowsMutations(t *testing.T) {
	ctx := context.Background()
	mgr, err := NewManager(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.UpdateProgress(ctx, "001/01/01_t.md", 25); err != nil {
		t.Errorf("UpdateProgress with NoopGate should not error, got: %v", err)
	}
}
