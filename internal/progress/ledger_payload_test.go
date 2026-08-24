package progress

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obedience-Corp/camp/pkg/ledgerkit"

	"github.com/Obedience-Corp/fest/internal/campledger"
	"github.com/Obedience-Corp/fest/internal/workspace"
)

// TestManager_CampaignLedgerPayloads is the fest#276 regression net.
// It drives production Manager call sites (MarkComplete / ReportBlocker /
// ResetTask) against a temp campaign and asserts the JSONL payloads they
// write. Hand-building maps and passing them to Emit would still pass if
// manager.go reverted to target=blocked|reset or status=completed.
func TestManager_CampaignLedgerPayloads(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		taskID        string
		mutate        func(*Manager, string) error
		wantKind      ledgerkit.Kind
		want          map[string]string
		forbiddenKeys []string
	}{
		{
			name:     "MarkComplete",
			taskID:   "001_PHASE/01_seq/01_complete.md",
			mutate:   func(m *Manager, id string) error { return m.MarkComplete(ctx, id) },
			wantKind: ledgerkit.KindCompleted,
			want: map[string]string{
				"target": "task",
				"to":     "completed",
			},
			forbiddenKeys: []string{"status"},
		},
		{
			name:     "ReportBlocker",
			taskID:   "001_PHASE/01_seq/02_block.md",
			mutate:   func(m *Manager, id string) error { return m.ReportBlocker(ctx, id, "waiting on spec") },
			wantKind: ledgerkit.KindTransitioned,
			want: map[string]string{
				"target": "task",
				"from":   StatusPending,
				"to":     "blocked",
				"action": "block",
			},
		},
		{
			name:     "ResetTask",
			taskID:   "001_PHASE/01_seq/03_reset.md",
			mutate:   func(m *Manager, id string) error { return m.ResetTask(ctx, id) },
			wantKind: ledgerkit.KindTransitioned,
			want: map[string]string{
				"target": "task",
				"to":     "pending",
				"action": "reset",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			camp, fest := setupCampaignFestival(t)
			if !campledger.NewFromFestival(ctx, fest, func(error) {}).Enabled() {
				t.Skip("writer id resolution disabled emitter in this environment")
			}

			mgr, err := NewManager(ctx, fest)
			if err != nil {
				t.Fatalf("NewManager: %v", err)
			}
			if err := tt.mutate(mgr, tt.taskID); err != nil {
				t.Fatalf("%s: %v", tt.name, err)
			}

			events := readCampaignLedgerEvents(t, camp)
			if len(events) == 0 {
				t.Fatal("expected campaign ledger events from Manager mutation")
			}

			var match *ledgerkit.Event
			for i := range events {
				ev := &events[i]
				if ev.Kind != tt.wantKind {
					continue
				}
				if match != nil {
					t.Fatalf("expected one %s event, got multiple", tt.wantKind)
				}
				match = ev
			}
			if match == nil {
				t.Fatalf("no %s event in ledger", tt.wantKind)
			}
			assertLedgerPayload(t, match, tt.want, tt.forbiddenKeys)
		})
	}
}

// TestManager_PayloadToCarriesDestinationStatus drives ReportBlocker and
// asserts the written JSONL uses to=blocked (not target as the status).
func TestManager_PayloadToCarriesDestinationStatus(t *testing.T) {
	ctx := context.Background()
	camp, fest := setupCampaignFestival(t)
	if !campledger.NewFromFestival(ctx, fest, func(error) {}).Enabled() {
		t.Skip("writer id resolution disabled emitter in this environment")
	}

	mgr, err := NewManager(ctx, fest)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := mgr.ReportBlocker(ctx, "001_PHASE/01_seq/01_task.md", "blocked"); err != nil {
		t.Fatalf("ReportBlocker: %v", err)
	}

	events := readCampaignLedgerEvents(t, camp)
	if len(events) == 0 {
		t.Fatal("expected campaign ledger events from ReportBlocker")
	}
	found := false
	for _, ev := range events {
		if ev.Kind != ledgerkit.KindTransitioned {
			continue
		}
		found = true
		if got := payloadString(ev.Payload, "to"); got != "blocked" {
			t.Errorf(`payload "to" = %q, want "blocked"`, got)
		}
		if got := payloadString(ev.Payload, "target"); got != "task" {
			t.Errorf(`payload "target" = %q, want "task" (artifact kind, not status)`, got)
		}
	}
	if !found {
		t.Fatal(`expected transitioned event with "to":"blocked"`)
	}
}

func setupCampaignFestival(t *testing.T) (campRoot, festPath string) {
	t.Helper()
	campRoot = t.TempDir()
	t.Setenv(workspace.EnvCampaignRoot, campRoot)
	if err := os.MkdirAll(filepath.Join(campRoot, ".campaign"), 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "id: test-campaign-id\nname: test\n"
	if err := os.WriteFile(filepath.Join(campRoot, ".campaign", "campaign.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	festPath = filepath.Join(campRoot, "festivals", "active", "demo-fest-DF0001")
	if err := os.MkdirAll(festPath, 0o755); err != nil {
		t.Fatal(err)
	}
	return campRoot, festPath
}

func readCampaignLedgerEvents(t *testing.T, campRoot string) []ledgerkit.Event {
	t.Helper()
	eventsRoot := filepath.Join(campRoot, ".campaign", "events")
	var events []ledgerkit.Event
	err := filepath.Walk(eventsRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if line == "" {
				continue
			}
			var ev ledgerkit.Event
			if json.Unmarshal([]byte(line), &ev) == nil {
				events = append(events, ev)
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return events
}

func assertLedgerPayload(t *testing.T, ev *ledgerkit.Event, want map[string]string, forbiddenKeys []string) {
	t.Helper()
	if ev.Payload == nil {
		t.Fatal("event missing payload")
	}
	target := payloadString(ev.Payload, "target")
	if target != "festival" && target != "task" {
		t.Errorf("target=%q must be artifact kind festival|task, not action or status", target)
	}
	for key, wantVal := range want {
		if got := payloadString(ev.Payload, key); got != wantVal {
			t.Errorf("payload %q = %q, want %q", key, got, wantVal)
		}
	}
	for _, key := range forbiddenKeys {
		if _, ok := ev.Payload[key]; ok {
			t.Errorf("payload must not contain %q: %v", key, ev.Payload)
		}
	}
}

func payloadString(p map[string]any, key string) string {
	if p == nil {
		return ""
	}
	s, _ := p[key].(string)
	return s
}
