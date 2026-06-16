package watch

import (
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

func TestClassifyCycleKey(t *testing.T) {
	cases := []struct {
		name string
		in   []byte
		want cycleDirection
		ok   bool
	}{
		{"ctrl_c", []byte{0x03}, cycleQuit, true},
		{"right_arrow", []byte{0x1b, '[', 'C'}, cycleNext, true},
		{"left_arrow", []byte{0x1b, '[', 'D'}, cyclePrev, true},
		{"up_arrow_ignored", []byte{0x1b, '[', 'A'}, cycleNone, false},
		{"plain_char", []byte{'x'}, cycleNone, false},
		{"short_escape", []byte{0x1b, '['}, cycleNone, false},
		{"empty", nil, cycleNone, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := classifyCycleKey(tc.in)
			if got != tc.want || ok != tc.ok {
				t.Errorf("classifyCycleKey(%v) = (%v, %v), want (%v, %v)", tc.in, got, ok, tc.want, tc.ok)
			}
		})
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
