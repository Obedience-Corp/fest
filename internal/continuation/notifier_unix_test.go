//go:build unix

package continuation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestCLINotifierSendsEnvelopeOnStdin exercises the thin real implementation
// against a stand-in executable, asserting the 'session notify' subcommand and
// that the full JSON envelope arrives on stdin. A stand-in avoids depending on
// an installed obey while still testing the exec wrapper end to end.
func TestCLINotifierSendsEnvelopeOnStdin(t *testing.T) {
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args.txt")
	stdinPath := filepath.Join(dir, "stdin.json")
	binPath := filepath.Join(dir, "obey-fake")

	script := "#!/bin/sh\n" +
		"printf '%s' \"$*\" > " + argsPath + "\n" +
		"cat > " + stdinPath + "\n"
	if err := os.WriteFile(binPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake obey: %v", err)
	}

	envelope := BuildNotification(approvalResult())
	if err := (CLINotifier{Binary: binPath}).Notify(context.Background(), envelope); err != nil {
		t.Fatalf("Notify: %v", err)
	}

	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("read args: %v", err)
	}
	if string(args) != "session notify" {
		t.Fatalf("args = %q, want \"session notify\"", string(args))
	}

	raw, err := os.ReadFile(stdinPath)
	if err != nil {
		t.Fatalf("read stdin: %v", err)
	}
	var got SessionNotification
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("stdin is not the JSON envelope: %v (%s)", err, raw)
	}
	if got.DeliveryID != envelope.DeliveryID || got.TargetSessionID != envelope.TargetSessionID {
		t.Fatalf("stdin envelope mismatch: %+v", got)
	}
}
