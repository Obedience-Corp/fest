package commands

import (
	"context"
	"errors"
	"testing"

	"github.com/Obedience-Corp/fest/internal/hooks"
)

// Exercise the same execution boundary used by ExecuteContext without loading
// machine config or writing fixtures. Defers in the command release locks first.
func TestNotificationExecutionBoundary(t *testing.T) {
	for _, failed := range []bool{false, true} {
		ctx, q := hooks.WithNotifications(context.Background())
		locked := false
		result := errors.New("later rendering failed")
		calls := 0
		run := func(context.Context) error {
			locked = true
			defer func() { locked = false }()
			hooks.QueueNotification(ctx, hooks.Notification{Name: hooks.PresentationOpenName})
			if failed {
				return result
			}
			return nil
		}
		err := executeWithNotifications(ctx, q, run, func(context.Context, hooks.Notification) {
			if locked {
				t.Fatal("hook ran inside checkpoint lock")
			}
			calls++
		})
		if calls != 1 || (failed && !errors.Is(err, result)) || (!failed && err != nil) {
			t.Fatalf("calls=%d err=%v", calls, err)
		}
	}
}
