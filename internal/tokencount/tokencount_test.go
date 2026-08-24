package tokencount

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatCompact(t *testing.T) {
	tests := []struct {
		name string
		n    int
		want string
	}{
		{"zero", 0, "0"},
		{"small", 42, "42"},
		{"hundreds", 999, "999"},
		{"just over 1k", 1000, "1.0k"},
		{"1.2k", 1234, "1.2k"},
		{"9.9k", 9876, "9.9k"},
		{"ten k rounds", 10000, "10k"},
		{"12345 rounds to 12k", 12345, "12k"},
		{"hundred k", 130000, "130k"},
		{"million", 1500000, "1500k"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatCompact(tt.n)
			if got != tt.want {
				t.Errorf("FormatCompact(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

func TestNewCounter_EmptyCampaignRootDisabled(t *testing.T) {
	tc, err := NewCounter(t.Context(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tc.CountFestival(t.Context(), "/nonexistent") != 0 {
		t.Error("disabled counter should return 0")
	}
}

func TestCountFestival_RealDirectory(t *testing.T) {
	tmp := t.TempDir()
	// Create a festival-like directory with some markdown files.
	festDir := filepath.Join(tmp, "test-fest")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "README.md"), []byte("# Test Festival\n\nSome content here.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "plan.md"), []byte("# Plan\n\nA plan document.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use tmp as the "campaign root" so the cache lives under tmp/.campaign/cache/tokens/
	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tokens := tc.CountFestival(t.Context(), festDir)
	if tokens <= 0 {
		t.Errorf("expected positive token count, got %d", tokens)
	}
}

func TestCountFestival_CacheHit(t *testing.T) {
	tmp := t.TempDir()
	festDir := filepath.Join(tmp, "test-fest")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "doc.md"), []byte("# Document\n\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	first := tc.CountFestival(t.Context(), festDir)
	if first <= 0 {
		t.Fatalf("expected positive token count on first call, got %d", first)
	}

	// Second call should return the same value from cache without re-counting.
	second := tc.CountFestival(t.Context(), festDir)
	if second != first {
		t.Errorf("cache miss: first=%d, second=%d", first, second)
	}

	// Verify cache file exists on disk.
	cacheDir := filepath.Join(tmp, ".campaign", "cache", "tokens")
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		t.Fatalf("cache dir not created: %v", err)
	}
	if len(entries) == 0 {
		t.Error("no cache files written")
	}
}

func TestCountFestival_CacheInvalidatedOnChange(t *testing.T) {
	tmp := t.TempDir()
	festDir := filepath.Join(tmp, "test-fest")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "doc.md"), []byte("# Short\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	first := tc.CountFestival(t.Context(), festDir)

	// Modify the file — the fingerprint changes so the cache should be bypassed.
	if err := os.WriteFile(filepath.Join(festDir, "doc.md"), []byte("# Much longer content that adds more tokens to the document.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	second := tc.CountFestival(t.Context(), festDir)
	if second <= first {
		t.Errorf("expected token count to increase after adding content: first=%d, second=%d", first, second)
	}
}

func TestCountFestival_NonexistentDirectory(t *testing.T) {
	tmp := t.TempDir()
	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Nonexistent path: returns 0, no error.
	got := tc.CountFestival(t.Context(), filepath.Join(tmp, "no-such-fest"))
	if got != 0 {
		t.Errorf("expected 0 for nonexistent directory, got %d", got)
	}
}

func TestCountFestivals_MultiplePaths(t *testing.T) {
	tmp := t.TempDir()
	fest1 := filepath.Join(tmp, "fest-a")
	fest2 := filepath.Join(tmp, "fest-b")
	for _, d := range []string{fest1, fest2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "doc.md"), []byte("# Content\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := tc.CountFestivals(t.Context(), []string{fest1, fest2, "/nonexistent"})
	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	if result[fest1] <= 0 {
		t.Errorf("fest-a: expected positive count, got %d", result[fest1])
	}
	if result[fest2] <= 0 {
		t.Errorf("fest-b: expected positive count, got %d", result[fest2])
	}
	if result["/nonexistent"] != 0 {
		t.Errorf("nonexistent: expected 0, got %d", result["/nonexistent"])
	}
}
