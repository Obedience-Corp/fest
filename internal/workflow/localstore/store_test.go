package localstore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateManifest(t *testing.T) {
	valid := Manifest{
		Version:    ManifestVersion,
		Kind:       ManifestKind,
		WorkflowID: "wf-x",
	}
	if err := validateManifest(&valid, "/virtual/.workflow/workflow.yaml"); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}

	cases := []struct {
		name string
		m    Manifest
		want string
	}{
		{
			name: "missing version",
			m: Manifest{
				Kind:       ManifestKind,
				WorkflowID: "wf-x",
			},
			want: "missing version",
		},
		{
			name: "unsupported version",
			m: Manifest{
				Version:    99,
				Kind:       ManifestKind,
				WorkflowID: "wf-x",
			},
			want: "unsupported workflow manifest version",
		},
		{
			name: "wrong kind",
			m: Manifest{
				Version:    ManifestVersion,
				Kind:       "not-workflow",
				WorkflowID: "wf-x",
			},
			want: "kind mismatch",
		},
		{
			name: "missing workflow_id",
			m: Manifest{
				Version: ManifestVersion,
				Kind:    ManifestKind,
			},
			want: "missing workflow_id",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateManifest(&c.m, "/virtual/.workflow/workflow.yaml")
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not contain %q", err.Error(), c.want)
			}
		})
	}
}

func TestReplayEventStream_DerivesProgress(t *testing.T) {
	events := strings.Join([]string{
		`{"event_type":"workflow_run_started"}`,
		`{"event_type":"wf_step_start"}`,
		`{"event_type":"wf_step_block"}`,
		`{"event_type":"wf_step_skip"}`,
		`{"event_type":"wf_step_start"}`,
		`{"event_type":"wf_step_done"}`,
		`{"event_type":"unknown_event"}`,
		`not-json`,
	}, "\n")

	state, err := replayEventStream(strings.NewReader(events), RunManifest{
		Summary: RunSummary{
			CurrentStep:    999,
			CompletedSteps: 998,
			Blocked:        true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if state.CurrentStep != 2 || state.CompletedSteps != 2 {
		t.Errorf("CurrentStep=%d CompletedSteps=%d, want 2/2", state.CurrentStep, state.CompletedSteps)
	}
	if state.Blocked {
		t.Error("Blocked = true, want false after skip/done")
	}
	if state.Status != "active" {
		t.Errorf("Status = %q, want active", state.Status)
	}
}

func TestReplayEventStream_StatusTransitions(t *testing.T) {
	cases := []struct {
		name          string
		events        string
		wantStatus    string
		wantBlocked   bool
		wantCompleted int
	}{
		{
			name:        "block marks blocked",
			events:      `{"event_type":"wf_step_start"}` + "\n" + `{"event_type":"wf_step_block"}`,
			wantStatus:  "blocked",
			wantBlocked: true,
		},
		{
			name:          "completion marks completed",
			events:        `{"event_type":"wf_step_start"}` + "\n" + `{"event_type":"wf_step_done"}` + "\n" + `{"event_type":"workflow_run_completed"}`,
			wantStatus:    "completed",
			wantCompleted: 1,
		},
		{
			name:       "abandon marks abandoned",
			events:     `{"event_type":"workflow_run_abandoned"}`,
			wantStatus: "abandoned",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			state, err := replayEventStream(strings.NewReader(c.events), RunManifest{})
			if err != nil {
				t.Fatal(err)
			}
			if state.Status != c.wantStatus {
				t.Errorf("Status = %q, want %q", state.Status, c.wantStatus)
			}
			if state.Blocked != c.wantBlocked {
				t.Errorf("Blocked = %v, want %v", state.Blocked, c.wantBlocked)
			}
			if state.CompletedSteps != c.wantCompleted {
				t.Errorf("CompletedSteps = %d, want %d", state.CompletedSteps, c.wantCompleted)
			}
		})
	}
}

func TestNewRunID(t *testing.T) {
	ts := time.Date(2026, 5, 18, 12, 34, 56, 0, time.UTC)
	if got := newRunID(ts); got != "run-20260518T123456Z" {
		t.Fatalf("newRunID() = %q", got)
	}
}

func TestStore_InitReturnsCanceledContextBeforeMutation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Open("/virtual/.workflow", "/virtual/WORKFLOW.md").Init(ctx, InitOptions{WorkflowID: "wf-x"})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// Real .workflow/ filesystem mutation coverage lives in
// tests/integration/standalone_*_test.go so unit tests do not mutate the host.
