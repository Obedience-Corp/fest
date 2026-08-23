package runloop

import "testing"

func TestClassify(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		snap    Snapshot
		outcome string
	}{
		{name: "complete", snap: Snapshot{Complete: true, Label: "DONE"}, outcome: OutcomeCompleted},
		{name: "judge", snap: Snapshot{WaitingOnJudge: true, Label: "GATE"}, outcome: OutcomeWaitingJudge},
		{name: "blocked", snap: Snapshot{Blocked: true, Label: "WAIT"}, outcome: OutcomeWaitingHuman},
		{name: "human approval field", snap: Snapshot{HumanApproval: true, Label: "SHIP"}, outcome: OutcomeWaitingHuman},
		{name: "user_approval checkpoint", snap: Snapshot{Checkpoint: "user_approval", Label: "REVIEW"}, outcome: OutcomeWaitingHuman},
		{name: "approval_required alias", snap: Snapshot{Checkpoint: "approval_required", Label: "REVIEW"}, outcome: OutcomeWaitingHuman},
		{name: "operator attestation", snap: Snapshot{CheckpointClass: "operator_attestation", Label: "ACCEPT"}, outcome: OutcomeWaitingHuman},
		{name: "blocked task status", snap: Snapshot{TaskStatus: "blocked", Label: "AUTH"}, outcome: OutcomeWaitingHuman},
		{name: "low autonomy", snap: Snapshot{AutonomyLevel: "low", Label: "DESIGN"}, outcome: OutcomeWaitingHuman},
		{name: "festival empty", snap: Snapshot{Kind: "festival"}, outcome: OutcomeFailed},
		{name: "runnable standalone", snap: Snapshot{Kind: "standalone", Label: "ALIGN"}, outcome: OutcomeRunnable},
		{name: "runnable festival task", snap: Snapshot{Kind: "festival", Label: "01_build", AutonomyLevel: "high"}, outcome: OutcomeRunnable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Classify(tc.snap)
			if got.Outcome != tc.outcome {
				t.Fatalf("outcome = %s, want %s (%s)", got.Outcome, tc.outcome, got.Reason)
			}
		})
	}
}
