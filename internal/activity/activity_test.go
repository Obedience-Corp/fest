package activity

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupCampaign(t *testing.T, campaignID string) (campaignRoot, festivalPath string) {
	t.Helper()
	campaignRoot = t.TempDir()
	campDir := filepath.Join(campaignRoot, ".campaign")
	if err := os.MkdirAll(campDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if campaignID != "" {
		yaml := "id: " + campaignID + "\nname: test\n"
		if err := os.WriteFile(filepath.Join(campDir, "campaign.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	festivalPath = filepath.Join(campaignRoot, "festivals", "active", "demo-fest-DF0001")
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}
	return campaignRoot, festivalPath
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

func TestEmitter_FestivalLevelOnly(t *testing.T) {
	campaignRoot, festivalPath := setupCampaign(t, "test-campaign")
	e := NewFromFestival(context.Background(), festivalPath, func(err error) { t.Fatalf("unexpected warn: %v", err) })

	e.Emit(context.Background(), "task.created", Scope{}, "fest create task --name setup",
		WithData(map[string]any{"task": "01_setup.md"}))

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	events := readJSONL(t, festFile)
	if len(events) != 1 {
		t.Fatalf("expected 1 festival event, got %d", len(events))
	}
	if events[0]["event"] != "task.created" {
		t.Fatalf("expected event 'task.created', got %v", events[0]["event"])
	}
	if events[0]["v"].(float64) != float64(SchemaVersion) {
		t.Fatalf("expected v=%d, got %v", SchemaVersion, events[0]["v"])
	}

	// task.created is DestFestivalOnly — campaign file should NOT exist.
	campFile := filepath.Join(campaignRoot, ".campaign", "fest", CampaignFileName)
	if _, err := os.Stat(campFile); !os.IsNotExist(err) {
		t.Fatal("campaign-level file should not exist for festival-only events")
	}
}

func TestEmitter_DualEmission(t *testing.T) {
	campaignRoot, festivalPath := setupCampaign(t, "test-campaign")
	e := NewFromFestival(context.Background(), festivalPath, func(err error) { t.Fatalf("unexpected warn: %v", err) })

	e.Emit(context.Background(), "phase.created", Scope{Phase: "002_PLAN"}, "fest create phase --name PLAN --type planning",
		WithData(map[string]any{"phase_type": "planning"}))

	// Both files should have the event.
	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	campFile := filepath.Join(campaignRoot, ".campaign", "fest", CampaignFileName)

	festEvents := readJSONL(t, festFile)
	if len(festEvents) != 1 {
		t.Fatalf("expected 1 festival event, got %d", len(festEvents))
	}
	campEvents := readJSONL(t, campFile)
	if len(campEvents) != 1 {
		t.Fatalf("expected 1 campaign event, got %d", len(campEvents))
	}

	// Verify festival.promoted data.from/data.to pattern works too.
	e.Emit(context.Background(), "festival.promoted", Scope{}, "fest promote",
		WithData(map[string]any{"from": "planning", "to": "active"}))

	campEvents = readJSONL(t, campFile)
	if len(campEvents) != 2 {
		t.Fatalf("expected 2 campaign events, got %d", len(campEvents))
	}
}

func TestEmitter_WithErrorOption(t *testing.T) {
	// WithError is the Option API for future fail-path logging. Command
	// packages in this PR emit only on success; this test does not claim
	// production error-path wiring.
	_, festivalPath := setupCampaign(t, "test-campaign")
	e := NewFromFestival(context.Background(), festivalPath, func(err error) { t.Fatalf("unexpected warn: %v", err) })

	e.Emit(context.Background(), "validate.ran", Scope{}, "fest validate",
		WithData(map[string]any{"ok": false, "errors": []string{"missing file"}}),
		WithError(errFake("validation failed")))

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
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

func TestEmitter_RotatesAndStartsFresh(t *testing.T) {
	_, festivalPath := setupCampaign(t, "test-campaign")
	e := NewFromFestival(context.Background(), festivalPath, func(err error) { t.Fatalf("unexpected warn: %v", err) })

	e.Emit(context.Background(), "task.created", Scope{Task: "01_setup.md"},
		"fest create task --name setup", WithData(map[string]any{"n": 0}))

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	if err := os.Truncate(festFile, RotationThreshold+1); err != nil {
		t.Fatalf("forcing size past RotationThreshold: %v", err)
	}

	e.Emit(context.Background(), "task.completed", Scope{Task: "01_setup.md"},
		"fest task completed", WithData(map[string]any{"marker": "post-rotate"}))

	if _, err := os.Stat(festFile); err != nil {
		t.Fatalf("canonical activity.jsonl missing after rotation: %v", err)
	}

	events := readJSONL(t, festFile)
	if len(events) != 1 {
		t.Fatalf("canonical path should contain only post-rotate events, got %d", len(events))
	}
	if events[0]["event"] != "task.completed" {
		t.Fatalf("expected post-rotate event 'task.completed', got %v", events[0]["event"])
	}
	data, _ := events[0]["data"].(map[string]any)
	if data["marker"] != "post-rotate" {
		t.Fatalf("expected data.marker=post-rotate, got %v", data["marker"])
	}

	rotated := filepath.Join(festivalPath, ".fest", "activity.1.jsonl")
	if _, err := os.Stat(rotated); err != nil {
		t.Fatalf("expected rotated archive %s: %v", rotated, err)
	}
	rotData, err := os.ReadFile(rotated)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rotData), "task.created") {
		t.Fatal("rotated archive missing pre-rotate event")
	}
	if strings.Contains(string(rotData), "post-rotate") {
		t.Fatal("triggering write went to the rotated inode instead of the new canonical file")
	}
}

func TestEmitter_Redaction(t *testing.T) {
	_, festivalPath := setupCampaign(t, "test-campaign")
	e := NewFromFestival(context.Background(), festivalPath, func(err error) { t.Fatalf("unexpected warn: %v", err) })

	e.Emit(context.Background(), "festival.created", Scope{}, "fest create festival --name test --token supersecret123",
		WithData(map[string]any{}))

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	data, err := os.ReadFile(festFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "supersecret123") {
		t.Fatal("token value should be redacted from source_cmd")
	}
	if !strings.Contains(string(data), "<REDACTED>") {
		t.Fatal("expected <REDACTED> in source_cmd")
	}
}

func TestEmitter_RedactionInlineValue(t *testing.T) {
	_, festivalPath := setupCampaign(t, "test-campaign")
	e := NewFromFestival(context.Background(), festivalPath, func(err error) { t.Fatalf("unexpected warn: %v", err) })

	e.Emit(context.Background(), "festival.created", Scope{},
		"fest create festival --name test --password=hunter2", nil)

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	data, _ := os.ReadFile(festFile)
	if strings.Contains(string(data), "hunter2") {
		t.Fatal("password value should be redacted from source_cmd")
	}
	if !strings.Contains(string(data), "<REDACTED>") {
		t.Fatal("expected <REDACTED> in source_cmd")
	}
}

func TestEmitter_ReasonNotRedacted(t *testing.T) {
	_, festivalPath := setupCampaign(t, "test-campaign")
	e := NewFromFestival(context.Background(), festivalPath, func(err error) { t.Fatalf("unexpected warn: %v", err) })

	e.Emit(context.Background(), "workflow.skipped", Scope{},
		`fest workflow skip --reason "already done externally"`, nil)

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	data, _ := os.ReadFile(festFile)
	if !strings.Contains(string(data), "already done externally") {
		t.Fatal("arbitrary user messages (--reason) should be logged verbatim, not redacted")
	}
}

func TestEmitter_NoCampaignIsFestivalOnly(t *testing.T) {
	root := t.TempDir()
	festivalPath := filepath.Join(root, "standalone-fest")
	if err := os.MkdirAll(filepath.Join(festivalPath, ".fest"), 0o755); err != nil {
		t.Fatal(err)
	}

	e := NewFromFestival(context.Background(), festivalPath, func(error) {})
	e.Emit(context.Background(), "phase.created", Scope{}, "fest create phase", nil)

	// Even DestBoth events should write to festival level only when no campaign.
	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	events := readJSONL(t, festFile)
	if len(events) != 1 {
		t.Fatalf("expected 1 festival event, got %d", len(events))
	}
}

func TestEmitter_DisabledWhenFestivalPathEmpty(t *testing.T) {
	e := NewFromFestival(context.Background(), "", func(error) {})
	if !e.disabled {
		t.Fatal("expected disabled emitter when festivalPath is empty")
	}
	e.Emit(context.Background(), "task.created", Scope{}, "fest create task", nil)
	// No panic, no file written.
}

func TestEmitter_ReadOnlyCommandsEmitNothing(t *testing.T) {
	// This test documents the contract: the activity package does not auto-emit
	// for read-only commands. Commands that should not emit simply do not call
	// Emit(). This test verifies that calling Emit with a known read-only
	// concept produces no special behavior — the caller decides.
	_, festivalPath := setupCampaign(t, "test-campaign")
	e := NewFromFestival(context.Background(), festivalPath, func(error) {})

	// If someone mistakenly calls Emit for a read-only event, it still writes.
	// The contract is that callers don't call Emit for read-only commands.
	// This test verifies the Emitter itself works correctly.
	e.Emit(context.Background(), "next.resolved", Scope{}, "fest next", nil)

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	events := readJSONL(t, festFile)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestEmitter_ConcurrentWrites(t *testing.T) {
	campaignRoot, festivalPath := setupCampaign(t, "test-campaign")

	const numWriters = 10
	const eventsPerWriter = 20
	done := make(chan error, numWriters)

	for i := 0; i < numWriters; i++ {
		go func(writerID int) {
			e := NewFromFestival(context.Background(), festivalPath, func(error) {})
			for j := 0; j < eventsPerWriter; j++ {
				e.Emit(context.Background(), "task.completed", Scope{Task: "01_task.md"},
					"fest task completed", WithData(map[string]any{"writer": writerID}))
			}
			done <- nil
		}(i)
	}

	for i := 0; i < numWriters; i++ {
		if err := <-done; err != nil {
			t.Fatalf("writer %d failed: %v", i, err)
		}
	}

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	events := readJSONL(t, festFile)
	expected := numWriters * eventsPerWriter
	if len(events) != expected {
		t.Fatalf("expected %d events, got %d", expected, len(events))
	}

	// Each line should be valid JSON (already verified by readJSONL).
	_ = campaignRoot
}

func TestCatalog_DestinationDefaults(t *testing.T) {
	for _, ev := range []string{"festival.created", "festival.promoted", "phase.created", "sequence.created"} {
		if destination(ev) != DestBoth {
			t.Fatalf("expected %s to be DestBoth", ev)
		}
	}
	for _, ev := range []string{"task.created", "task.completed", "task.blocked", "task.reset", "validate.ran", "next.resolved", "go.navigated", "workflow.skipped"} {
		if destination(ev) != DestFestivalOnly {
			t.Fatalf("expected %s to be DestFestivalOnly", ev)
		}
	}
	if destination("unknown.event") != DestFestivalOnly {
		t.Fatal("unknown events should default to DestFestivalOnly")
	}
}

func TestCatalog_UnpublishedEventsAreNotLive(t *testing.T) {
	// Reserved names must not be in the live catalog until a caller Emit()s them.
	for _, ev := range []string{
		"festival.linked", "festival.unlinked", "festival.deleted", "festival.renamed",
		"gate.applied", "gate.skipped",
		"phase.started", "phase.completed", "phase.deleted",
		"sequence.started", "sequence.completed", "sequence.deleted",
		"task.started", "task.deleted", "task.renamed", "commit.made",
		"init.ran", "tui.action",
	} {
		if _, live := catalog[ev]; live {
			t.Fatalf("%s is catalogued as live but is not emitted in this PR", ev)
		}
	}
}

func TestRedact(t *testing.T) {
	cases := []struct {
		input  string
		expect string
	}{
		{"fest create --token secret", "fest create --token <REDACTED>"},
		{"fest --password=pw create", "fest --password=<REDACTED> create"},
		{"fest --signing-key key", "fest --signing-key <REDACTED>"},
		{"fest --secret s --reason ok", "fest --secret <REDACTED> --reason ok"},
		{"fest create --name test", "fest create --name test"},
		{"", ""},
	}
	for _, tc := range cases {
		got := redact(tc.input)
		if got != tc.expect {
			t.Fatalf("redact(%q) = %q, want %q", tc.input, got, tc.expect)
		}
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }
