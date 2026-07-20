package continuation

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name   string
		env    map[string]string
		wantOK bool
		want   Context
	}{
		{
			name:   "both present and valid",
			env:    map[string]string{EnvSessionID: "ses_01J9ZX3ABCDEF", EnvCampaignID: "camp_01J9ZWXYZ0123"},
			wantOK: true,
			want:   Context{SessionID: "ses_01J9ZX3ABCDEF", CampaignID: "camp_01J9ZWXYZ0123"},
		},
		{
			name:   "session absent disables continuation",
			env:    map[string]string{EnvCampaignID: "camp_01J9ZWXYZ0123"},
			wantOK: false,
		},
		{
			name:   "campaign absent fails closed",
			env:    map[string]string{EnvSessionID: "ses_01J9ZX3ABCDEF"},
			wantOK: false,
		},
		{
			name:   "empty session is not an identity",
			env:    map[string]string{EnvSessionID: "   ", EnvCampaignID: "camp_01J9ZWXYZ0123"},
			wantOK: false,
		},
		{
			name:   "unsafe characters rejected",
			env:    map[string]string{EnvSessionID: "ses foo; rm -rf", EnvCampaignID: "camp_01J9ZWXYZ0123"},
			wantOK: false,
		},
		{
			name:   "surrounding whitespace trimmed",
			env:    map[string]string{EnvSessionID: "  ses_X  ", EnvCampaignID: "  camp_Y  "},
			wantOK: true,
			want:   Context{SessionID: "ses_X", CampaignID: "camp_Y"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := Resolve(lookupFrom(tc.env))
			if ok != tc.wantOK {
				t.Fatalf("Resolve ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Fatalf("Resolve = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDeliveryIDIsDeterministic(t *testing.T) {
	const runID = "8f3c1a2b-4d5e-6f70-8192-a3b4c5d6e7f8"
	want := "fest-judge:" + runID
	if got := DeliveryID(runID); got != want {
		t.Fatalf("DeliveryID = %q, want %q", got, want)
	}
}

func TestNextCommand(t *testing.T) {
	if got := NextCommand(VerdictApproved); got != "fest next" {
		t.Fatalf("approved next command = %q, want fest next", got)
	}
	for _, v := range []Verdict{VerdictRejected, VerdictFailed} {
		if got := NextCommand(v); got != "fest workflow judge" {
			t.Fatalf("%s next command = %q, want fest workflow judge", v, got)
		}
	}
}

func approvalResult() JudgeResult {
	return JudgeResult{
		RunID:         "8f3c1a2b-4d5e-6f70-8192-a3b4c5d6e7f8",
		TargetSession: "ses_01J9ZX3ABCDEF",
		CampaignID:    "camp_01J9ZWXYZ0123",
		FestivalID:    "FE0001",
		FestivalName:  "example-festival",
		Phase:         "001_IMPLEMENT",
		StepNumber:    2,
		StepName:      "Implement the change",
		Verdict:       VerdictApproved,
		Feedback:      "Evidence is complete.",
	}
}

func TestRenderMessageApproval(t *testing.T) {
	msg := RenderMessage(approvalResult())
	if !strings.Contains(msg, "approved step 2 (Implement the change) in example-festival") {
		t.Fatalf("approval message missing location: %q", msg)
	}
	if !strings.HasSuffix(msg, "Continue with: fest next") {
		t.Fatalf("approval message must end with the next command: %q", msg)
	}
	if strings.Contains(msg, "Feedback:") || strings.Contains(strings.ToLower(msg), "reject") {
		t.Fatalf("approval message must not carry rejection instruction: %q", msg)
	}
}

func TestRenderMessageRejection(t *testing.T) {
	r := approvalResult()
	r.Verdict = VerdictRejected
	r.Feedback = "missing acceptance proof"
	msg := RenderMessage(r)
	if !strings.Contains(msg, "rejected step 2") {
		t.Fatalf("rejection message missing verdict: %q", msg)
	}
	if !strings.Contains(msg, "Feedback: missing acceptance proof") {
		t.Fatalf("rejection message missing feedback: %q", msg)
	}
	if !strings.HasSuffix(msg, "run: fest workflow judge") {
		t.Fatalf("rejection message must end with the rejudge command: %q", msg)
	}
	if strings.Contains(msg, "Fixes required:") {
		t.Fatalf("rejection with no followups must not render a fixes list: %q", msg)
	}
}

func TestRenderMessageRejectionFollowups(t *testing.T) {
	r := approvalResult()
	r.Verdict = VerdictRejected
	r.Feedback = "missing acceptance proof"
	r.Followups = []string{"Attach the test console output.", "Include the ledger transition as captured output."}
	msg := RenderMessage(r)
	if !strings.Contains(msg, "Feedback: missing acceptance proof") {
		t.Fatalf("rejection message missing feedback: %q", msg)
	}
	if !strings.Contains(msg, "Fixes required:") {
		t.Fatalf("rejection message missing fixes list: %q", msg)
	}
	for _, f := range r.Followups {
		if !strings.Contains(msg, "- "+f) {
			t.Fatalf("rejection message missing followup %q: %q", f, msg)
		}
	}
	// The itemized fixes precede the closing rejudge instruction.
	if !strings.HasSuffix(msg, "Address the feedback, then run: fest workflow judge") {
		t.Fatalf("rejection message must end with the rejudge instruction: %q", msg)
	}
}

func TestRenderFollowupsBoundsAndSanitizes(t *testing.T) {
	// Count is capped so a malformed verdict cannot make the message unbounded.
	many := make([]string, maxFollowups+5)
	for i := range many {
		many[i] = "fix item"
	}
	got := renderFollowups(many)
	if n := strings.Count(got, "\n- "); n != maxFollowups {
		t.Fatalf("rendered %d items, want cap of %d", n, maxFollowups)
	}
	// Blank/whitespace-only items are dropped rather than rendered empty.
	if renderFollowups([]string{"   ", ""}) != "" {
		t.Fatalf("blank followups must render nothing")
	}
}

func TestRenderMessageFailure(t *testing.T) {
	r := approvalResult()
	r.Verdict = VerdictFailed
	r.Feedback = "approval judge failed: exit status 127"
	msg := RenderMessage(r)
	if !strings.Contains(msg, "failed for step 2") {
		t.Fatalf("failure message missing verdict: %q", msg)
	}
	if !strings.HasSuffix(msg, "run: fest workflow judge") {
		t.Fatalf("failure message must end with the rejudge command: %q", msg)
	}
	if !strings.Contains(msg, "Inspect the judge configuration or evidence") {
		t.Fatalf("failure message must be actionable: %q", msg)
	}
}

func TestSanitizeFeedbackBoundsAndFlattens(t *testing.T) {
	raw := "line one\nline two\twith\r\ncontrol\x00chars" + strings.Repeat(" padded", 200)
	r := approvalResult()
	r.Verdict = VerdictRejected
	r.Feedback = raw
	msg := RenderMessage(r)
	// The message must stay structured: exactly three lines (verdict, feedback,
	// action). Embedded newlines in feedback would break the required trailing
	// command, so they must be collapsed.
	lines := strings.Split(msg, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 message lines, got %d: %q", len(lines), msg)
	}
	if !strings.HasSuffix(msg, "run: fest workflow judge") {
		t.Fatalf("bounded message must still end with rejudge command: %q", msg)
	}
	feedbackLine := lines[1]
	if len([]rune(feedbackLine)) > len("Feedback: ")+maxFeedbackLen+len("...") {
		t.Fatalf("feedback line not bounded: %d runes", len([]rune(feedbackLine)))
	}
	if strings.Contains(feedbackLine, "\x00") || strings.Contains(feedbackLine, "\t") {
		t.Fatalf("control characters not stripped: %q", feedbackLine)
	}
}

func TestSanitizeFeedbackEmpty(t *testing.T) {
	if got := sanitizeFeedback("   \n\t "); got != "(no detail provided)" {
		t.Fatalf("empty feedback = %q, want placeholder", got)
	}
}

func TestBuildNotificationShape(t *testing.T) {
	n := BuildNotification(approvalResult())
	if n.SchemaVersion != NotificationSchema {
		t.Fatalf("schema = %q", n.SchemaVersion)
	}
	if n.Source != "fest" || n.Kind != "workflow.judge" {
		t.Fatalf("source/kind = %q/%q", n.Source, n.Kind)
	}
	if n.DeliveryID != "fest-judge:8f3c1a2b-4d5e-6f70-8192-a3b4c5d6e7f8" {
		t.Fatalf("delivery id = %q", n.DeliveryID)
	}
	if n.TargetSessionID != "ses_01J9ZX3ABCDEF" || n.CampaignID != "camp_01J9ZWXYZ0123" {
		t.Fatalf("identity = %q/%q", n.TargetSessionID, n.CampaignID)
	}
	wantMeta := map[string]string{
		"festival_id": "FE0001",
		"phase":       "001_IMPLEMENT",
		"step_number": "2",
		"judge_type":  "approval",
		"authority":   "authoritative",
		"verdict":     "approved",
	}
	for k, want := range wantMeta {
		if got := n.Metadata[k]; got != want {
			t.Fatalf("metadata[%q] = %q, want %q", k, got, want)
		}
	}
}

// TestBuildNotificationMatchesFixture locks the envelope structure against the
// frozen canonical fixture. The rendered message is producer-owned free text
// and is asserted by substring rather than byte equality.
func TestBuildNotificationMatchesFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/session-notification-v1.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture SessionNotification
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	got := BuildNotification(approvalResult())

	if got.SchemaVersion != fixture.SchemaVersion {
		t.Fatalf("schema %q != fixture %q", got.SchemaVersion, fixture.SchemaVersion)
	}
	if got.DeliveryID != fixture.DeliveryID {
		t.Fatalf("delivery id %q != fixture %q", got.DeliveryID, fixture.DeliveryID)
	}
	if got.TargetSessionID != fixture.TargetSessionID || got.CampaignID != fixture.CampaignID {
		t.Fatalf("identity mismatch with fixture")
	}
	if got.Source != fixture.Source || got.Kind != fixture.Kind {
		t.Fatalf("source/kind mismatch with fixture")
	}
	for k, want := range fixture.Metadata {
		if got.Metadata[k] != want {
			t.Fatalf("metadata[%q] = %q, fixture %q", k, got.Metadata[k], want)
		}
	}
	if !strings.Contains(got.Message, "Continue with: fest next") {
		t.Fatalf("message missing next command: %q", got.Message)
	}
}
