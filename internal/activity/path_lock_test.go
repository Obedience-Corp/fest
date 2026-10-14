//go:build container_fs

package activity

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActivityPathLockNotBesideLogFile(t *testing.T) {
	lockRoot := t.TempDir()
	t.Setenv(activityPathLockDirEnv, lockRoot)

	_, festivalPath := setupCampaign(t, "path-lock")
	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)

	e := NewFromFestival(t.Context(), festivalPath, func(error) {})
	e.Emit(t.Context(), "task.completed", Scope{Task: "01_task.md"}, "fest task completed", nil)

	sibling := festFile + ".lock"
	if _, err := os.Stat(sibling); err == nil {
		t.Fatalf("must not create workspace lock sibling %s", sibling)
	}

	entries, err := os.ReadDir(lockRoot)
	if err != nil {
		t.Fatal(err)
	}
	var lockFiles []string
	for _, ent := range entries {
		if strings.HasSuffix(ent.Name(), ".lock") {
			lockFiles = append(lockFiles, ent.Name())
		}
	}
	if len(lockFiles) != 1 {
		t.Fatalf("expected exactly one cache lock file, got %v", lockFiles)
	}

	lockPath, err := activityLockPath(festFile)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(lockPath) != lockRoot {
		t.Fatalf("lock path dir = %q, want %q", filepath.Dir(lockPath), lockRoot)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("expected lock file at %s: %v", lockPath, err)
	}
}
