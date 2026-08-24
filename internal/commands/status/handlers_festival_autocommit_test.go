package status

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	ferrors "github.com/Obedience-Corp/fest/internal/errors"
)

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// setupStatusCampaignWithAutoCommitPolicy creates a festivals tree with
// agent.require_auto_commit set at workspace level and a planning festival.
func setupStatusCampaignWithAutoCommitPolicy(t *testing.T, requireAutoCommit bool) (festivalsDir, festPath string) {
	t.Helper()
	root := t.TempDir()
	festivalsDir = filepath.Join(root, "festivals")
	dotFestivalDir := filepath.Join(festivalsDir, ".festival")
	if err := os.MkdirAll(dotFestivalDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgData := "version: \"1.0\"\nagent:\n  require_auto_commit: " + boolStr(requireAutoCommit) + "\n"
	if err := os.WriteFile(filepath.Join(dotFestivalDir, "config.yaml"), []byte(cfgData), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"planning", "ready", "active"} {
		if err := os.MkdirAll(filepath.Join(festivalsDir, status), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	festPath = filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001")
	if err := os.MkdirAll(festPath, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "version: \"1.0\"\nmetadata:\n  id: FE0001\n  status_history:\n    - status: planning\n"
	if err := os.WriteFile(filepath.Join(festPath, "fest.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festPath, "FESTIVAL_GOAL.md"), []byte("# Goal\nTest goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return festivalsDir, festPath
}

func planningFestival(festPath string) *show.FestivalInfo {
	return &show.FestivalInfo{
		Name:   "alpha-feature-FE0001",
		Path:   festPath,
		Status: "planning",
	}
}

func captureStatusStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	done := make(chan struct{})
	var buf bytes.Buffer
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fnErr := fn()
	_ = w.Close()
	<-done
	return buf.String(), fnErr
}

func assertFestivalUnmoved(t *testing.T, festivalsDir string) {
	t.Helper()
	oldPath := filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001")
	if _, err := os.Stat(oldPath); err != nil {
		t.Fatalf("festival should remain in planning (not moved), stat err: %v", err)
	}
	movedPath := filepath.Join(festivalsDir, "ready", "alpha-feature-FE0001")
	if _, err := os.Stat(movedPath); !os.IsNotExist(err) {
		t.Fatal("festival must not have been moved when --no-commit was rejected")
	}
}

// TestExecuteFestivalMove_NoCommitRejectedWhenPolicyRequiresAutoCommit verifies
// that --no-commit is rejected before any filesystem mutation when
// agent.require_auto_commit is true.
func TestExecuteFestivalMove_NoCommitRejectedWhenPolicyRequiresAutoCommit(t *testing.T) {
	festivalsDir, festPath := setupStatusCampaignWithAutoCommitPolicy(t, true)

	err := executeFestivalMove(t.Context(), planningFestival(festPath), "ready", &statusOptions{noCommit: true})
	if err == nil {
		t.Fatal("expected error when --no-commit is used with require_auto_commit policy, got nil")
	}
	if !strings.Contains(err.Error(), "auto-commit is required by policy") {
		t.Fatalf("error should mention auto-commit policy, got: %v", err)
	}
	if strings.Contains(err.Error(), "to disable this guard") {
		t.Fatalf("hint must not tell operators to set require_auto_commit to disable the guard, got: %v", err)
	}
	if !strings.Contains(err.Error(), "remove --no-commit") {
		t.Fatalf("hint should tell operators to remove --no-commit, got: %v", err)
	}

	assertFestivalUnmoved(t, festivalsDir)
}

// TestExecuteFestivalMove_NoCommitRejectedJSONExitsNonZero verifies that
// --no-commit rejection under --json produces a non-zero exit with a JSON error body.
func TestExecuteFestivalMove_NoCommitRejectedJSONExitsNonZero(t *testing.T) {
	festivalsDir, festPath := setupStatusCampaignWithAutoCommitPolicy(t, true)

	out, err := captureStatusStdout(t, func() error {
		return executeFestivalMove(t.Context(), planningFestival(festPath), "ready", &statusOptions{noCommit: true, json: true})
	})

	if !errors.Is(err, ferrors.ErrAlreadyPrinted) {
		t.Fatalf("--no-commit rejection under --json must return ErrAlreadyPrinted, got %v", err)
	}
	var body map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &body); jsonErr != nil {
		t.Fatalf("stdout must be parseable JSON, got %q (err: %v)", out, jsonErr)
	}
	if body["success"] != false {
		t.Fatalf("JSON body should carry success=false, got %v", body["success"])
	}
	if body["error"] == nil || body["error"] == "" {
		t.Fatalf("JSON body should carry a non-empty error, got %v", body["error"])
	}
	hint, _ := body["hint"].(string)
	if !strings.Contains(hint, "remove --no-commit") {
		t.Fatalf("JSON hint should tell operators to remove --no-commit, got: %q", hint)
	}
	if strings.Contains(hint, "to disable this guard") {
		t.Fatalf("JSON hint must not tell operators to set require_auto_commit to disable the guard, got: %q", hint)
	}

	assertFestivalUnmoved(t, festivalsDir)
}

// TestExecuteFestivalMove_NoCommitAllowedWhenPolicyNotSet verifies that
// --no-commit still works (backward compat) when require_auto_commit is not enabled.
func TestExecuteFestivalMove_NoCommitAllowedWhenPolicyNotSet(t *testing.T) {
	festivalsDir, festPath := setupStatusCampaignWithAutoCommitPolicy(t, false)

	err := executeFestivalMove(t.Context(), planningFestival(festPath), "ready", &statusOptions{noCommit: true})
	if err != nil {
		t.Fatalf("--no-commit should succeed when policy does not require auto-commit, got: %v", err)
	}

	movedPath := filepath.Join(festivalsDir, "ready", "alpha-feature-FE0001")
	if _, statErr := os.Stat(movedPath); statErr != nil {
		t.Fatalf("festival should have been moved to ready: %v", statErr)
	}
	oldPath := filepath.Join(festivalsDir, "planning", "alpha-feature-FE0001")
	if _, statErr := os.Stat(oldPath); !os.IsNotExist(statErr) {
		t.Fatalf("planning copy should be gone after move, stat err=%v", statErr)
	}
}

// TestExecuteFestivalMove_NoCommitAbortedWhenPolicyConfigUnreadable verifies
// that a malformed workspace config.yaml does not fail-open to honoring --no-commit.
func TestExecuteFestivalMove_NoCommitAbortedWhenPolicyConfigUnreadable(t *testing.T) {
	festivalsDir, festPath := setupStatusCampaignWithAutoCommitPolicy(t, true)
	cfgPath := filepath.Join(festivalsDir, ".festival", "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("version: \"1.0\"\nagent: [not, a, map]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := executeFestivalMove(t.Context(), planningFestival(festPath), "ready", &statusOptions{noCommit: true})
	if err == nil {
		t.Fatal("expected error when auto-commit policy config is unreadable, got nil")
	}
	if !strings.Contains(err.Error(), "loading auto-commit policy") {
		t.Fatalf("error should mention policy load, got: %v", err)
	}

	assertFestivalUnmoved(t, festivalsDir)
}
