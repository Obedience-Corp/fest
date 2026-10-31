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
	n.notificationHooks = func(context.Context, string) (string, error) { return "", nil }
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
				want := 2
				if failed {
					want = 0
				}
				if len(events) != want {
					t.Fatalf("events=%+v", events)
				}
				if !failed {
					e := events[0]
					if e.Name != hooks.PresentationOpenName || e.Coord.Step != 2 || e.Coord.FestivalPath != n.festivalPath || events[1].Name != hooks.ApprovalNotifyName || events[1].Verb != hooks.VerbWorkflowApprovalNeeded || events[1].Coord.Step != 2 {
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

func TestApprovalCompletionDoesNotNotify(t *testing.T) {
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
	if len(events) != 0 {
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

func TestOwnedJudgeApprovalOnlyNotifiesDestination(t *testing.T) {
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
		if len(events) != 1 || events[0].Name != hooks.PresentationOpenName || events[0].Coord.Step != 2 {
			t.Fatalf("events=%+v", events)
		}
	}
}

func TestApprovalNeededEntryRouting(t *testing.T) {
	for _, tc := range []struct {
		name    string
		step    WorkflowStep
		judge   bool
		invalid bool
		want    bool
	}{
		{name: "nonblocking", step: WorkflowStep{Number: 2, Name: "APPROVAL"}},
		{name: "human-required-with-judge", step: WorkflowStep{Number: 2, Name: "APPROVAL", Checkpoint: CheckpointUserApproval, Approval: "human-required"}, judge: true, want: true},
		{name: "attestation-with-judge", step: WorkflowStep{Number: 2, Name: "APPROVAL", Checkpoint: CheckpointUserApproval, CheckpointClass: CheckpointClassOperatorAttestation}, judge: true, want: true},
		{name: "artifact-no-judge", step: WorkflowStep{Number: 2, Name: "REVIEW", Checkpoint: CheckpointUserApproval, CheckpointClass: CheckpointClassArtifactReview}, want: true},
		{name: "artifact-with-judge", step: WorkflowStep{Number: 2, Name: "REVIEW", Checkpoint: CheckpointUserApproval, CheckpointClass: CheckpointClassArtifactReview}, judge: true},
		{name: "invalid-judge-config", step: WorkflowStep{Number: 2, Name: "REVIEW", Checkpoint: CheckpointUserApproval, CheckpointClass: CheckpointClassArtifactReview}, invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n, _ := notificationNavigator(t, []WorkflowStep{{Number: 1, Name: "READ"}, tc.step})
			n.notificationHooks = func(context.Context, string) (string, error) {
				if tc.invalid {
					return "", errors.New("invalid judge configuration")
				}
				if tc.judge {
					return "judge", nil
				}
				return "", nil
			}
			ctx, q := hooks.WithNotifications(context.Background())
			if err := n.Advance(ctx); err != nil {
				t.Fatal(err)
			}
			var events []hooks.Notification
			q.Drain(ctx, func(_ context.Context, e hooks.Notification) { events = append(events, e) })
			want := 0
			if tc.want {
				want = 1
			}
			if len(events) != want {
				t.Fatalf("events=%+v", events)
			}
			if tc.want && (events[0].Name != hooks.ApprovalNotifyName || events[0].Verb != hooks.VerbWorkflowApprovalNeeded || events[0].Coord.Step != 2) {
				t.Fatalf("context=%+v", events)
			}
		})
	}
}

func TestApprovalNotificationClearedWithinCommand(t *testing.T) {
	for _, tc := range []struct {
		name             string
		next             *WorkflowStep
		fail             bool
		wantApprovalStep int
	}{
		{name: "approve-final-step"},
		{name: "approve-into-nonblocking", next: &WorkflowStep{Number: 2, Name: "FINALIZE"}},
		{name: "approve-into-human-gate", next: &WorkflowStep{Number: 2, Name: "APPROVAL", Checkpoint: CheckpointUserApproval, Approval: "human-required"}, wantApprovalStep: 2},
		{name: "failed-approval-preserves-request", fail: true, wantApprovalStep: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			steps := []WorkflowStep{{Number: 1, Name: "PRESENT", Checkpoint: CheckpointUserApproval}}
			if tc.next != nil {
				steps = append(steps, *tc.next)
			}
			n, store := notificationNavigator(t, steps)
			ctx, q := hooks.WithNotifications(context.Background())
			// Match Initialize's persisted entry before approval in the same command.
			if err := n.saveTransition(ctx, 0); err != nil {
				t.Fatal(err)
			}
			if tc.fail {
				store.err = errors.New("approval save failed")
			}
			if err := n.Approve(ctx); (err != nil) != tc.fail {
				t.Fatalf("approval error=%v", err)
			}
			var events []hooks.Notification
			q.Drain(ctx, func(_ context.Context, e hooks.Notification) { events = append(events, e) })
			want := 1 // PRESENT is still an entry event.
			if tc.wantApprovalStep != 0 {
				want++
			}
			if len(events) != want || events[0].Name != hooks.PresentationOpenName {
				t.Fatalf("events=%+v", events)
			}
			if tc.wantApprovalStep != 0 && (events[1].Name != hooks.ApprovalNotifyName || events[1].Coord.Step != tc.wantApprovalStep) {
				t.Fatalf("events=%+v", events)
			}
		})
	}
}
