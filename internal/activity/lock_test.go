//go:build unix

package activity

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenLocked_ClosesHandleWhenLockFails(t *testing.T) {
	prev := lockActivityFile
	t.Cleanup(func() { lockActivityFile = prev })
	lockActivityFile = func(*os.File) error {
		return os.ErrPermission
	}

	path := filepath.Join(t.TempDir(), "activity.jsonl")
	f, err := openLocked(path)
	if err == nil {
		if f != nil {
			closeLocked(f)
		}
		t.Fatal("expected lock error")
	}
	if f != nil {
		t.Fatal("expected nil file on lock failure")
	}

	data, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("reading activity log: %v", readErr)
	}
	if len(data) > 0 {
		t.Fatalf("lock failure must not write activity data, got %q", data)
	}
}

func TestEmit_DoesNotReportSuccessfulWriteWhenLockFails(t *testing.T) {
	prev := lockActivityFile
	t.Cleanup(func() { lockActivityFile = prev })
	lockActivityFile = func(*os.File) error {
		return os.ErrPermission
	}

	_, festivalPath := setupCampaign(t, "test-lock-fail")
	var warned error
	e := NewFromFestival(context.Background(), festivalPath, func(err error) { warned = err })

	e.Emit(context.Background(), "task.created", Scope{}, "fest create task --name setup",
		WithData(map[string]any{"task": "01_setup.md"}))

	if warned == nil {
		t.Fatal("expected warn callback on lock failure")
	}
	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	data, err := os.ReadFile(festFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("reading festival activity log: %v", err)
	}
	if len(data) > 0 {
		t.Fatalf("expected no persisted activity on lock failure, got %q", data)
	}
	if !errors.Is(warned, os.ErrPermission) {
		t.Fatalf("warn error = %v, want cause %v", warned, os.ErrPermission)
	}
}
