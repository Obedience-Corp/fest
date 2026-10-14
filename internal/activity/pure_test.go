package activity

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEmitter_DisabledWhenFestivalPathEmpty(t *testing.T) {
	e := NewFromFestival(context.Background(), "", func(error) {})
	if !e.disabled {
		t.Fatal("expected disabled emitter when festivalPath is empty")
	}
	e.Emit(context.Background(), "task.created", Scope{}, "fest create task", nil)
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

func TestActivityPathLockDeterministicForSameLog(t *testing.T) {
	t.Setenv(activityPathLockDirEnv, filepath.Join("test-cache", "activity-locks"))
	logPath := filepath.Join("test-camp", ".fest", FestivalFileName)
	a, err := activityLockPath(logPath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := activityLockPath(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("lock paths differ for same log: %q vs %q", a, b)
	}
}
