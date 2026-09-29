package task

import (
	"context"
	stderrors "errors"
	"os"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/spf13/cobra"
)

func stubOperatorGuardTerminal(t *testing.T, isTerminal bool) {
	t.Helper()
	restore := operatorGuardStdinIsTerminal
	operatorGuardStdinIsTerminal = func() bool { return isTerminal }
	t.Cleanup(func() { operatorGuardStdinIsTerminal = restore })
}

func stubOperatorGuardAncestry(t *testing.T, chain []string, err error) {
	t.Helper()
	restore := operatorGuardAncestry
	operatorGuardAncestry = func(context.Context) ([]string, error) { return chain, err }
	t.Cleanup(func() { operatorGuardAncestry = restore })
}

func clearOperatorAgentMarkers(t *testing.T) {
	t.Helper()
	for _, marker := range operatorAgentMarkers {
		original, present := os.LookupEnv(marker)
		if !present {
			continue
		}
		if err := os.Unsetenv(marker); err != nil {
			t.Fatalf("unsetting %s: %v", marker, err)
		}
		t.Cleanup(func() {
			if err := os.Setenv(marker, original); err != nil {
				t.Errorf("restoring %s: %v", marker, err)
			}
		})
	}
}

func TestOperatorGuardRefusesOffTTY(t *testing.T) {
	stubOperatorGuardTerminal(t, false)
	stubOperatorGuardAncestry(t, []string{"zsh"}, nil)
	clearOperatorAgentMarkers(t)

	audit, err := OperatorGuard(t.Context(), "deferral")
	if err == nil {
		t.Fatal("expected refusal off a TTY")
	}
	if audit != nil {
		t.Error("refusal must return no audit record")
	}
	if got, want := err.Error(), "deferral is an operator decision; run this from your terminal"; !strings.Contains(got, want) {
		t.Errorf("message = %q, want it to contain %q", got, want)
	}
}

func TestOperatorGuardRefusesEachAgentMarker(t *testing.T) {
	stubOperatorGuardTerminal(t, true)
	stubOperatorGuardAncestry(t, []string{"zsh"}, nil)

	for _, marker := range []string{"OBEY_AGENT", "CLAUDE_CODE", "CODEX_TASK", "OBEY_SESSION_ID"} {
		t.Run(marker, func(t *testing.T) {
			clearOperatorAgentMarkers(t)
			t.Setenv(marker, "1")

			audit, err := OperatorGuard(t.Context(), "deferral")
			if err == nil {
				t.Fatalf("expected refusal with %s set", marker)
			}
			if audit != nil {
				t.Error("refusal must return no audit record")
			}
			if !strings.Contains(err.Error(), marker) {
				t.Errorf("message = %q, want it to name %s", err, marker)
			}
		})
	}
}

func TestOperatorGuardAllowsOperatorAndRecordsAbsentMarkers(t *testing.T) {
	stubOperatorGuardTerminal(t, true)
	stubOperatorGuardAncestry(t, []string{"zsh", "login", "init"}, nil)
	clearOperatorAgentMarkers(t)

	audit, err := OperatorGuard(t.Context(), "deferral")
	if err != nil {
		t.Fatalf("OperatorGuard() error = %v, want nil", err)
	}
	if audit == nil {
		t.Fatal("OperatorGuard() returned no audit record")
	}
	if audit.Actor != "operator" {
		t.Errorf("Actor = %q, want %q", audit.Actor, "operator")
	}
	if !audit.TTY {
		t.Error("TTY = false, want true")
	}
	if len(audit.AgentMarkers) != len(operatorAgentMarkers) {
		t.Fatalf("AgentMarkers = %v, want all %d markers recorded as absent", audit.AgentMarkers, len(operatorAgentMarkers))
	}
	for i, marker := range operatorAgentMarkers {
		if audit.AgentMarkers[i] != marker {
			t.Errorf("AgentMarkers[%d] = %q, want %q", i, audit.AgentMarkers[i], marker)
		}
	}
	if strings.Join(audit.Ancestry, ",") != "zsh,login,init" {
		t.Errorf("Ancestry = %v, want the clean chain recorded", audit.Ancestry)
	}
}

