package tokencount

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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

// onnxHeader is the leading bytes of a real ONNX model file. It contains no
// null bytes, so tcount's binary detection classifies it as text and tokenizes
// it — which is exactly how a festival holding model weights hung `fest list`.
var onnxHeader = []byte{
	0x08, 0x07, 0x12, 0x07, 0x70, 0x79, 0x74, 0x6f, 0x72, 0x63, 0x68, 0x1a,
	0x05, 0x32, 0x2e, 0x36, 0x2e, 0x30, 0x3a, 0xc9, 0xe3, 0xa2, 0x9b, 0x01,
	0x0a, 0x8b, 0x01, 0x0a, 0x37, 0x6b, 0x6d, 0x6f, 0x64, 0x65, 0x6c, 0x2e,
	0x64, 0x65, 0x63, 0x6f, 0x64, 0x65, 0x72, 0x2e,
}

// writeModelLikeFile writes size bytes of ONNX-like content at path.
func writeModelLikeFile(t *testing.T, path string, size int) {
	t.Helper()
	buf := make([]byte, 0, size+len(onnxHeader))
	for len(buf) < size {
		buf = append(buf, onnxHeader...)
	}
	if err := os.WriteFile(path, buf[:size], 0o644); err != nil {
		t.Fatal(err)
	}
}

// cacheEntryCount reports how many cache files the counter has written. The
// tokenizing path always writes one, so an empty cache dir proves the guard
// returned before tokenizing.
func cacheEntryCount(t *testing.T, campaignRoot string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(campaignRoot, ".campaign", "cache", "tokens"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return len(entries)
}

func TestCountFestival_OversizedFileSkipsTokenizing(t *testing.T) {
	tmp := t.TempDir()
	festDir := filepath.Join(tmp, "heavy-HH0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "GOAL.md"), []byte("# Goal\n\nShip the model.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeModelLikeFile(t, filepath.Join(festDir, "model.onnx"), 5<<20)

	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	start := time.Now()
	got := tc.CountFestival(t.Context(), festDir)
	elapsed := time.Since(start)

	if got != 0 {
		t.Errorf("expected 0 tokens for a festival holding an oversized file, got %d", got)
	}
	if n := cacheEntryCount(t, tmp); n != 0 {
		t.Errorf("guard should return before tokenizing, but %d cache entries were written", n)
	}
	// The stat-only scan is orders of magnitude faster than tokenizing 5 MB;
	// the bound is loose so a busy machine cannot make this flaky.
	if elapsed > perFestivalBudget {
		t.Errorf("guarded festival took %s, expected a stat-only scan", elapsed)
	}
}

func TestCountFestival_ManyFilesOverAggregateCapSkipsTokenizing(t *testing.T) {
	tmp := t.TempDir()
	festDir := filepath.Join(tmp, "bulky-BB0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Each file is under maxFileBytes; together they exceed maxFestivalBytes.
	for i := 0; i < 10; i++ {
		writeModelLikeFile(t, filepath.Join(festDir, fmt.Sprintf("shard-%02d.bin", i)), 2<<20)
	}

	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := tc.CountFestival(t.Context(), festDir); got != 0 {
		t.Errorf("expected 0 tokens for a festival over the aggregate cap, got %d", got)
	}
	if n := cacheEntryCount(t, tmp); n != 0 {
		t.Errorf("guard should return before tokenizing, but %d cache entries were written", n)
	}
}

func TestCountFestival_NormalFestivalStillCounts(t *testing.T) {
	tmp := t.TempDir()
	festDir := filepath.Join(tmp, "normal-NN0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "GOAL.md"), []byte("# Goal\n\nA planning document with prose.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeModelLikeFile(t, filepath.Join(festDir, "sample.bin"), 1<<10)

	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := tc.CountFestival(t.Context(), festDir); got <= 0 {
		t.Errorf("expected a positive token count for a small festival, got %d", got)
	}
	if n := cacheEntryCount(t, tmp); n != 1 {
		t.Errorf("expected 1 cache entry after counting, got %d", n)
	}
}

func TestWithinSizeLimits(t *testing.T) {
	tests := []struct {
		name string
		scan dirScan
		want bool
	}{
		{"empty", dirScan{}, true},
		{"small", dirScan{totalBytes: 4 << 10, largestFile: 4 << 10}, true},
		{"largest file at the cap", dirScan{totalBytes: maxFileBytes, largestFile: maxFileBytes}, true},
		{"single oversized file", dirScan{totalBytes: maxFileBytes + 1, largestFile: maxFileBytes + 1}, false},
		{"aggregate over cap", dirScan{totalBytes: maxFestivalBytes + 1, largestFile: 1 << 10}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withinSizeLimits(tt.scan); got != tt.want {
				t.Errorf("withinSizeLimits(%+v) = %v, want %v", tt.scan, got, tt.want)
			}
		})
	}
}

func TestScanDir_ReportsSizesAndFollowsSymlinks(t *testing.T) {
	tmp := t.TempDir()
	festDir := filepath.Join(tmp, "linked-LL0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "GOAL.md"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(tmp, "model.onnx")
	writeModelLikeFile(t, target, 3<<20)
	if err := os.Symlink(target, filepath.Join(festDir, "model.onnx")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	scan, err := scanDir(t.Context(), festDir)
	if err != nil {
		t.Fatalf("scanDir: %v", err)
	}
	if scan.largestFile != 3<<20 {
		t.Errorf("largestFile = %d, want the symlink target's size %d", scan.largestFile, 3<<20)
	}
	if withinSizeLimits(scan) {
		t.Error("a symlink to an oversized file should trip the guard")
	}
}

func TestCountFestivals_CancelledContextYieldsZeros(t *testing.T) {
	tmp := t.TempDir()
	festDir := filepath.Join(tmp, "fest-CC0001")
	if err := os.MkdirAll(festDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(festDir, "GOAL.md"), []byte("# Goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tc, err := NewCounter(t.Context(), tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	result := tc.CountFestivals(ctx, []string{festDir})
	if len(result) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result))
	}
	if result[festDir] != 0 {
		t.Errorf("cancelled context should yield 0, got %d", result[festDir])
	}
}
