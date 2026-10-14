package task

import (
	"io"
	"strings"
	"testing"
)

func TestDeferCmdLongTextExplainsNoninteractiveUse(t *testing.T) {
	long := newDeferCmd().Long
	for _, want := range []string{"--yes", "--json requires --yes", "automation", "fest task unblock"} {
		if !strings.Contains(long, want) {
			t.Errorf("help text missing %q:\n%s", want, long)
		}
	}
}

func TestDeferCmdRequiresReason(t *testing.T) {
	cmd := newDeferCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(nil)

	err := cmd.ExecuteContext(t.Context())
	if err == nil {
		t.Fatal("fest task defer without --reason must fail")
	}
	if !strings.Contains(err.Error(), "reason") {
		t.Errorf("error = %q, want it to name the required reason flag", err)
	}
}
