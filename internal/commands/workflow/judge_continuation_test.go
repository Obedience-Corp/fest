package workflow

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/continuation"
	"github.com/Obedience-Corp/fest/internal/guidance"
	wf "github.com/Obedience-Corp/fest/internal/guidance/workflow"
)

type fakeContinuationNotifier struct {
	calls []continuation.SessionNotification
	err   error
}

func (f *fakeContinuationNotifier) Notify(_ context.Context, n continuation.SessionNotification) error {
	f.calls = append(f.calls, n)
	return f.err
}

func swapContinuationNotifier(t *testing.T, n continuation.Notifier) {
	t.Helper()
	orig := judgeContinuationNotifier
	judgeContinuationNotifier = n
	t.Cleanup(func() { judgeContinuationNotifier = orig })
}

func continuationTestNav(t *testing.T, festivalName string) *wf.Navigator {
	t.Helper()
	festivalPath := filepath.Join(t.TempDir(), festivalName)
	if err := os.MkdirAll(festivalPath, 0o755); err != nil {
		t.Fatalf("mkdir festival: %v", err)
	}
	gctx := &guidance.GuidanceContext{
		FestivalPath: festivalPath,
		FestivalName: festivalName,
		PhaseName:    "001_IMPLEMENT",
		Mode:         guidance.ModeWorkflow,
	}
	nav, err := wf.NewNavigator(gctx, guidance.ModeWorkflow)
	if err != nil {
		t.Fatalf("NewNavigator: %v", err)
	}
	return nav
}

