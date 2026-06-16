package watch

import (
	"bytes"
	"context"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
)

func TestCycleWatchOptionsSetsCycleHint(t *testing.T) {
	opts := options{summary: true, goals: true, collapsed: true}
	got := cycleWatchOptions(opts)
	if !got.CycleHint {
		t.Fatal("cycleWatchOptions must set CycleHint = true")
	}
	if !got.Summary || !got.Goals || !got.Collapsed {
		t.Fatalf("cycleWatchOptions dropped flags: %#v", got)
	}
}

func TestRunWatchCycle_EmptyPaths(t *testing.T) {
	err := runWatchCycle(t.Context(), nil, 0, options{}, commandDeps{
		detectFestival: func(context.Context, string) (*show.FestivalInfo, error) {
			return nil, nil
		},
	})
	if err == nil {
		t.Fatal("expected error for empty path list")
	}
}

func TestRunWatchCycle_SinglePath_CallsWatch(t *testing.T) {
	watchCalled := false
	deps := commandDeps{
		detectFestival: func(_ context.Context, path string) (*show.FestivalInfo, error) {
			return &show.FestivalInfo{Path: path}, nil
		},
		watch: func(_ context.Context, _ *show.FestivalInfo, _ show.WatchOptions) error {
			watchCalled = true
			return nil
		},
	}
	paths := []string{"/festivals/active/fest-FS0001"}
	if err := runWatchCycle(t.Context(), paths, 0, options{}, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !watchCalled {
		t.Fatal("watch delegate was not called for single-path cycle")
	}
}

func TestReadCycleKey_RightArrow(t *testing.T) {
	rightArrow := []byte{0x1b, '[', 'C'}
	r := bytes.NewReader(rightArrow)
	cancelCalled := false
	cancel := func() { cancelCalled = true }
	dir := readCycleKey(t.Context(), r, cancel)
	if dir != cycleNext {
		t.Errorf("readCycleKey = %v, want cycleNext", dir)
	}
	if cancelCalled {
		t.Error("cancel should not be called for right arrow")
	}
}

func TestReadCycleKey_LeftArrow(t *testing.T) {
	leftArrow := []byte{0x1b, '[', 'D'}
	r := bytes.NewReader(leftArrow)
	dir := readCycleKey(t.Context(), r, func() {})
	if dir != cyclePrev {
		t.Errorf("readCycleKey = %v, want cyclePrev", dir)
	}
}

func TestReadCycleKey_CtrlC(t *testing.T) {
	ctrlC := []byte{0x03}
	r := bytes.NewReader(ctrlC)
	cancelCalled := false
	dir := readCycleKey(t.Context(), r, func() { cancelCalled = true })
	if dir != cycleQuit {
		t.Errorf("readCycleKey = %v, want cycleQuit", dir)
	}
	if !cancelCalled {
		t.Error("cancel should be called for ctrl+c")
	}
}

func TestReadCycleKey_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := bytes.NewReader(nil)
	dir := readCycleKey(ctx, r, func() {})
	if dir != cycleQuit {
		t.Errorf("readCycleKey on cancelled context = %v, want cycleQuit", dir)
	}
}

func TestDefaultListCycleTargets_NoCampaign(t *testing.T) {
	dir := t.TempDir()
	paths, err := defaultListCycleTargets(t.Context(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected empty paths for no campaign, got %v", paths)
	}
}

func TestWatchCommandUsesListCycleTargets(t *testing.T) {
	listCalled := false
	watchCalled := false

	deps := commandDeps{
		resolveStandalone: func(context.Context) (*show.StandaloneWorkflowInfo, error) {
			return nil, nil
		},
		resolve: func(_ context.Context, _ string) (*show.FestivalInfo, error) {
			return &show.FestivalInfo{Path: "/festivals/active/single-FS0001"}, nil
		},
		watch: func(_ context.Context, _ *show.FestivalInfo, _ show.WatchOptions) error {
			watchCalled = true
			return nil
		},
		detectFestival: func(_ context.Context, path string) (*show.FestivalInfo, error) {
			return &show.FestivalInfo{Path: path}, nil
		},
		listCycleTargets: func(_ context.Context, _ string) ([]string, error) {
			listCalled = true
			return []string{"/festivals/active/one-FS0001"}, nil
		},
	}

	if err := runWatch(t.Context(), []string{}, &options{}, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !listCalled {
		t.Fatal("listCycleTargets was not called")
	}
	if !watchCalled {
		t.Fatal("watch delegate was not called (single-path should delegate)")
	}
}

func TestWatchCommandSkipsCyclingWhenSelectorProvided(t *testing.T) {
	listCalled := false
	deps := commandDeps{
		resolveStandalone: func(context.Context) (*show.StandaloneWorkflowInfo, error) {
			return nil, nil
		},
		resolve: func(_ context.Context, _ string) (*show.FestivalInfo, error) {
			return &show.FestivalInfo{Path: "/festivals/active/explicit-FE0001"}, nil
		},
		watch: func(_ context.Context, _ *show.FestivalInfo, _ show.WatchOptions) error {
			return nil
		},
		listCycleTargets: func(_ context.Context, _ string) ([]string, error) {
			listCalled = true
			return nil, nil
		},
	}

	if err := runWatch(t.Context(), []string{"explicit-FE0001"}, &options{}, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listCalled {
		t.Fatal("listCycleTargets must not be called when an explicit selector is given")
	}
}
