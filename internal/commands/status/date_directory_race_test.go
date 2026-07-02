package status

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestMoveToDateDirectory_ConcurrentCompletionPreservesFestival(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	sourcePath := filepath.Join(root, "active", "race-fest-RF0001")
	if err := os.MkdirAll(filepath.Join(sourcePath, ".fest"), 0755); err != nil {
		t.Fatalf("setup source: %v", err)
	}
	marker := filepath.Join(sourcePath, ".fest", "status_history.json")
	want := []byte(`[{"from":"active","to":"completed"}]`)
	if err := os.WriteFile(marker, want, 0644); err != nil {
		t.Fatalf("setup marker: %v", err)
	}

	completedDir := filepath.Join(root, "dungeon", "completed")
	dateDir := "2026-07-01"

	var wg sync.WaitGroup
	results := make([]string, 2)
	errs := make([]error, 2)
	start := make(chan struct{})
	for i := range 2 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx], errs[idx] = MoveToDateDirectory(sourcePath, completedDir, dateDir)
		}(i)
	}
	close(start)
	wg.Wait()

	successes := 0
	var destPath string
	for i := range 2 {
		if errs[i] == nil {
			successes++
			destPath = results[i]
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one successful completion, got %d (errs: %v, %v)", successes, errs[0], errs[1])
	}

	got, err := os.ReadFile(filepath.Join(destPath, ".fest", "status_history.json"))
	if err != nil {
		t.Fatalf("festival lost after concurrent completion: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("festival content corrupted: got %q want %q", got, want)
	}

	if _, err := os.Stat(filepath.Join(completedDir, dateDir, "race-fest-RF0001.partial")); !os.IsNotExist(err) {
		t.Errorf("staging directory leaked after move")
	}
}

func TestCopyAndDelete_PreservesForeignDestination(t *testing.T) {
	root := resolvePath(t, t.TempDir())
	sourcePath := filepath.Join(root, "source", "fest")
	if err := os.MkdirAll(sourcePath, 0755); err != nil {
		t.Fatalf("setup source: %v", err)
	}

	destPath := filepath.Join(root, "dest", "fest")
	if err := os.MkdirAll(destPath, 0755); err != nil {
		t.Fatalf("setup dest: %v", err)
	}
	foreign := filepath.Join(destPath, "owned-by-other.txt")
	want := []byte("do not delete me")
	if err := os.WriteFile(foreign, want, 0644); err != nil {
		t.Fatalf("setup foreign file: %v", err)
	}

	if _, err := copyAndDelete(sourcePath, destPath); err == nil {
		t.Fatal("expected error moving into an existing destination")
	}

	got, err := os.ReadFile(foreign)
	if err != nil {
		t.Fatalf("foreign destination was removed: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("foreign destination corrupted: got %q want %q", got, want)
	}
}
