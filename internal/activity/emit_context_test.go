//go:build container_fs

package activity

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestEmit_CancelledContextDoesNotWrite(t *testing.T) {
	_, festivalPath := setupCampaign(t, "test-cancel")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var warned error
	e := NewFromFestival(ctx, festivalPath, func(err error) { warned = err })
	e.Emit(ctx, "task.created", Scope{}, "fest create task --name setup",
		WithData(map[string]any{"task": "01_setup.md"}))

	if warned == nil {
		t.Fatal("expected warn callback when context is cancelled")
	}
	if !errors.Is(warned, context.Canceled) {
		t.Fatalf("warn error = %v, want context.Canceled", warned)
	}

	festFile := filepath.Join(festivalPath, ".fest", FestivalFileName)
	data, err := os.ReadFile(festFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("reading festival activity log: %v", err)
	}
	if len(data) > 0 {
		t.Fatalf("cancelled emit must not persist activity, got %q", data)
	}
}
