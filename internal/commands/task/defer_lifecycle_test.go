package task

import (
	stderrors "errors"
	"os"
	"strings"
	"testing"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/scope"
	"github.com/spf13/cobra"
)

// chdirForFixture enters a fixture festival and restores the previous working
// directory, falling back to the temp root because sibling tests in this
// package can leave the process cwd inside a directory that was removed.
func chdirForFixture(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		original = os.TempDir()
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			if fallbackErr := os.Chdir(os.TempDir()); fallbackErr != nil {
				t.Errorf("restoring cwd: %v", fallbackErr)
			}
		}
	})
}

func runDeferInFixture(t *testing.T, status, phaseType string) error {
	t.Helper()
	festDir, taskRel := setupTaskFixture(t, status, phaseType)
	chdirForFixture(t, festDir)

	cmd := &cobra.Command{}
	cmd.SetContext(scope.WithFestival(t.Context(), festDir))

	deferReason = "the vendor replies next week"
	t.Cleanup(func() { deferReason = "" })

	devnull, _ := os.Open(os.DevNull)
	origOut := os.Stdout
	os.Stdout = devnull
	err := runDefer(cmd, []string{taskRel})
	os.Stdout = origOut
	_ = devnull.Close()

	return err
}

func TestDeferRefusedInNonActiveFestival(t *testing.T) {
	for _, status := range []string{"planning", "ready"} {
		t.Run(status, func(t *testing.T) {
			err := runDeferInFixture(t, status, "implementation")
			if err == nil {
				t.Fatalf("fest task defer must be refused in a %s festival", status)
			}
			if !stderrors.Is(err, errors.ErrAlreadyPrinted) {
				t.Errorf("error = %v, want the lifecycle gate refusal", err)
			}
		})
	}
}

func TestDeferInActiveFestivalStillPassesThroughTheOperatorGuard(t *testing.T) {
	stubOperatorGuardTerminal(t, false)

	err := runDeferInFixture(t, "active", "implementation")
	if err == nil {
		t.Fatal("fest task defer off a terminal must be refused even in an active festival")
	}
	if !strings.Contains(err.Error(), "run this from your terminal") {
		t.Errorf("error = %v, want the operator guard refusal", err)
	}
}
