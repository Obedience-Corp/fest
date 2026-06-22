package watch

import (
	"context"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/commands/show"
	"github.com/Obedience-Corp/fest/internal/errors"
)

func TestCycleWatchOptionsSetsCycleHint(t *testing.T) {
	opts := options{summary: true, goals: true, collapsed: true}
	got := cycleWatchOptions(opts, true)
	if !got.CycleHint {
		t.Fatal("cycleWatchOptions must set CycleHint = true")
	}
	if !got.Cycling {
		t.Fatal("cycleWatchOptions must propagate cycling = true")
	}
	if !got.Summary || !got.Goals || !got.Collapsed {
		t.Fatalf("cycleWatchOptions dropped flags: %#v", got)
	}
	if single := cycleWatchOptions(opts, false); single.Cycling {
		t.Fatal("cycleWatchOptions must leave Cycling = false for a single target")
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

func TestCurrentFestivalIndex(t *testing.T) {
	paths := []string{
		"/festivals/active/a-FS0001",
		"/festivals/ready/b-FS0002",
		"/festivals/planning/c-FS0003",
	}
	tests := []struct {
		name           string
		detectFestival func(context.Context, string) (*show.FestivalInfo, error)
		want           int
	}{
		{
			name: "detected festival selects its index",
			detectFestival: func(context.Context, string) (*show.FestivalInfo, error) {
				return &show.FestivalInfo{Path: paths[2]}, nil
			},
			want: 2,
		},
		{
			name: "not in a festival falls back to zero",
			detectFestival: func(context.Context, string) (*show.FestivalInfo, error) {
				return nil, errors.NotFound("festival")
			},
			want: 0,
		},
		{
			name: "detected festival outside the cycle falls back to zero",
			detectFestival: func(context.Context, string) (*show.FestivalInfo, error) {
				return &show.FestivalInfo{Path: "/festivals/active/unlisted-FS0009"}, nil
			},
			want: 0,
		},
		{
			name:           "missing detector falls back to zero",
			detectFestival: nil,
			want:           0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deps := commandDeps{detectFestival: tt.detectFestival}
			if got := currentFestivalIndex(t.Context(), "/some/cwd", paths, deps); got != tt.want {
				t.Fatalf("currentFestivalIndex = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestWatchCommandCycleStartsAtCurrentFestival(t *testing.T) {
	paths := []string{
		"/festivals/active/first-FS0001",
		"/festivals/ready/current-FS0002",
		"/festivals/planning/third-FS0003",
	}
	var watched string
	deps := commandDeps{
		resolveStandalone: func(context.Context) (*show.StandaloneWorkflowInfo, error) {
			return nil, nil
		},
		listCycleTargets: func(_ context.Context, _ string) ([]string, error) {
			return paths, nil
		},
		detectFestival: func(_ context.Context, p string) (*show.FestivalInfo, error) {
			if strings.HasPrefix(p, "/festivals/") {
				return &show.FestivalInfo{Path: p}, nil
			}
			return &show.FestivalInfo{Path: paths[1]}, nil
		},
		watch: func(_ context.Context, festival *show.FestivalInfo, _ show.WatchOptions) error {
			watched = festival.Path
			return nil
		},
	}

	if err := runWatch(t.Context(), nil, &options{}, deps); err != nil {
		t.Fatalf("runWatch: %v", err)
	}
	if watched != paths[1] {
		t.Fatalf("watch started at %q, want current festival %q", watched, paths[1])
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
		{"lower_p_promotes", []byte{'p'}, cyclePromote, true},
		{"upper_p_promotes", []byte{'P'}, cyclePromote, true},
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
