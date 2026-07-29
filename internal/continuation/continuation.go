// Package continuation renders and submits judge-continuation notifications
// that resume the originating Obey session after a terminal judge verdict.
//
// The package is a strictly additive producer: when no Obey session identity is
// present in the environment it yields no continuation context, and standalone
// Fest keeps its existing behavior. Identity is captured once at judge launch
// and pinned into the detached judge payload; the detached runner reads it from
// the payload and never re-reads a possibly changed environment.
package continuation

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const (
	// EnvSessionID and EnvCampaignID carry the originating Obey session
	// identity into the agent process environment. Obey injects both; either
	// missing disables continuation delivery.
	EnvSessionID  = "OBEY_SESSION_ID"
	EnvCampaignID = "OBEY_CAMPAIGN_ID"

	// NotificationSchema is the wire schema for the envelope sent to
	// 'obey session notify'.
	NotificationSchema = "obey.session.notification/v1"
	// NotificationSource identifies Fest as the producer.
	NotificationSource = "fest"
	// NotificationKind routes the notification as a workflow judge result.
	NotificationKind = "workflow.judge"

	// JudgeType is the judge-kind discriminator. The approval judge sends
	// "approval"; consumers treat a missing value as "approval".
	JudgeType = "approval"
	// Authority marks the approval judge verdict as authoritative rather than
	// advisory.
	Authority = "authoritative"

	// DeliveryIDPrefix namespaces the deterministic delivery id derived from
	// the judge run id.
	DeliveryIDPrefix = "fest-judge:"

	// maxIdentifierLen bounds an accepted session or campaign identifier.
	maxIdentifierLen = 256
	// maxFeedbackLen bounds rendered feedback so a malformed or oversized
	// judge reason cannot produce an unbounded agent message.
	maxFeedbackLen = 500
	// maxFollowups bounds the number of judge fix items rendered into a
	// continuation message so a malformed verdict cannot make it unbounded.
	maxFollowups = 12
)

// Verdict is the terminal judge outcome carried at the transport layer.
type Verdict string

const (
	VerdictApproved Verdict = "approved"
	VerdictRejected Verdict = "rejected"
	VerdictFailed   Verdict = "failed"
)

// Context is the resolved originating Obey session identity. It is empty and
// unused unless Resolve reports a present, valid identity.
type Context struct {
	SessionID  string
	CampaignID string
}

// Resolve reads the Obey session identity once through lookup (os.LookupEnv in
// production). It returns a Context and true only when both the session id and
// the campaign id are present and are safe identifiers. A missing or malformed
// identity returns false, which disables continuation delivery and preserves
// the existing manual-fallback behavior.
func Resolve(lookup func(string) (string, bool)) (Context, bool) {
	sessionID, ok := lookup(EnvSessionID)
	if !ok {
		return Context{}, false
	}
	sessionID = strings.TrimSpace(sessionID)
	if !validIdentifier(sessionID) {
		return Context{}, false
	}
	campaignID, ok := lookup(EnvCampaignID)
	if !ok {
		return Context{}, false
	}
	campaignID = strings.TrimSpace(campaignID)
	if !validIdentifier(campaignID) {
		return Context{}, false
	}
	return Context{SessionID: sessionID, CampaignID: campaignID}, true
}

// validIdentifier reports whether s is a non-empty, bounded identifier drawn
// from a conservative character set. Obey session and campaign ids fit this
// set; the check fails closed on whitespace, control, or shell-significant
// characters even though the envelope travels over stdin rather than a shell.
func validIdentifier(s string) bool {
	if s == "" || len(s) > maxIdentifierLen {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '_' || r == '-' || r == '.' || r == ':':
		default:
			return false
		}
	}
	return true
}

// SessionNotification is the obey.session.notification/v1 envelope submitted to
// 'obey session notify' over stdin.
type SessionNotification struct {
	SchemaVersion   string            `json:"schema_version"`
	DeliveryID      string            `json:"delivery_id"`
	TargetSessionID string            `json:"target_session_id"`
	CampaignID      string            `json:"campaign_id"`
	Source          string            `json:"source"`
	Kind            string            `json:"kind"`
	Message         string            `json:"message"`
	Metadata        map[string]string `json:"metadata"`
}

