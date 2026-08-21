package commands

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

func TestIgnoreCanceled(t *testing.T) {
	t.Parallel()

	if got := ignoreCanceled(nil); got != nil {
		t.Fatalf("ignoreCanceled(nil) = %v, want nil", got)
	}
	if got := ignoreCanceled(context.Canceled); got != nil {
		t.Fatalf("ignoreCanceled(context.Canceled) = %v, want nil", got)
	}
	wrapped := errors.Join(errors.New("watch stopped"), context.Canceled)
	if got := ignoreCanceled(wrapped); got != nil {
		t.Fatalf("ignoreCanceled(wrapped Canceled) = %v, want nil", got)
	}
	plain := errors.New("boom")
	if got := ignoreCanceled(plain); !errors.Is(got, plain) {
		t.Fatalf("ignoreCanceled(plain) = %v, want the original error", got)
	}
	if got := ignoreCanceled(context.DeadlineExceeded); !errors.Is(got, context.DeadlineExceeded) {
		t.Fatalf("ignoreCanceled(DeadlineExceeded) = %v, want DeadlineExceeded (timeouts are not a user stop)", got)
	}
}

func TestExecuteContext_CanceledContextIsUserStop(t *testing.T) {
	cmd := &cobra.Command{
		Use:   "ctx-check-list-watch-cancel",
		Short: "test helper",
		Annotations: map[string]string{
			"scope": string(scope.Global),
		},
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			<-cmd.Context().Done()
			return cmd.Context().Err()
		},
	}
	rootCmd.AddCommand(cmd)
	t.Cleanup(func() {
		rootCmd.RemoveCommand(cmd)
		rootCmd.SetArgs(nil)
	})

	oldArgs := os.Args
	os.Args = []string{"fest", cmd.Use}
	t.Cleanup(func() { os.Args = oldArgs })
	rootCmd.SetArgs([]string{cmd.Use})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ExecuteContext(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("ExecuteContext(canceled) error = %v, want context.Canceled", err)
	}
	if got := ignoreCanceled(err); got != nil {
		t.Fatalf("ignoreCanceled(ExecuteContext) = %v, want nil so Ctrl+C exits 0", got)
	}
}
