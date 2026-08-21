package workflow

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

func TestNormalizeEvidenceStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: ""},
		{name: "none", in: "none", want: evidenceStatusNone},
		{name: "upper_none", in: "NONE", want: evidenceStatusNone},
		{name: "insufficient_padded", in: " insufficient ", want: evidenceStatusInsufficient},
		{name: "ok", in: "ok", want: evidenceStatusOK},
		{name: "unknown", in: "received", want: ""},
		{name: "audit_only_unacknowledged", in: "unacknowledged", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeEvidenceStatus(tt.in); got != tt.want {
				t.Fatalf("normalizeEvidenceStatus(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestReportJudgeEvidenceStatus(t *testing.T) {
	t.Parallel()
	reqWithEvidence := approvalJudgeRequest{Evidence: []string{"output_specs/PRESENTATION.md"}}

	t.Run("omitted_status_with_evidence_warns", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		resp := &approvalJudgeResponse{Decision: "reject", Reason: "blind"}
		if err := reportJudgeEvidenceStatus(&buf, reqWithEvidence, resp); err != nil {
			t.Fatalf("warn path must not fail closed: %v", err)
		}
		if !strings.Contains(buf.String(), "did not report evidence_status") {
			t.Fatalf("missing stale-binary warning: %q", buf.String())
		}
	})

	t.Run("none_approve_with_evidence_fails_closed", func(t *testing.T) {
		t.Parallel()
		resp := &approvalJudgeResponse{Decision: "approve", Reason: "ok", EvidenceStatus: evidenceStatusNone}
		err := reportJudgeEvidenceStatus(&bytes.Buffer{}, reqWithEvidence, resp)
		if err == nil {
			t.Fatal("expected fail-closed error")
		}
		if !strings.Contains(err.Error(), "approved without reading the delivered evidence") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("none_reject_with_evidence_warns", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		resp := &approvalJudgeResponse{Decision: "reject", Reason: "cannot see files", EvidenceStatus: evidenceStatusNone}
		if err := reportJudgeEvidenceStatus(&buf, reqWithEvidence, resp); err != nil {
			t.Fatalf("reject with none must still record: %v", err)
		}
		if !strings.Contains(buf.String(), "evidence_status=none") {
			t.Fatalf("missing none warning: %q", buf.String())
		}
	})

	t.Run("ok_approve_is_silent", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		resp := &approvalJudgeResponse{Decision: "approve", Reason: "ok", EvidenceStatus: evidenceStatusOK}
		if err := reportJudgeEvidenceStatus(&buf, reqWithEvidence, resp); err != nil {
			t.Fatalf("ok approve: %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %q", buf.String())
		}
	})

	t.Run("no_evidence_sent_is_silent", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		resp := &approvalJudgeResponse{Decision: "approve", Reason: "ok"}
		if err := reportJudgeEvidenceStatus(&buf, approvalJudgeRequest{}, resp); err != nil {
			t.Fatalf("docs-only approve: %v", err)
		}
		if buf.Len() != 0 {
			t.Fatalf("unexpected warning: %q", buf.String())
		}
	})

	t.Run("nil_response", func(t *testing.T) {
		t.Parallel()
		if err := reportJudgeEvidenceStatus(&bytes.Buffer{}, reqWithEvidence, nil); err != nil {
			t.Fatalf("nil resp: %v", err)
		}
	})
}

func TestEvaluateApprovalJudge_EvidenceStatusInAudit(t *testing.T) {
	withApprovalJudgeRunner(t, judgeRunner(func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"reject","reason":"deliverable is a stub","evidence_status":"insufficient"}`), nil
	}))

	req := testApprovalJudgeRequest()
	req.Evidence = []string{"output_specs/PRESENTATION.md"}
	decision, audit, err := evaluateApprovalJudge(context.Background(), req, approvalJudgeOptions{
		JudgeCommand: "ob judge",
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("evaluateApprovalJudge: %v", err)
	}
	if decision.EvidenceStatus != evidenceStatusInsufficient {
		t.Fatalf("evidence_status = %q, want insufficient", decision.EvidenceStatus)
	}
	if !strings.Contains(audit, "evidence_status=insufficient") {
		t.Fatalf("audit missing structured status: %q", audit)
	}
	idxStatus := strings.Index(audit, "evidence_status=")
	idxReason := strings.Index(audit, " reason=")
	if idxStatus < 0 || idxReason < 0 || idxStatus > idxReason {
		t.Fatalf("evidence_status must precede reason= so DisplayFeedback can unquote: %q", audit)
	}
}

func TestEvaluateApprovalJudge_NoneApproveFailsClosed(t *testing.T) {
	withApprovalJudgeRunner(t, judgeRunner(func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"looks fine","evidence_status":"none"}`), nil
	}))

	req := testApprovalJudgeRequest()
	req.Evidence = []string{"output_specs/PRESENTATION.md"}
	_, _, err := evaluateApprovalJudge(context.Background(), req, approvalJudgeOptions{
		JudgeCommand: "ob judge",
		Timeout:      time.Second,
	})
	if err == nil {
		t.Fatal("expected fail-closed error")
	}
	if !strings.Contains(err.Error(), "approved without reading the delivered evidence") {
		t.Fatalf("error = %v", err)
	}
}

func TestEvaluateApprovalJudge_OmittedStatusStillAccepts(t *testing.T) {
	withApprovalJudgeRunner(t, judgeRunner(func(ctx context.Context, command string, stdin []byte) ([]byte, error) {
		return []byte(`{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"ok"}`), nil
	}))

	req := testApprovalJudgeRequest()
	req.Evidence = []string{"output_specs/PRESENTATION.md"}
	decision, audit, err := evaluateApprovalJudge(context.Background(), req, approvalJudgeOptions{
		JudgeCommand: "legacy-judge",
		Timeout:      time.Second,
	})
	if err != nil {
		t.Fatalf("omitted status must stay additive: %v", err)
	}
	if decision.EvidenceStatus != "" {
		t.Fatalf("omitted status = %q, want empty", decision.EvidenceStatus)
	}
	if !strings.Contains(audit, "evidence_status=unacknowledged") {
		t.Fatalf("audit must mark unacknowledged: %q", audit)
	}
}

func TestApprovalJudgeAuditPlacesStatusBeforeReason(t *testing.T) {
	got := approvalJudgeAudit("ob judge", &approvalJudgeResponse{
		Decision:       "reject",
		Reason:         `the note says reason=needs more evidence`,
		EvidenceStatus: evidenceStatusInsufficient,
	})
	want := `approval auto mode: schema_version=fest.approval.judge/v1 judge_command="ob judge" decision=reject evidence_status=insufficient reason="the note says reason=needs more evidence"`
	if got != want {
		t.Fatalf("audit = %q, want %q", got, want)
	}
}
