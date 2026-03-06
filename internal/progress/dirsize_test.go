package progress

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestComputeDirectorySize(t *testing.T) {
	dir := t.TempDir()

	// Create some .md files
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "b.md"), []byte("world!"), 0644)
	// Non-.md files should be ignored
	_ = os.WriteFile(filepath.Join(dir, "c.txt"), []byte("ignored"), 0644)

	size, err := ComputeDirectorySize(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "hello" = 5 bytes, "world!" = 6 bytes
	if size != 11 {
		t.Errorf("expected 11 bytes, got %d", size)
	}
}

func TestComputeDirectorySizeNested(t *testing.T) {
	dir := t.TempDir()

	subDir := filepath.Join(dir, "sub")
	_ = os.MkdirAll(subDir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "root.md"), []byte("abc"), 0644)
	_ = os.WriteFile(filepath.Join(subDir, "nested.md"), []byte("defgh"), 0644)

	size, err := ComputeDirectorySize(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "abc" = 3, "defgh" = 5
	if size != 8 {
		t.Errorf("expected 8 bytes, got %d", size)
	}
}

func TestComputeDirectorySizeSkipsHidden(t *testing.T) {
	dir := t.TempDir()

	hiddenDir := filepath.Join(dir, ".fest")
	_ = os.MkdirAll(hiddenDir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "visible.md"), []byte("abc"), 0644)
	_ = os.WriteFile(filepath.Join(hiddenDir, "hidden.md"), []byte("should-be-ignored"), 0644)

	size, err := ComputeDirectorySize(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 3 {
		t.Errorf("expected 3 bytes (hidden dir skipped), got %d", size)
	}
}

func TestComputeDirectorySizeEmpty(t *testing.T) {
	dir := t.TempDir()

	size, err := ComputeDirectorySize(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if size != 0 {
		t.Errorf("expected 0 bytes for empty dir, got %d", size)
	}
}

func TestComputeDirectorySizeCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello"), 0644)

	_, err := ComputeDirectorySize(ctx, dir)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestComputeContentDelta(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("hello world!"), 0644) // 12 bytes

	delta, err := ComputeContentDelta(context.Background(), dir, 8)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if delta.InitialBytes != 8 {
		t.Errorf("expected initial=8, got %d", delta.InitialBytes)
	}
	if delta.CurrentBytes != 12 {
		t.Errorf("expected current=12, got %d", delta.CurrentBytes)
	}
	if delta.DeltaBytes != 4 {
		t.Errorf("expected delta=4, got %d", delta.DeltaBytes)
	}
	if delta.DeltaTokens != 1 {
		t.Errorf("expected delta_tokens=1, got %d", delta.DeltaTokens)
	}
}

func TestComputeContentDeltaNegative(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.md"), []byte("hi"), 0644) // 2 bytes

	delta, err := ComputeContentDelta(context.Background(), dir, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if delta.DeltaBytes >= 0 {
		t.Errorf("expected negative delta, got %d", delta.DeltaBytes)
	}
}
