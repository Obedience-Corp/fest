package watch

import "testing"

func TestShowWatchOptionsAlwaysEnableInProgress(t *testing.T) {
	got := showWatchOptions(options{})
	if !got.InProgress {
		t.Fatal("fest watch must always enable in-progress expansion")
	}
}

func TestShowWatchOptionsMapCommandFlags(t *testing.T) {
	got := showWatchOptions(options{
		summary:   true,
		goals:     true,
		collapsed: true,
	})

	if !got.Summary {
		t.Fatal("summary flag was not mapped")
	}
	if !got.Goals {
		t.Fatal("goals flag was not mapped")
	}
	if !got.Collapsed {
		t.Fatal("collapsed flag was not mapped")
	}
}

func TestWatchCommandDoesNotExposeJSONFlag(t *testing.T) {
	cmd := NewWatchCommand()
	if flag := cmd.Flags().Lookup("json"); flag != nil {
		t.Fatal("fest watch should not expose --json in the first production PR")
	}
}
