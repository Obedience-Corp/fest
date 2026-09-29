package validator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/progress"
)

const deferralTaskID = "001_PHASE/01_seq/01_cache_warmup.md"

// writeDeferralStore seeds the event log directly. The corrupt states below are
// unreachable through the verbs, which is the point of the rule, so the fixture
// writes the events a hand edit would write rather than calling a verb.
func writeDeferralStore(t *testing.T, events ...map[string]any) string {
	t.Helper()
	festivalPath := t.TempDir()

	seqDir := filepath.Join(festivalPath, "001_PHASE", "01_seq")
	if err := os.MkdirAll(seqDir, 0o755); err != nil {
		t.Fatalf("mkdir sequence: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(festivalPath, progress.ProgressDir), 0o755); err != nil {
		t.Fatalf("mkdir .fest: %v", err)
	}

	var lines []string
	for _, event := range events {
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatalf("marshal event: %v", err)
		}
		lines = append(lines, string(encoded))
	}
	path := filepath.Join(festivalPath, progress.ProgressDir, progress.ProgressEventsFile)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
	return festivalPath
}

func stamp(minutesAgo int) string {
	return time.Now().UTC().Add(-time.Duration(minutesAgo) * time.Minute).Format(time.RFC3339Nano)
}

func blockedEventMap() map[string]any {
	return map[string]any{
		"ts": stamp(30), "event": string(progress.EventBlocked),
		"task": deferralTaskID, "reason": "provider API removed",
	}
}

func deferredEventMap(reason string) map[string]any {
	event := map[string]any{
		"ts": stamp(20), "event": string(progress.EventBlockerDeferred),
		"task": deferralTaskID, "reason": "provider API removed",
		"deferred_by": "Ada Lovelace", "actor": "operator", "tty": true,
	}
	if reason != "" {
		event["deferral_reason"] = reason
	}
	return event
}

func codesIn(issues []Issue) []string {
	var codes []string
	for _, issue := range issues {
		codes = append(codes, issue.Code)
	}
	return codes
}

// A deferral on a task that is no longer blocked. A started event after the
// deferral moves the status without clearing the flag, which no verb does:
// unblock, complete and reset all call clearDeferral on the way past.
func TestValidateDeferralsReportsADeferralOnANonBlockedTask(t *testing.T) {
	festivalPath := writeDeferralStore(t,
		blockedEventMap(),
		deferredEventMap("the vendor replies next week"),
		map[string]any{
			"ts": stamp(10), "event": string(progress.EventStarted),
			"task": deferralTaskID,
		},
	)

	issues, err := ValidateDeferrals(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("ValidateDeferrals() error = %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("issues = %v, want exactly the non-blocked finding", codesIn(issues))
	}
	if issues[0].Code != CodeDeferralNotBlocked {
		t.Errorf("Code = %q, want %q", issues[0].Code, CodeDeferralNotBlocked)
	}
	if issues[0].Level != LevelError {
		t.Errorf("Level = %q, want %q for a store inconsistency", issues[0].Level, LevelError)
	}
	if !strings.Contains(issues[0].Message, "in_progress") {
		t.Errorf("Message = %q, want it to name the actual status", issues[0].Message)
	}
	if issues[0].Path != filepath.FromSlash(deferralTaskID) {
		t.Errorf("Path = %q, want the task ID", issues[0].Path)
	}
}

func TestValidateDeferralsReportsADeferralWithNoReason(t *testing.T) {
	festivalPath := writeDeferralStore(t, blockedEventMap(), deferredEventMap(""))

	issues, err := ValidateDeferrals(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("ValidateDeferrals() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Code != CodeDeferralNoReason {
		t.Fatalf("issues = %v, want exactly the missing-reason finding", codesIn(issues))
	}
	if issues[0].Level != LevelError {
		t.Errorf("Level = %q, want %q", issues[0].Level, LevelError)
	}
}

// The test that stops the rule firing on every healthy festival.
func TestValidateDeferralsIsSilentOnAHealthyDeferral(t *testing.T) {
	festivalPath := writeDeferralStore(t, blockedEventMap(), deferredEventMap("the vendor replies next week"))

	issues, err := ValidateDeferrals(t.Context(), festivalPath)
	if err != nil {
		t.Fatalf("ValidateDeferrals() error = %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("issues = %v, want none for a blocked, deferred task with a reason", codesIn(issues))
	}
}

func TestValidateDeferralsIsSilentWithNoStore(t *testing.T) {
	issues, err := ValidateDeferrals(t.Context(), t.TempDir())
	if err != nil {
		t.Fatalf("ValidateDeferrals() error = %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("issues = %v, want none for a festival with no progress store", codesIn(issues))
	}
}
