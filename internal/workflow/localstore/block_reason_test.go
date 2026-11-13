package localstore

import (
	"strings"
	"testing"
)

func TestReplayEventStream_BlockReason(t *testing.T) {
	const (
		start    = `{"event_type":"wf_step_start"}`
		block    = `{"event_type":"wf_step_block"}`
		rejected = `{"event_type":"wf_checkpoint_rejected","feedback":"missing evidence"}`
		approved = `{"event_type":"wf_checkpoint_approved"}`
		done     = `{"event_type":"wf_step_done"}`
	)
	cases := []struct {
		name         string
		events       []string
		wantBlocked  bool
		wantReason   string
		wantFeedback string
		wantAwaiting bool
		wantDone     int
	}{
		{"checkpoint reached", []string{start, block}, true, BlockReasonCheckpoint, "", true, 0},
		{"rejected waits on a revision", []string{start, block, rejected}, true, BlockReasonRejected, "missing evidence", false, 0},
		{"re-submitted after rejection", []string{start, block, rejected, block}, true, BlockReasonCheckpoint, "", true, 0},
		{"approved and done", []string{start, block, approved, done}, false, "", "", false, 1},
		{"approved clears a rejection's feedback", []string{start, block, rejected, block, approved, done}, false, "", "", false, 1},
		{"not blocked", []string{start}, false, "", "", false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, err := replayEventStream(strings.NewReader(strings.Join(c.events, "\n")))
			if err != nil {
				t.Fatal(err)
			}
			if state.Blocked != c.wantBlocked || state.BlockReason != c.wantReason ||
				state.Feedback != c.wantFeedback || state.AwaitingApproval() != c.wantAwaiting ||
				state.CompletedSteps != c.wantDone {
				t.Fatalf("state = %+v", state)
			}
		})
	}
}

func TestAwaitingApproval_NilState(t *testing.T) {
	var s *RunState
	if s.AwaitingApproval() {
		t.Fatal("nil state must not be awaiting approval")
	}
}
