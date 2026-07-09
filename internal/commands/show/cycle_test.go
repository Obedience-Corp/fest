package show

import (
	"context"
	"testing"

	"golang.org/x/term"
)

func noopExtraKey(context.Context, *FestivalInfo, *term.State) (string, bool) {
	return "", false
}

func TestClassifyCycleKey(t *testing.T) {
	extra := map[byte]ExtraKeyHandler{'p': noopExtraKey, 'q': noopExtraKey}
	cases := []struct {
		name      string
		in        []byte
		extraKeys map[byte]ExtraKeyHandler
		want      CycleAction
		wantKey   byte
		ok        bool
	}{
		{"ctrl_c", []byte{0x03}, extra, CycleQuit, 0, true},
		{"right_arrow", []byte{0x1b, '[', 'C'}, extra, CycleNext, 0, true},
		{"left_arrow", []byte{0x1b, '[', 'D'}, extra, CyclePrev, 0, true},
		{"registered_p_dispatches_extra", []byte{'p'}, extra, CycleExtra, 'p', true},
		{"registered_q_dispatches_extra", []byte{'q'}, extra, CycleExtra, 'q', true},
		{"unregistered_p_ignored", []byte{'p'}, nil, CycleNone, 0, false},
		{"up_arrow_ignored", []byte{0x1b, '[', 'A'}, extra, CycleNone, 0, false},
		{"plain_char", []byte{'x'}, extra, CycleNone, 0, false},
		{"short_escape", []byte{0x1b, '['}, extra, CycleNone, 0, false},
		{"empty", nil, extra, CycleNone, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, key, ok := classifyCycleKey(tc.in, tc.extraKeys)
			if got != tc.want || key != tc.wantKey || ok != tc.ok {
				t.Errorf("classifyCycleKey(%v) = (%v, %q, %v), want (%v, %q, %v)",
					tc.in, got, key, ok, tc.want, tc.wantKey, tc.ok)
			}
		})
	}
}

func TestRunCycle_EmptyPaths(t *testing.T) {
	err := RunCycle(t.Context(), nil, 0, CycleOptions{
		Detect: func(context.Context, string) (*FestivalInfo, error) { return &FestivalInfo{}, nil },
		Render: func(context.Context, *FestivalInfo, bool) error { return nil },
	})
	if err == nil {
		t.Fatal("expected error for empty path list")
	}
}

func TestRunCycle_RequiresDelegates(t *testing.T) {
	err := RunCycle(t.Context(), []string{"/festivals/active/x-FS0001"}, 0, CycleOptions{})
	if err == nil {
		t.Fatal("expected validation error when detect/render delegates are missing")
	}
}

func TestRunCycle_NonTerminalRendersStartIndexOnce(t *testing.T) {
	paths := []string{
		"/festivals/active/a-FS0001",
		"/festivals/ready/b-FS0002",
		"/festivals/planning/c-FS0003",
	}
	var rendered string
	renderCount := 0
	opts := CycleOptions{
		Detect: func(_ context.Context, path string) (*FestivalInfo, error) {
			return &FestivalInfo{Path: path}, nil
		},
		Render: func(_ context.Context, festival *FestivalInfo, cycling bool) error {
			rendered = festival.Path
			renderCount++
			if cycling {
				t.Error("non-terminal single-frame render should not report cycling")
			}
			return nil
		},
	}

	if err := RunCycle(t.Context(), paths, 2, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rendered != paths[2] {
		t.Fatalf("rendered %q, want start-index festival %q", rendered, paths[2])
	}
	if renderCount != 1 {
		t.Fatalf("render called %d times, want 1 for non-terminal frame", renderCount)
	}
}

func TestRunCycle_NonTerminalPrefersRenderFallback(t *testing.T) {
	paths := []string{"/festivals/active/a-FS0001"}
	fallbackCount := 0
	err := RunCycle(t.Context(), paths, 0, CycleOptions{
		Detect: func(_ context.Context, path string) (*FestivalInfo, error) {
			return &FestivalInfo{Path: path}, nil
		},
		Render: func(context.Context, *FestivalInfo, bool) error {
			t.Error("cycle renderer must not run outside raw mode when a fallback is set")
			return nil
		},
		RenderFallback: func(_ context.Context, festival *FestivalInfo) error {
			if festival.Path != paths[0] {
				t.Errorf("fallback rendered %q, want %q", festival.Path, paths[0])
			}
			fallbackCount++
			return nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fallbackCount != 1 {
		t.Fatalf("fallback called %d times, want 1", fallbackCount)
	}
}

func TestRunCycle_NonTerminalDetectNilReturnsNotFound(t *testing.T) {
	err := RunCycle(t.Context(), []string{"/festivals/active/gone-FS0001"}, 0, CycleOptions{
		Detect: func(context.Context, string) (*FestivalInfo, error) { return nil, nil },
		Render: func(context.Context, *FestivalInfo, bool) error {
			t.Fatal("render must not run when the start festival is missing")
			return nil
		},
	})
	if err == nil {
		t.Fatal("expected not-found error for missing start festival")
	}
}

func TestShowCycleExtraKeysQuit(t *testing.T) {
	keys := showCycleExtraKeys()
	for _, b := range []byte{'q', 'Q'} {
		handler, ok := keys[b]
		if !ok {
			t.Fatalf("show cycle should register %q as a quit key", b)
		}
		if newPath, cont := handler(t.Context(), &FestivalInfo{}, nil); cont || newPath != "" {
			t.Errorf("quit handler for %q = (%q, %v), want (\"\", false)", b, newPath, cont)
		}
	}
	if _, ok := keys['p']; ok {
		t.Error("show cycle must not register a promote key")
	}
}
