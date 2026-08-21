package festival

import (
	"context"
	"errors"
	"testing"
)

func TestPreviewCreateTask_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := previewCreateTask(ctx, &CreateTaskOptions{Names: []string{"setup"}}, ".", "", "", nil, nil, nil, nil, false)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRunCreateSequence_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RunCreateSequence(ctx, &CreateSequenceOptions{Name: "hello", DryRun: true})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestRunCreateTask_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := RunCreateTask(ctx, &CreateTaskOptions{Names: []string{"setup"}, DryRun: true})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}