// The payload struct is the durable carrier of the captured identity. When no
// identity is captured, the continuation fields must not appear on the wire so
// standalone payloads are byte-for-byte unchanged.
func TestJudgeExecPayloadOmitsContinuationWhenAbsent(t *testing.T) {
	data, err := json.Marshal(judgeExecPayload{
		SchemaVersion: judgeExecPayloadSchema,
		StepNumber:    2,
		RunID:         "run-1",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(data), "target_session_id") || strings.Contains(string(data), "campaign_id") {
		t.Fatalf("absent identity must not be serialized: %s", data)
	}
}

func TestJudgeExecPayloadCarriesCapturedIdentity(t *testing.T) {
	data, err := json.Marshal(judgeExecPayload{
		SchemaVersion:   judgeExecPayloadSchema,
		StepNumber:      2,
		RunID:           "run-1",
		TargetSessionID: "ses_A",
		CampaignID:      "camp_A",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got judgeExecPayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.TargetSessionID != "ses_A" || got.CampaignID != "camp_A" {
		t.Fatalf("identity not preserved: %+v", got)
	}
	if continuation.DeliveryID(got.RunID) != "fest-judge:run-1" {
		t.Fatalf("delivery id = %q", continuation.DeliveryID(got.RunID))
	}
}

// This is the exact capture the launch path performs: read identity once from
// the environment and pin it into the payload.
func TestLaunchCaptureResolvesIdentityFromEnv(t *testing.T) {
	t.Setenv(continuation.EnvSessionID, "ses_launch")
	t.Setenv(continuation.EnvCampaignID, "camp_launch")

	cc, ok := continuation.Resolve(os.LookupEnv)
	if !ok {
		t.Fatal("expected identity to resolve from environment")
	}
	payload := judgeExecPayload{RunID: "run-9"}
	payload.TargetSessionID = cc.SessionID
	payload.CampaignID = cc.CampaignID

	if payload.TargetSessionID != "ses_launch" || payload.CampaignID != "camp_launch" {
		t.Fatalf("payload identity = %+v", payload)
	}
}

func TestFireJudgeContinuationNoIdentityIsNoop(t *testing.T) {
	fake := &fakeContinuationNotifier{}
	swapContinuationNotifier(t, fake)
	nav := continuationTestNav(t, "example-festival")

	payload := judgeExecPayload{StepNumber: 2, StepName: "Implement the change", RunID: "run-1"}
	out := captureStdout(t, func() {
		fireJudgeContinuation(context.Background(), nav, payload, continuation.VerdictApproved, "ok")
	})

	if len(fake.calls) != 0 {
		t.Fatalf("no identity must not notify, got %d calls", len(fake.calls))
	}
	if out != "" {
		t.Fatalf("no identity must be silent, got %q", out)
	}
}

func TestFireJudgeContinuationApprovalEnvelope(t *testing.T) {
	fake := &fakeContinuationNotifier{}
	swapContinuationNotifier(t, fake)
	nav := continuationTestNav(t, "example-festival")

	payload := judgeExecPayload{
		StepNumber:      2,
		StepName:        "Implement the change",
		RunID:           "8f3c1a2b",
		TargetSessionID: "ses_01J9ZX3ABCDEF",
		CampaignID:      "camp_01J9ZWXYZ0123",
	}
	fireJudgeContinuation(context.Background(), nav, payload, continuation.VerdictApproved, "Evidence is complete.")

	if len(fake.calls) != 1 {
		t.Fatalf("expected one notification, got %d", len(fake.calls))
	}
	n := fake.calls[0]
	if n.TargetSessionID != "ses_01J9ZX3ABCDEF" || n.CampaignID != "camp_01J9ZWXYZ0123" {
		t.Fatalf("identity = %q/%q", n.TargetSessionID, n.CampaignID)
	}
	if n.DeliveryID != "fest-judge:8f3c1a2b" {
		t.Fatalf("delivery id = %q", n.DeliveryID)
	}
	if n.Metadata["verdict"] != "approved" || n.Metadata["authority"] != "authoritative" {
		t.Fatalf("metadata = %+v", n.Metadata)
	}
	if !strings.Contains(n.Message, "Continue with: fest next") {
		t.Fatalf("approval message missing next command: %q", n.Message)
	}
	if strings.Contains(strings.ToLower(n.Message), "reject") {
		t.Fatalf("approval message must not carry rejection text: %q", n.Message)
	}
}

func TestFireJudgeContinuationRejectionEnvelope(t *testing.T) {
	fake := &fakeContinuationNotifier{}
	swapContinuationNotifier(t, fake)
	nav := continuationTestNav(t, "example-festival")

	payload := judgeExecPayload{
		StepNumber:      2,
		StepName:        "Implement the change",
		RunID:           "run-r",
		TargetSessionID: "ses_X",
		CampaignID:      "camp_Y",
	}
	fireJudgeContinuation(context.Background(), nav, payload, continuation.VerdictRejected, "missing acceptance proof")

	n := fake.calls[0]
	if n.Metadata["verdict"] != "rejected" {
		t.Fatalf("verdict metadata = %q", n.Metadata["verdict"])
	}
	if !strings.Contains(n.Message, "missing acceptance proof") {
		t.Fatalf("rejection feedback missing: %q", n.Message)
	}
	if !strings.HasSuffix(n.Message, "run: fest workflow judge") {
		t.Fatalf("rejection must end with rejudge command: %q", n.Message)
	}
}

// Proves the detached runner uses the identity captured at launch (pinned in
// the payload), not a later shell environment that may target another session.
func TestFireJudgeContinuationUsesCapturedIdentityNotEnv(t *testing.T) {
	fake := &fakeContinuationNotifier{}
	swapContinuationNotifier(t, fake)
	nav := continuationTestNav(t, "example-festival")

	// Environment now points at a different session than the one captured.
	t.Setenv(continuation.EnvSessionID, "ses_LATER")
	t.Setenv(continuation.EnvCampaignID, "camp_LATER")

	payload := judgeExecPayload{
		StepNumber:      1,
		StepName:        "Do the thing",
		RunID:           "run-c",
		TargetSessionID: "ses_CAPTURED",
		CampaignID:      "camp_CAPTURED",
	}
	fireJudgeContinuation(context.Background(), nav, payload, continuation.VerdictApproved, "ok")

	n := fake.calls[0]
	if n.TargetSessionID != "ses_CAPTURED" || n.CampaignID != "camp_CAPTURED" {
		t.Fatalf("detached runner re-read env instead of payload: %q/%q", n.TargetSessionID, n.CampaignID)
	}
}

// A replayed terminal event reuses the run id, so the derived delivery id is
// stable and Obey can deduplicate to a single effective turn.
func TestFireJudgeContinuationDuplicateStableDeliveryID(t *testing.T) {
	fake := &fakeContinuationNotifier{}
	swapContinuationNotifier(t, fake)
	nav := continuationTestNav(t, "example-festival")

	payload := judgeExecPayload{
		StepNumber:      2,
		StepName:        "Implement the change",
		RunID:           "run-dup",
		TargetSessionID: "ses_X",
		CampaignID:      "camp_Y",
	}
	fireJudgeContinuation(context.Background(), nav, payload, continuation.VerdictApproved, "ok")
	fireJudgeContinuation(context.Background(), nav, payload, continuation.VerdictApproved, "ok")

	if len(fake.calls) != 2 {
		t.Fatalf("expected two submit attempts, got %d", len(fake.calls))
	}
	if fake.calls[0].DeliveryID != fake.calls[1].DeliveryID {
		t.Fatalf("delivery id not stable: %q vs %q", fake.calls[0].DeliveryID, fake.calls[1].DeliveryID)
	}
	if fake.calls[0].DeliveryID != "fest-judge:run-dup" {
		t.Fatalf("delivery id = %q", fake.calls[0].DeliveryID)
	}
}

// A notification submission failure must never propagate into the judge outcome;
// it degrades to the manual next-step instruction.
func TestFireJudgeContinuationNotifierFailureIsolated(t *testing.T) {
	fake := &fakeContinuationNotifier{err: context.DeadlineExceeded}
	swapContinuationNotifier(t, fake)
	nav := continuationTestNav(t, "example-festival")

	payload := judgeExecPayload{
		StepNumber:      2,
		StepName:        "Implement the change",
		RunID:           "run-f",
		TargetSessionID: "ses_X",
		CampaignID:      "camp_Y",
	}
	out := captureStdout(t, func() {
		fireJudgeContinuation(context.Background(), nav, payload, continuation.VerdictRejected, "missing proof")
	})

	if len(fake.calls) != 1 {
		t.Fatalf("expected one submit attempt, got %d", len(fake.calls))
	}
	if !strings.Contains(out, "was not delivered") {
		t.Fatalf("failure must print manual fallback, got %q", out)
	}
	if !strings.Contains(out, "fest workflow judge") {
		t.Fatalf("fallback must name the recovery command, got %q", out)
	}
}