// JudgeResult carries the fields needed to render a continuation notification
// for one terminal judge run. The advisory next command is derived from Verdict
// rather than supplied, so a rendered message can never disagree with the
// verdict about which command is valid next.
type JudgeResult struct {
	RunID         string
	TargetSession string
	CampaignID    string
	FestivalID    string
	FestivalName  string
	Phase         string
	StepNumber    int
	StepName      string
	Verdict       Verdict
	Feedback      string
	// Followups carries the judge's itemized fixes for a rejection so the
	// resumed session receives the concrete revise list, not just the reason.
	Followups []string
}

// DeliveryID derives the deterministic, retry-stable delivery id for a run.
func DeliveryID(runID string) string {
	return DeliveryIDPrefix + runID
}

// NextCommand returns the valid next Fest command for a verdict. It is advisory
// text generated from the verdict, never a command Fest executes on the agent's
// behalf.
func NextCommand(v Verdict) string {
	if v == VerdictApproved {
		return "fest next"
	}
	return "fest workflow judge"
}

// RenderMessage renders the agent-facing continuation message. Approval omits
// feedback and points at the next workflow command; rejection and failure carry
// concise, bounded feedback and end with the rejudge command.
func RenderMessage(r JudgeResult) string {
	loc := fmt.Sprintf("step %d (%s) in %s", r.StepNumber, r.StepName, r.FestivalName)
	next := NextCommand(r.Verdict)
	switch r.Verdict {
	case VerdictApproved:
		return fmt.Sprintf("Approval judge approved %s.\nContinue with: %s", loc, next)
	case VerdictRejected:
		return fmt.Sprintf("Approval judge rejected %s.\nFeedback: %s%s\nAddress the feedback, then run: %s",
			loc, sanitizeFeedback(r.Feedback), renderFollowups(r.Followups), next)
	case VerdictFailed:
		return fmt.Sprintf("Approval judge failed for %s.\nFeedback: %s\nInspect the judge configuration or evidence, then run: %s",
			loc, sanitizeFeedback(r.Feedback), next)
	default:
		return fmt.Sprintf("Approval judge returned %s for %s.\nRun: %s", r.Verdict, loc, next)
	}
}

// BuildNotification constructs the obey.session.notification/v1 envelope for a
// terminal judge run.
func BuildNotification(r JudgeResult) SessionNotification {
	return SessionNotification{
		SchemaVersion:   NotificationSchema,
		DeliveryID:      DeliveryID(r.RunID),
		TargetSessionID: r.TargetSession,
		CampaignID:      r.CampaignID,
		Source:          NotificationSource,
		Kind:            NotificationKind,
		Message:         RenderMessage(r),
		Metadata: map[string]string{
			"festival_id": r.FestivalID,
			"phase":       r.Phase,
			"step_number": strconv.Itoa(r.StepNumber),
			"judge_type":  JudgeType,
			"authority":   Authority,
			"verdict":     string(r.Verdict),
		},
	}
}

// renderFollowups formats the judge's itemized fixes as an indented list
// appended after the feedback line. It returns "" when there are no usable
// followups so the rejection message is unchanged for a reason-only verdict.
// Each item is sanitized and bounded, and the count is capped, so a malformed
// verdict cannot produce an unbounded message.
func renderFollowups(followups []string) string {
	if len(followups) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nFixes required:")
	n := 0
	for _, f := range followups {
		item := sanitizeFeedback(f)
		if item == "(no detail provided)" {
			continue
		}
		b.WriteString("\n- ")
		b.WriteString(item)
		n++
		if n >= maxFollowups {
			break
		}
	}
	if n == 0 {
		return ""
	}
	return b.String()
}

// sanitizeFeedback collapses whitespace, strips control characters, and bounds
// the length of judge feedback so the rendered message stays single-purpose and
// cannot be pushed off screen or made unbounded by a hostile or broken judge.
func sanitizeFeedback(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
	collapsed := strings.Join(strings.Fields(cleaned), " ")
	if collapsed == "" {
		return "(no detail provided)"
	}
	runes := []rune(collapsed)
	if len(runes) > maxFeedbackLen {
		return strings.TrimSpace(string(runes[:maxFeedbackLen])) + "..."
	}
	return collapsed
}