func TestOperatorGuardAncestryRefusesEachAgentBinary(t *testing.T) {
	stubOperatorGuardTerminal(t, true)

	for binary := range agentBinaries {
		t.Run(binary, func(t *testing.T) {
			clearOperatorAgentMarkers(t)
			stubOperatorGuardAncestry(t, []string{"zsh", binary, "login", "init"}, nil)

			audit, err := OperatorGuard(t.Context(), "deferral")
			if err == nil {
				t.Fatalf("expected refusal with %s in the parent chain", binary)
			}
			if audit != nil {
				t.Error("refusal must return no audit record")
			}
			if !strings.Contains(err.Error(), "agent process is in the parent chain") {
				t.Errorf("message = %q, want it to name the parent chain", err)
			}
			var festErr *errors.Error
			if !stderrors.As(err, &festErr) {
				t.Fatalf("error is not a project error: %T", err)
			}
			if got := festErr.Fields["ancestor"]; got != binary {
				t.Errorf("ancestor field = %v, want %q", got, binary)
			}
		})
	}
}

func TestOperatorGuardAncestryRefusesPathQualifiedBinary(t *testing.T) {
	stubOperatorGuardTerminal(t, true)
	clearOperatorAgentMarkers(t)
	stubOperatorGuardAncestry(t, []string{"zsh", "/usr/local/bin/claude", "login"}, nil)

	audit, err := OperatorGuard(t.Context(), "deferral")
	if err == nil {
		t.Fatal("expected refusal for a path-qualified agent binary")
	}
	if audit != nil {
		t.Error("refusal must return no audit record")
	}
	if !strings.Contains(err.Error(), "agent process is in the parent chain") {
		t.Errorf("message = %q, want it to name the parent chain", err)
	}
}

func TestOperatorGuardAncestryUnreadableRecordsUnknown(t *testing.T) {
	stubOperatorGuardTerminal(t, true)

	cases := map[string][]string{
		"no chain":      nil,
		"partial chain": {"zsh"},
	}

	for name, chain := range cases {
		t.Run(name, func(t *testing.T) {
			clearOperatorAgentMarkers(t)
			stubOperatorGuardAncestry(t, chain, errors.Validation("ps unavailable"))

			audit, err := OperatorGuard(t.Context(), "deferral")
			if err != nil {
				t.Fatalf("OperatorGuard() error = %v, want nil when ancestry is unreadable", err)
			}
			if audit == nil {
				t.Fatal("OperatorGuard() returned no audit record")
			}
			if strings.Join(audit.Ancestry, ",") != "unknown" {
				t.Errorf("Ancestry = %v, want [unknown]", audit.Ancestry)
			}
		})
	}
}

func TestReadProcessAncestryTerminatesOnCycle(t *testing.T) {
	restore := operatorGuardPSEntry
	operatorGuardPSEntry = func(context.Context, int) (int, string, error) {
		return os.Getppid(), "zsh", nil
	}
	t.Cleanup(func() { operatorGuardPSEntry = restore })

	chain, err := readProcessAncestry(t.Context())
	if err != nil {
		t.Fatalf("readProcessAncestry() error = %v, want nil", err)
	}
	if len(chain) != 1 {
		t.Errorf("chain = %v, want the walk to stop as soon as a pid repeats", chain)
	}
}

func TestReadProcessAncestryTerminatesOnDepthLimit(t *testing.T) {
	restore := operatorGuardPSEntry
	next := 1000
	operatorGuardPSEntry = func(context.Context, int) (int, string, error) {
		next++
		return next, "zsh", nil
	}
	t.Cleanup(func() { operatorGuardPSEntry = restore })

	chain, err := readProcessAncestry(t.Context())
	if err != nil {
		t.Fatalf("readProcessAncestry() error = %v, want nil", err)
	}
	if len(chain) != ancestryDepthLimit {
		t.Errorf("chain length = %d, want the depth limit %d", len(chain), ancestryDepthLimit)
	}
}

// assertNoAgentFlags is the shared assertion that a guarded verb offers no
// agent-friendly escape hatch. Design doc 04 guard 1 makes the absence of
// these two flags part of the contract, so it is asserted rather than assumed.
func assertNoAgentFlags(t *testing.T, cmd *cobra.Command) {
	t.Helper()
	for _, name := range []string{"yes", "json"} {
		if cmd.Flags().Lookup(name) != nil {
			t.Errorf("guarded verb must not have --%s", name)
		}
	}
}

func TestAssertNoAgentFlagsDetectsAgentFlags(t *testing.T) {
	clean := &cobra.Command{Use: "clean"}
	clean.Flags().String("reason", "", "why")
	assertNoAgentFlags(t, clean)

	for _, name := range []string{"yes", "json"} {
		t.Run(name, func(t *testing.T) {
			dirty := &cobra.Command{Use: "dirty"}
			dirty.Flags().Bool(name, false, "agent flag")

			spy := &testing.T{}
			assertNoAgentFlags(spy, dirty)
			if !spy.Failed() {
				t.Errorf("assertNoAgentFlags did not fail on --%s", name)
			}
		})
	}
}
