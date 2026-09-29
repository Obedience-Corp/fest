package progress

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"
	"time"
)

func migrationBase() time.Time {
	return time.Date(2026, 9, 20, 10, 0, 0, 0, time.UTC)
}

func eventNames(events []ProgressEvent) []string {
	names := make([]string, 0, len(events))
	for _, e := range events {
		names = append(names, string(e.Event))
	}
	return names
}

func TestMigrateDeferredLegacyTask(t *testing.T) {
	base := migrationBase()
	started := base
	blocked := base.Add(30 * time.Minute)
	deferred := base.Add(45 * time.Minute)

	events := generateEventsFromState(map[string]*TaskProgress{
		"01_task.md": {
			TaskID:            "01_task.md",
			Status:            StatusBlocked,
			StartedAt:         &started,
			BlockedAt:         &blocked,
			BlockerMessage:    "provider API removed",
			BlockerAttempts:   []string{"checked the changelog"},
			BlockerDeferred:   true,
			BlockerDeferredAt: &deferred,
			BlockerDeferredBy: "Ada Lovelace",
			DeferralReason:    "the vendor replies next week",
		},
	})

	want := []string{"started", "blocked", "blocker_deferred"}
	if got := eventNames(events); !slices.Equal(got, want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	if got := events[1].Attempts; !slices.Equal(got, []string{"checked the changelog"}) {
		t.Errorf("blocked event Attempts = %q", got)
	}

	deferral := events[2]
	if !deferral.Timestamp.Equal(deferred) {
		t.Errorf("deferral Timestamp = %v, want %v", deferral.Timestamp, deferred)
	}
	if deferral.DeferralReason != "the vendor replies next week" {
		t.Errorf("deferral DeferralReason = %q", deferral.DeferralReason)
	}
	if deferral.DeferredBy != "Ada Lovelace" {
		t.Errorf("deferral DeferredBy = %q", deferral.DeferredBy)
	}
}

func TestMigrateDeferralTimestampEdges(t *testing.T) {
	base := migrationBase()
	started := base
	blocked := base.Add(30 * time.Minute)

	cases := []struct {
		name       string
		deferredAt *time.Time
		wantStamp  time.Time
	}{
		{"deferral stamped at the same instant as the block", &blocked, blocked},
		{"deferral with no timestamp falls back to the block", nil, blocked},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events := generateEventsFromState(map[string]*TaskProgress{
				"01_task.md": {
					TaskID:            "01_task.md",
					Status:            StatusBlocked,
					StartedAt:         &started,
					BlockedAt:         &blocked,
					BlockerMessage:    "provider API removed",
					BlockerDeferred:   true,
					BlockerDeferredAt: tc.deferredAt,
					DeferralReason:    "the vendor replies next week",
				},
			})

			want := []string{"started", "blocked", "blocker_deferred"}
			if got := eventNames(events); !slices.Equal(got, want) {
				t.Fatalf("events = %v, want %v", got, want)
			}
			if !events[2].Timestamp.Equal(tc.wantStamp) {
				t.Errorf("deferral Timestamp = %v, want %v", events[2].Timestamp, tc.wantStamp)
			}

			tasks, _ := materializeState(events)
			if !tasks["01_task.md"].BlockerDeferred {
				t.Error("replaying the migrated events must leave the task deferred")
			}
		})
	}
}

func TestMigrateWithNothingDeferredMatchesGolden(t *testing.T) {
	base := migrationBase()
	started := base
	blockStart := base.Add(20 * time.Minute)
	blocked := base.Add(30 * time.Minute)
	completed := base.Add(2 * time.Hour)
	inprog := base.Add(3 * time.Hour)
	pending := base.Add(4 * time.Hour)

	events := generateEventsFromState(map[string]*TaskProgress{
		"01_done.md": {TaskID: "01_done.md", Status: StatusCompleted, Progress: 100,
			StartedAt: &started, CompletedAt: &completed, TimeSpentMinutes: 120},
		"02_blocked.md": {TaskID: "02_blocked.md", Status: StatusBlocked, Progress: 40,
			StartedAt: &blockStart, BlockedAt: &blocked, BlockerMessage: "waiting on upstream"},
		"03_wip.md": {TaskID: "03_wip.md", Status: StatusInProgress, Progress: 55,
			StartedAt: &inprog},
		"04_pending.md": {TaskID: "04_pending.md", Status: StatusPending, StartedAt: &pending},
	})

	raw, err := json.Marshal(events)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Captured from the unmodified generateEventsFromState before this festival
	// touched it. A legacy store with nothing deferred must migrate to exactly
	// these events.
	const golden = `[{"ts":"2026-09-20T10:00:00Z","event":"started","task":"01_done.md"},` +
		`{"ts":"2026-09-20T10:20:00Z","event":"started","task":"02_blocked.md"},` +
		`{"ts":"2026-09-20T10:30:00Z","event":"blocked","task":"02_blocked.md","reason":"waiting on upstream"},` +
		`{"ts":"2026-09-20T12:00:00Z","event":"completed","task":"01_done.md","minutes":120},` +
		`{"ts":"2026-09-20T13:00:00Z","event":"started","task":"03_wip.md"},` +
		`{"ts":"2026-09-20T13:00:01Z","event":"progress","task":"03_wip.md","percent":55},` +
		`{"ts":"2026-09-20T14:00:00Z","event":"started","task":"04_pending.md"}]`

	if string(raw) != golden {
		t.Errorf("migrated events =\n%s\nwant\n%s", raw, golden)
	}
}

func TestMigrateThenReplayReproducesTheRecord(t *testing.T) {
	base := migrationBase()
	started := base
	blocked := base.Add(30 * time.Minute)
	deferred := base.Add(45 * time.Minute)

	original := &TaskProgress{
		TaskID:            "01_task.md",
		Status:            StatusBlocked,
		StartedAt:         &started,
		BlockedAt:         &blocked,
		BlockerMessage:    "provider API removed",
		BlockerAttempts:   []string{"checked the changelog", "asked the vendor"},
		BlockerDeferred:   true,
		BlockerDeferredAt: &deferred,
		BlockerDeferredBy: "Ada Lovelace",
		DeferralReason:    "the vendor replies next week",
	}

	events := generateEventsFromState(map[string]*TaskProgress{original.TaskID: original})
	tasks, _ := materializeState(events)

	got, ok := tasks[original.TaskID]
	if !ok {
		t.Fatal("task missing after migrate and replay")
	}
	if !reflect.DeepEqual(got, original) {
		t.Errorf("migrate then replay produced\n%+v\nwant\n%+v", got, original)
	}
}
