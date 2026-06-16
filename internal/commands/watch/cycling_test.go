package watch

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

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
	r := bytes.NewReader([]byte{0x1b, '[', 'C'})
	dir := readCycleKey(t.Context(), r)
	if dir != cycleNext {
		t.Errorf("readCycleKey = %v, want cycleNext", dir)
	}
}

func TestReadCycleKey_LeftArrow(t *testing.T) {
	r := bytes.NewReader([]byte{0x1b, '[', 'D'})
	dir := readCycleKey(t.Context(), r)
	if dir != cyclePrev {
		t.Errorf("readCycleKey = %v, want cyclePrev", dir)
	}
}

func TestReadCycleKey_CtrlC(t *testing.T) {
	r := bytes.NewReader([]byte{0x03})
	dir := readCycleKey(t.Context(), r)
	if dir != cycleQuit {
		t.Errorf("readCycleKey = %v, want cycleQuit", dir)
	}
}

func TestReadCycleKey_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := bytes.NewReader(nil)
	dir := readCycleKey(ctx, r)
	if dir != cycleQuit {
		t.Errorf("readCycleKey on cancelled context = %v, want cycleQuit", dir)
	}
}

type fakeDeadlineReader struct {
	timeouts int
	data     []byte
	consumed bool
}

func (f *fakeDeadlineReader) SetReadDeadline(time.Time) error { return nil }

func (f *fakeDeadlineReader) Read(p []byte) (int, error) {
	if f.timeouts > 0 {
		f.timeouts--
		return 0, os.ErrDeadlineExceeded
	}
	if f.consumed {
		return 0, io.EOF
	}
	n := copy(p, f.data)
	f.consumed = true
	return n, nil
}

type errDeadlineReader struct {
	*bytes.Reader
}

func (errDeadlineReader) SetReadDeadline(time.Time) error { return os.ErrInvalid }

func TestReadCycleKey_DeadlineReader_TimesOutThenReads(t *testing.T) {
	r := &fakeDeadlineReader{timeouts: 3, data: []byte{0x1b, '[', 'C'}}
	dir := readCycleKey(t.Context(), r)
	if dir != cycleNext {
		t.Errorf("readCycleKey = %v, want cycleNext", dir)
	}
}

func TestReadCycleKey_DeadlineReader_ContextCancelledBeforeRead(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	r := &fakeDeadlineReader{timeouts: 100}
	dir := readCycleKey(ctx, r)
	if dir != cycleQuit {
		t.Errorf("readCycleKey on cancelled ctx = %v, want cycleQuit", dir)
	}
}

func TestReadCycleKey_DeadlineReader_FallsBackWhenUnsupported(t *testing.T) {
	r := errDeadlineReader{bytes.NewReader([]byte{0x1b, '[', 'D'})}
	dir := readCycleKey(t.Context(), r)
	if dir != cyclePrev {
		t.Errorf("readCycleKey = %v, want cyclePrev (blocking fallback)", dir)
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
		{"up_arrow_ignored", []byte{0x1b, '[', 'A'}, 0, false},
		{"plain_char", []byte{'x'}, 0, false},
		{"short_escape", []byte{0x1b, '['}, 0, false},
		{"empty", nil, 0, false},
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
