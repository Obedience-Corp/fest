package commands

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
)

// Execute runs the root command with SIGINT/SIGTERM bound to context cancel.
//
// Convention: a user stop (Ctrl+C / SIGTERM) cancels cmd.Context() instead of
// killing the process by signal. Long-running watch loops such as
// `fest list --watch` already return nil on ctx.Done(), so the process exits 0
// rather than 130. A second Ctrl+C after the first cancel re-arms the default
// handler so a wedged command can still be force-killed.
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		stop()
	}()
	defer stop()
	return ignoreCanceled(ExecuteContext(ctx))
}

// ExecuteContext runs the root command against ctx. Tests use this to inject
// cancelation without installing process-wide signal handlers.
func ExecuteContext(ctx context.Context) error {
	os.Args = normalizeAutoWriteAlias(os.Args)

	// Try git-style plugin dispatch for unknown subcommands.
	// A fest-<name> binary on PATH becomes "fest <name> [args...]".
	if err := dispatchPlugin(); err != nil {
		if errors.Is(err, errPluginHandled) {
			return nil
		}
		return err
	}

	return rootCmd.ExecuteContext(ctx)
}

// ignoreCanceled treats a user-initiated context cancel as a successful stop
// so main does not print "Error: context canceled" or exit 1.
func ignoreCanceled(err error) error {
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
