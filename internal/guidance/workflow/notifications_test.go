package workflow

import (
	"context"
	"errors"
	"testing"

	"github.com/Obedience-Corp/fest/internal/guidance"
	"github.com/Obedience-Corp/fest/internal/hooks"
)

type notificationStore struct {
	err   error
	saved int
}

func (*notificationStore) LoadWorkflowPhaseState(string) (*WorkflowState, bool) { return nil, false }
func (*notificationStore) QueueWorkflowEvents([]WorkflowEvent)                  {}
func (s *notificationStore) SaveEvents(context.Context) error                   { s.saved++; return s.err }

// This fixture performs no filesystem mutations: the base only reads missing
// guidance state, and workflow persistence is injected in memory.
func notificationNavigator(t *testing.T, steps []WorkflowStep) (*Navigator, *notificationStore) {
	t.Helper()
	n, err := NewNavigator(&guidance.GuidanceContext{FestivalPath: "/nonexistent/fest-notification-fixture"}, guidance.ModeWorkflow)
	if err != nil {
		t.Fatal(err)
	}
	if err = n.BaseNavigator.Initialize(context.Background()); err != nil {
		t.Fatal(err)
	}
	n.steps = steps
	n.festivalPath = n.Ctx.FestivalPath
	n.phaseName = "001_REVIEW"
	n.workflowState = NewWorkflowState(len(steps))
	n.workflowState.Initialize(steps)
	store := &notificationStore{}
	n.SetStateStore(store)
	return n, store
}

func TestPresentationTransitionNotifications(t *testing.T) {
	for _, method := range []string{"advance", "complete", "skip", "approve"} {
		for _, failed := range []bool{false, true} {
			t.Run(method+map[bool]string{true: "-save-failure", false: "-success"}[failed], func(t *testing.T) {
				n, store := notificationNavigator(t, []WorkflowStep{{Number: 1, Name: "READ"}, {Number: 2, Name: "PRESENT", Checkpoint: CheckpointUserApproval, Approval: "human-required"}})
				if failed {
					store.err = errors.New("disk unavailable")
				}
				ctx, q := hooks.WithNotifications(context.Background())
				var err error
				switch method {
				case "advance":
					err = n.Advance(ctx)
				case "complete":
					err = n.MarkComplete(ctx, "step_1")
				case "skip":
					err = n.SkipCurrentStep(ctx, StepStatusSkipped, "external")
				case "approve":
					err = n.Approve(ctx)
				}
				if (err != nil) != failed {
					t.Fatalf("err=%v", err)
				}
				var events []hooks.Notification
				q.Drain(ctx, func(_ context.Context, e hooks.Notification) { events = append(events, e) })
				want := 1
				if failed {
					want = 0
				} else if method == "approve" {
					want = 2
				}
				if len(events) != want {
					t.Fatalf("events=%+v", events)
				}
				if !failed {
					e := events[len(events)-1]
					if e.Name != hooks.PresentationOpenName || e.Coord.Step != 2 || e.Coord.FestivalPath != n.festivalPath {
						t.Fatalf("entry=%+v", e)
					}
					if n.workflowState.GetCurrentStepState().Status.IsTerminal() {
						t.Fatal("notification approved human gate")
					}
				}
			})
		}
	}
}

func TestPresentationResetAndReadNotifications(t *testing.T) {
	n, _ := notificationNavigator(t, []WorkflowStep{{Number: 1, Name: "PRESENT"}})
	ctx, q := hooks.WithNotifications(context.Background())
	if _, err := n.GetNext(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := n.GetProgress(ctx); err != nil {
		t.Fatal(err)
	}
	count := 0
	q.Drain(ctx, func(context.Context, hooks.Notification) { count++ })
	if count != 0 {
		t.Fatal("read queued notification")
	}
	ctx, q = hooks.WithNotifications(context.Background())
	if err := n.Reset(ctx); err != nil {
		t.Fatal(err)
	}
	q.Drain(ctx, func(context.Context, hooks.Notification) { count++ })
	if count != 1 {
		t.Fatalf("reset count=%d", count)
	}
}

func TestApprovalCompletionDoesNotReplay(t *testing.T) {
	n, _ := notificationNavigator(t, []WorkflowStep{{Number: 1, Name: "REVIEW", Checkpoint: CheckpointUserApproval}})
	ctx, q := hooks.WithNotifications(context.Background())
	if err := n.Approve(ctx); err != nil {
		t.Fatal(err)
	}
	if err := n.Approve(ctx); err != nil {
		t.Fatal(err)
	}
	var events []hooks.Notification
	q.Drain(ctx, func(_ context.Context, e hooks.Notification) { events = append(events, e) })
	if len(events) != 1 || events[0].Name != hooks.ApprovalNotifyName || events[0].Coord.Step != 1 {
		t.Fatalf("events=%+v", events)
	}
}

func TestSupersededJudgeDoesNotNotify(t *testing.T) {
	n, _ := notificationNavigator(t, []WorkflowStep{{Number: 1, Name: "REVIEW"}, {Number: 2, Name: "PRESENT"}})
	ctx, q := hooks.WithNotifications(context.Background())
	applied, err := n.ApplyJudgeApproval(ctx, 1, "unowned", "", DecisionMetadata{}, JudgeVerdictExtras{})
	if applied || err != nil {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	q.Drain(ctx, func(context.Context, hooks.Notification) { t.Fatal("superseded judge notified") })
}

func TestOwnedJudgeApprovalQueuesApprovedThenPresent(t *testing.T) {
	for _, failed := range []bool{false, true} {
		n, store := notificationNavigator(t, []WorkflowStep{{Number: 1, Name: "REVIEW", Checkpoint: CheckpointUserApproval}, {Number: 2, Name: "PRESENT"}})
		n.workflowState.GetCurrentStepState().Judge = &JudgeState{Status: JudgeRunning, RunID: "owned", Pid: 1}
		if failed {
			store.err = errors.New("save failed")
		}
		ctx, q := hooks.WithNotifications(context.Background())
		applied, err := n.ApplyJudgeApproval(ctx, 1, "owned", "reviewed", DecisionMetadata{Actor: "agent", Summary: "approved"}, JudgeVerdictExtras{})
		if !applied || (err != nil) != failed {
			t.Fatalf("applied=%v err=%v", applied, err)
		}
		var events []hooks.Notification
		q.Drain(ctx, func(_ context.Context, e hooks.Notification) { events = append(events, e) })
		if failed {
			if len(events) != 0 {
				t.Fatal(events)
			}
			continue
		}
		if len(events) != 2 || events[0].Name != hooks.ApprovalNotifyName || events[0].Coord.Step != 1 || events[1].Name != hooks.PresentationOpenName || events[1].Coord.Step != 2 {
			t.Fatalf("events=%+v", events)
		}
	}
}
