package activity_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/activity"
)

// TestIntegration_FestivalCreationEmitsOnBothFiles verifies that a
// festival.created event (DestBoth) is written to both the festival-level and
// campaign-level activity.jsonl files.
func TestIntegration_FestivalCreationEmitsOnBothFiles(t *testing.T) {
	campaignRoot := t.TempDir()
	campDir := filepath.Join(campaignRoot, ".campaign")
	if err := os.MkdirAll(campDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-int\nname: test\n"
	if err := os.WriteFile(filepath.Join(campDir, "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	festivalPath := filepath.Join(campaignRoot, "festivals", "active", "demo-fest-DF0001")
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}

	e := activity.NewFromFestival(context.Background(), festivalPath, func(err error) {
		t.Fatalf("unexpected warn: %v", err)
	})

	e.Emit(context.Background(), "festival.created", activity.Scope{},
		"fest create festival --name demo-fest",
		activity.WithData(map[string]any{
			"dest": "planning",
		}))

	// Both files should exist and contain the event.
	festFile := filepath.Join(festivalPath, ".fest", activity.FestivalFileName)
	campFile := filepath.Join(campaignRoot, ".campaign", "fest", activity.CampaignFileName)

	if !fileContains(t, festFile, "festival.created") {
		t.Fatal("festival-level file missing festival.created event")
	}
	if !fileContains(t, campFile, "festival.created") {
		t.Fatal("campaign-level file missing festival.created event")
	}

	// Verify schema version is present.
	festEvents := readJSONL(t, festFile)
	if len(festEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(festEvents))
	}
	if festEvents[0]["v"].(float64) != float64(activity.SchemaVersion) {
		t.Fatalf("expected v=%d, got %v", activity.SchemaVersion, festEvents[0]["v"])
	}
	if _, ok := festEvents[0]["actor"].(map[string]any); !ok {
		t.Fatal("event missing actor")
	}
	if _, ok := festEvents[0]["scope"].(map[string]any); !ok {
		t.Fatal("event missing scope")
	}
}

// TestIntegration_PhaseCreatedEmitsOnBothFiles verifies that a phase.created
// event (DestBoth) is written to both files with phase scope.
func TestIntegration_PhaseCreatedEmitsOnBothFiles(t *testing.T) {
	campaignRoot := t.TempDir()
	campDir := filepath.Join(campaignRoot, ".campaign")
	if err := os.MkdirAll(campDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-phase\nname: test\n"
	if err := os.WriteFile(filepath.Join(campDir, "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	festivalPath := filepath.Join(campaignRoot, "festivals", "active", "demo-fest-DF0002")
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}

	e := activity.NewFromFestival(context.Background(), festivalPath, func(err error) {
		t.Fatalf("unexpected warn: %v", err)
	})

	e.Emit(context.Background(), "phase.created", activity.Scope{Phase: "002_PLAN"},
		"fest create phase --name PLAN --type planning",
		activity.WithData(map[string]any{
			"phase_type":  "planning",
			"phase_order": 2,
		}))

	festFile := filepath.Join(festivalPath, ".fest", activity.FestivalFileName)
	campFile := filepath.Join(campaignRoot, ".campaign", "fest", activity.CampaignFileName)

	festEvents := readJSONL(t, festFile)
	if len(festEvents) != 1 {
		t.Fatalf("expected 1 festival event, got %d", len(festEvents))
	}
	campEvents := readJSONL(t, campFile)
	if len(campEvents) != 1 {
		t.Fatalf("expected 1 campaign event, got %d", len(campEvents))
	}

	// Verify scope.phase is set.
	scope := festEvents[0]["scope"].(map[string]any)
	if scope["phase"] != "002_PLAN" {
		t.Fatalf("expected scope.phase=002_PLAN, got %v", scope["phase"])
	}
}

// TestIntegration_TaskCreatedEmitsFestivalOnly verifies that a task.created
// event (DestFestivalOnly) is written ONLY to the festival file, not the
// campaign file.
func TestIntegration_TaskCreatedEmitsFestivalOnly(t *testing.T) {
	campaignRoot := t.TempDir()
	campDir := filepath.Join(campaignRoot, ".campaign")
	if err := os.MkdirAll(campDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-task\nname: test\n"
	if err := os.WriteFile(filepath.Join(campDir, "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	festivalPath := filepath.Join(campaignRoot, "festivals", "active", "demo-fest-DF0003")
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}

	e := activity.NewFromFestival(context.Background(), festivalPath, func(err error) {
		t.Fatalf("unexpected warn: %v", err)
	})

	e.Emit(context.Background(), "task.created", activity.Scope{Task: "01_setup.md"},
		"fest create task --name setup",
		activity.WithData(map[string]any{"tasks_created": []string{"01_setup.md"}}))

	festFile := filepath.Join(festivalPath, ".fest", activity.FestivalFileName)
	campFile := filepath.Join(campaignRoot, ".campaign", "fest", activity.CampaignFileName)

	// Festival file should have the event.
	if !fileContains(t, festFile, "task.created") {
		t.Fatal("festival-level file missing task.created event")
	}

	// Campaign file should NOT exist (festival-only event).
	if _, err := os.Stat(campFile); !os.IsNotExist(err) {
		t.Fatal("campaign-level file should not exist for festival-only events")
	}
}

// TestIntegration_PromotionEmitsOnBothFilesWithFromTo verifies that a
// festival.promoted event is emitted on both files with data.from/data.to.
func TestIntegration_PromotionEmitsOnBothFilesWithFromTo(t *testing.T) {
	campaignRoot := t.TempDir()
	campDir := filepath.Join(campaignRoot, ".campaign")
	if err := os.MkdirAll(campDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-promo\nname: test\n"
	if err := os.WriteFile(filepath.Join(campDir, "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	festivalPath := filepath.Join(campaignRoot, "festivals", "active", "demo-fest-DF0004")
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}

	e := activity.NewFromFestival(context.Background(), festivalPath, func(err error) {
		t.Fatalf("unexpected warn: %v", err)
	})

	e.Emit(context.Background(), "festival.promoted", activity.Scope{},
		"fest promote --to active",
		activity.WithData(map[string]any{
			"from": "planning",
			"to":   "active",
		}))

	campFile := filepath.Join(campaignRoot, ".campaign", "fest", activity.CampaignFileName)
	campEvents := readJSONL(t, campFile)
	if len(campEvents) != 1 {
		t.Fatalf("expected 1 campaign event, got %d", len(campEvents))
	}

	data := campEvents[0]["data"].(map[string]any)
	if data["from"] != "planning" {
		t.Fatalf("expected data.from=planning, got %v", data["from"])
	}
	if data["to"] != "active" {
		t.Fatalf("expected data.to=active, got %v", data["to"])
	}
}

// TestIntegration_ValidateRanEmitsFestivalOnly verifies that validate.ran
// is emitted on the festival file only, not the campaign file.
func TestIntegration_ValidateRanEmitsFestivalOnly(t *testing.T) {
	campaignRoot := t.TempDir()
	campDir := filepath.Join(campaignRoot, ".campaign")
	if err := os.MkdirAll(campDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-validate\nname: test\n"
	if err := os.WriteFile(filepath.Join(campDir, "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}

	festivalPath := filepath.Join(campaignRoot, "festivals", "active", "demo-fest-DF0005")
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}

	e := activity.NewFromFestival(context.Background(), festivalPath, func(err error) {
		t.Fatalf("unexpected warn: %v", err)
	})

	e.Emit(context.Background(), "validate.ran", activity.Scope{},
		"fest validate",
		activity.WithData(map[string]any{
			"ok":       true,
			"errors":   0,
			"warnings": 0,
		}))

	festFile := filepath.Join(festivalPath, ".fest", activity.FestivalFileName)
	campFile := filepath.Join(campaignRoot, ".campaign", "fest", activity.CampaignFileName)

	// Festival file should have the event.
	if !fileContains(t, festFile, "validate.ran") {
		t.Fatal("festival-level file missing validate.ran event")
	}

	// Campaign file should NOT exist (festival-only event).
	if _, err := os.Stat(campFile); !os.IsNotExist(err) {
		t.Fatal("campaign-level file should not exist for festival-only events")
	}
}

// TestIntegration_WorkflowSkipEmitsWithVerbatimReason verifies that
// workflow.skipped is emitted with the verbatim reason.
func TestIntegration_WorkflowSkipEmitsWithVerbatimReason(t *testing.T) {
	_, festivalPath := setupIntegrationCampaign(t, "test-campaign-wfskip")

	e := activity.NewFromFestival(context.Background(), festivalPath, func(err error) {
		t.Fatalf("unexpected warn: %v", err)
	})

	reason := "already completed externally"
	e.Emit(context.Background(), "workflow.skipped", activity.Scope{Phase: "003_IMPLEMENT"},
		`fest workflow skip --reason "`+reason+`"`,
		activity.WithData(map[string]any{
			"reason":         reason,
			"terminal_state": "skipped",
			"steps_skipped":  3,
		}))

	festFile := filepath.Join(festivalPath, ".fest", activity.FestivalFileName)
	data, err := os.ReadFile(festFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), reason) {
		t.Fatal("workflow.skipped should contain verbatim reason")
	}
}

// TestIntegration_WithErrorSetsResultFalse verifies the WithError option API.
// Command packages emit only after a successful mutation; this is not a
// production fail-path wiring test.
func TestIntegration_WithErrorSetsResultFalse(t *testing.T) {
	_, festivalPath := setupIntegrationCampaign(t, "test-campaign-fail")

	e := activity.NewFromFestival(context.Background(), festivalPath, func(err error) {
		t.Fatalf("unexpected warn: %v", err)
	})

	e.Emit(context.Background(), "validate.ran", activity.Scope{},
		"fest validate",
		activity.WithData(map[string]any{"ok": false, "errors": []string{"missing file"}}),
		activity.WithError(errWrap("validation failed: missing file")))

	festFile := filepath.Join(festivalPath, ".fest", activity.FestivalFileName)
	events := readJSONL(t, festFile)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	result := events[0]["result"].(map[string]any)
	if result["ok"] != false {
		t.Fatal("expected result.ok = false when WithError is applied")
	}
	if result["error"] == nil || result["error"] == "" {
		t.Fatal("expected non-empty error message")
	}
}

// TestIntegration_ConcurrentWritersNoInterleaving verifies that concurrent
// writes from multiple goroutines do not interleave (file lock test).
func TestIntegration_ConcurrentWritersNoInterleaving(t *testing.T) {
	_, festivalPath := setupIntegrationCampaign(t, "test-campaign-concurrent")

	const numWriters = 10
	const eventsPerWriter = 20
	done := make(chan struct{}, numWriters)

	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			e := activity.NewFromFestival(context.Background(), festivalPath, func(error) {})
			for j := 0; j < eventsPerWriter; j++ {
				e.Emit(context.Background(), "task.completed", activity.Scope{Task: "01_task.md"},
					"fest task completed", activity.WithData(map[string]any{"writer": writerID}))
			}
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < numWriters; i++ {
		<-done
	}

	festFile := filepath.Join(festivalPath, ".fest", activity.FestivalFileName)
	events := readJSONL(t, festFile)
	expected := numWriters * eventsPerWriter
	if len(events) != expected {
		t.Fatalf("expected %d events, got %d", expected, len(events))
	}
}

// Helpers

func setupIntegrationCampaign(t *testing.T, campaignID string) (campaignRoot, festivalPath string) {
	t.Helper()
	campaignRoot = t.TempDir()
	campDir := filepath.Join(campaignRoot, ".campaign")
	if err := os.MkdirAll(campDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: " + campaignID + "\nname: test\n"
	if err := os.WriteFile(filepath.Join(campDir, "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	festivalPath = filepath.Join(campaignRoot, "festivals", "active", "demo-fest-DF0001")
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}
	return campaignRoot, festivalPath
}

func fileContains(t *testing.T, path, substr string) bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return strings.Contains(string(data), substr)
}

func readJSONL(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var events []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			t.Fatalf("unmarshaling line: %v\nline: %s", err, line)
		}
		events = append(events, ev)
	}
	return events
}

type testErr string

func (e testErr) Error() string { return string(e) }

func errWrap(msg string) error { return testErr(msg) }
