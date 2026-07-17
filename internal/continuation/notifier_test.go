package continuation

import (
	"context"
	"strings"
	"testing"
)

func TestCLINotifierMissingBinaryReturnsError(t *testing.T) {
	n := CLINotifier{Binary: "obey-not-installed-xyz"}
	err := n.Notify(context.Background(), BuildNotification(approvalResult()))
	if err == nil {
		t.Fatal("expected error when the obey binary is absent")
	}
	if !strings.Contains(err.Error(), "resolving obey binary") {
		t.Fatalf("error = %v, want missing-binary wrap", err)
	}
}

func TestCLINotifierCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := (CLINotifier{}).Notify(ctx, BuildNotification(approvalResult())); err == nil {
		t.Fatal("expected canceled context to abort before submit")
	}
}
