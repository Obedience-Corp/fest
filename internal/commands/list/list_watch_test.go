package list

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/commands/show"
)

func TestFormatAllHumanEmpty(t *testing.T) {
	out := formatAllHuman(nil, nil, nil, nil, 0, false, nil)
	if !strings.Contains(out, "No festivals found") {
		t.Fatalf("expected empty-board message, got %q", out)
	}
	if !strings.Contains(out, "fest create festival") {
		t.Fatalf("expected create hint, got %q", out)
	}
}

func TestFormatDungeonHumanEmpty(t *testing.T) {
	out := formatDungeonHuman(nil, nil, nil, 0, false, nil)
	if !strings.Contains(out, "No festivals in dungeon") {
		t.Fatalf("expected empty dungeon message, got %q", out)
	}
}

// A campaign with no residents must render exactly what the pre-rail binary
// rendered: no residents header, no separator, no count.
func TestFormatStatusHuman_NoResidentsIsUnchanged(t *testing.T) {
	withResidents := formatStatusHuman("active", nil, nil, nil, false, nil)
	if strings.Contains(strings.ToLower(withResidents), "resident") {
		t.Errorf("resident-free output mentions residents: %q", withResidents)
	}
}

func TestFormatStatusHuman_ResidentsAppendAfterFestivals(t *testing.T) {
	residents := []*show.ResidentCard{
		{Name: "alpha", Title: "Alpha", Type: "design"},
		{Name: "beta", Title: "Beta", Type: "explore"},
	}
	out := formatStatusHuman("active", nil, residents, nil, false, nil)

	if !strings.Contains(out, "ACTIVE Residents (2)") {
		t.Errorf("missing residents header: %q", out)
	}
	for _, want := range []string{"alpha", "beta", "[design]", "[explore]"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %q", want, out)
		}
	}
	// Festivals first, residents after.
	if fi, ri := strings.Index(out, "ACTIVE Festivals"), strings.Index(out, "ACTIVE Residents"); fi < 0 || ri < fi {
		t.Errorf("residents block should follow the festival block: %q", out)
	}
}

func TestFormatAllHuman_NoResidentsKeepsLegacyPath(t *testing.T) {
	// With no residents the empty-board message must still appear, unchanged.
	out := formatAllHuman(nil, nil, nil, nil, 0, false, nil)
	if !strings.Contains(out, "No festivals found") {
		t.Errorf("expected the legacy empty message, got %q", out)
	}
	if strings.Contains(strings.ToLower(out), "resident") {
		t.Errorf("resident-free output mentions residents: %q", out)
	}
}

func TestWaitListWatchEvents_CanceledContextReturnsNil(t *testing.T) {
	restore := silenceStdout(t)
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitListWatchEvents(ctx, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("waitListWatchEvents(canceled) = %v, want nil (Ctrl+C is a clean user stop)", err)
	}
}

func TestWaitListWatchEvents_CancelDuringWatchReturnsNil(t *testing.T) {
	restore := silenceStdout(t)
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchDone := make(chan error)
	errCh := make(chan error, 1)
	go func() {
		errCh <- waitListWatchEvents(ctx, watchDone, nil, nil, nil, nil)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("waitListWatchEvents() after cancel = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waitListWatchEvents() did not return after context cancel")
	}
}

func TestWaitListWatchEvents_WatcherErrorReturnedWhenLive(t *testing.T) {
	restore := silenceStdout(t)
	defer restore()

	watchDone := make(chan error, 1)
	want := errors.New("watch failed")
	watchDone <- want
	err := waitListWatchEvents(context.Background(), watchDone, nil, nil, nil, nil)
	if !errors.Is(err, want) {
		t.Fatalf("waitListWatchEvents() = %v, want %v", err, want)
	}
}

func TestWaitListWatchEvents_WatcherExitOnCancelIsNil(t *testing.T) {
	restore := silenceStdout(t)
	defer restore()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	watchDone := make(chan error, 1)
	watchDone <- errors.New("watcher closed after cancel")
	if err := waitListWatchEvents(ctx, watchDone, nil, nil, nil, nil); err != nil {
		t.Fatalf("canceled watch with watcher error = %v, want nil", err)
	}
}

func TestWaitListWatchEvents_OnChangeError(t *testing.T) {
	restore := silenceStdout(t)
	defer restore()

	changes := make(chan struct{}, 1)
	changes <- struct{}{}
	want := errors.New("paint failed")
	err := waitListWatchEvents(context.Background(), nil, changes, nil, func() error {
		return want
	}, nil)
	if !errors.Is(err, want) {
		t.Fatalf("waitListWatchEvents() = %v, want paint error", err)
	}
}

func TestWaitListWatchEvents_OnTickError(t *testing.T) {
	restore := silenceStdout(t)
	defer restore()

	tick := make(chan time.Time, 1)
	tick <- time.Now()
	want := errors.New("reconcile failed")
	err := waitListWatchEvents(context.Background(), nil, nil, tick, nil, func() error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("waitListWatchEvents() = %v, want tick error", err)
	}
}

func silenceStdout(t *testing.T) func() {
	t.Helper()
	orig := os.Stdout
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open os.DevNull: %v", err)
	}
	os.Stdout = devNull
	return func() {
		os.Stdout = orig
		_ = devNull.Close()
	}
}

func TestFormatAllHuman_ResidentOnlyStageStillRenders(t *testing.T) {
	residents := map[string][]*show.ResidentCard{
		"active": {{Name: "solo", Title: "Solo", Type: "design"}},
	}
	// totalCount 0: no festivals at all, only a resident. It must not fall into
	// the "No festivals found" branch, or the resident would be invisible.
	out := formatAllHuman(nil, residents, []string{"active"}, nil, 0, false, nil)
	if strings.Contains(out, "No festivals found") {
		t.Fatalf("a resident-only campaign must not report an empty board: %q", out)
	}
	if !strings.Contains(out, "solo") {
		t.Errorf("resident missing from output: %q", out)
	}
}
