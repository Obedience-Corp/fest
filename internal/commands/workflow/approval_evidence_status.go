package workflow

import (
	"fmt"
	"io"
	"strings"

	festerrors "github.com/Obedience-Corp/fest/internal/errors"
)

// Additive fest.approval.judge/v1 verdict field. An older judge omits it.
const (
	evidenceStatusNone           = "none"
	evidenceStatusInsufficient   = "insufficient"
	evidenceStatusOK             = "ok"
	evidenceStatusUnacknowledged = "unacknowledged"
)

func normalizeEvidenceStatus(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case evidenceStatusNone, evidenceStatusInsufficient, evidenceStatusOK:
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

// reportJudgeEvidenceStatus records what the judge said about deliverables.
//
// When fest sent evidence and the verdict omits evidence_status, this is the
// stale-binary signal: an older judge ignored the evidence field. Warn; do
// not fail closed, so custom judges keep working until they add the field.
//
// When fest sent evidence and the judge approves with evidence_status=none,
// fail closed. A judge that attests it loaded nothing cannot verify the
// claim, and an unverifiable claim must not approve.
func reportJudgeEvidenceStatus(w io.Writer, req approvalJudgeRequest, resp *approvalJudgeResponse) error {
	if resp == nil {
		return nil
	}
	resp.EvidenceStatus = normalizeEvidenceStatus(resp.EvidenceStatus)
	if len(req.Evidence) == 0 {
		return nil
	}

	switch resp.EvidenceStatus {
	case "":
		_, _ = fmt.Fprintln(w, "fest: approval judge did not report evidence_status; a binary older than the evidence field cannot see deliverables and will reject or approve blindly. Upgrade the judge (ob judge) so the ledger can distinguish no evidence from insufficient evidence.")
		return nil
	case evidenceStatusNone:
		if resp.Decision == "approve" {
			return festerrors.Validation("approval judge approved without reading the delivered evidence").
				WithField("evidence_status", evidenceStatusNone).
				WithField("evidence_count", len(req.Evidence)).
				WithHint("a judge that reports evidence_status=none cannot approve; upgrade the judge binary or reject until it can read the evidence manifest")
		}
		_, _ = fmt.Fprintf(w, "fest: approval judge reported evidence_status=none but the request listed %d deliverable(s); if this was a stale binary, upgrade ob judge.\n", len(req.Evidence))
		return nil
	default:
		return nil
	}
}
