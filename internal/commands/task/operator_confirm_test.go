package task

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Obedience-Corp/fest/internal/errors"
	"github.com/Obedience-Corp/fest/internal/progress"
	"github.com/Obedience-Corp/fest/internal/ui"
)

func stubOperatorPrompt(t *testing.T, input string) *bytes.Buffer {
	t.Helper()
	restoreIn, restoreOut := operatorPromptIn, operatorPromptOut
	out := &bytes.Buffer{}
	operatorPromptIn = strings.NewReader(input)
	operatorPromptOut = out
	t.Cleanup(func() {
		operatorPromptIn, operatorPromptOut = restoreIn, restoreOut
	})
	return out
}

func blockedTaskFixture(attempts []string) *progress.TaskProgress {
	blockedAt := time.Now().UTC().Add(-3 * time.Hour)
	return &progress.TaskProgress{
		TaskID:          "004_PHASE/02_seq/03_task.md",
		Status:          progress.StatusBlocked,
		BlockerMessage:  "provider API removed; warm-up not possible",
		BlockedAt:       &blockedAt,
		BlockerAttempts: attempts,
	}
}

func TestConfirmDeferralPromptWithAttempts(t *testing.T) {
	out := stubOperatorPrompt(t, "03\n")
	task := blockedTaskFixture([]string{"retried against the v2 endpoint", "asked the provider support channel"})

	if !confirmDeferral(task.TaskID, task) {
		t.Fatal("typing the task number must confirm")
	}

	rendered := out.String()
	for _, want := range []string{
		"provider API removed; warm-up not possible",
		"Blocked: 3h",
		"  Tried:",
		"    - retried against the v2 endpoint",
		"    - asked the provider support channel",
		"Type the task number (03) to defer",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("prompt missing %q\n--- prompt ---\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "No unblock attempts were recorded") {
		t.Error("prompt warned about missing attempts although attempts were recorded")
	}
}

func TestConfirmDeferralPromptWithoutAttempts(t *testing.T) {
	out := stubOperatorPrompt(t, "03\n")
	task := blockedTaskFixture(nil)

	if !confirmDeferral(task.TaskID, task) {
		t.Fatal("typing the task number must confirm")
	}

	rendered := out.String()
	warning := ui.Warning("  No unblock attempts were recorded. Consider 'fest task unblock --note' instead.")
	if !strings.Contains(rendered, warning) {
		t.Errorf("prompt missing the rendered warning\n--- prompt ---\n%s", rendered)
	}
	if strings.Contains(rendered, "Tried:") {
		t.Errorf("prompt showed a Tried heading with no attempts\n--- prompt ---\n%s", rendered)
	}
}

func TestConfirmDeferralAcceptsOnlyTheTaskNumber(t *testing.T) {
	cases := map[string]struct {
		input string
		want  bool
	}{
		"task number":  {input: "03\n", want: true},
		"wrong number": {input: "02\n", want: false},
		"y":            {input: "y\n", want: false},
		"yes":          {input: "yes\n", want: false},
		"empty line":   {input: "\n", want: false},
		"closed stdin": {input: "", want: false},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			stubOperatorPrompt(t, tc.input)
			task := blockedTaskFixture([]string{"tried once"})

			if got := confirmDeferral(task.TaskID, task); got != tc.want {
				t.Errorf("confirmDeferral() = %v, want %v for input %q", got, tc.want, tc.input)
			}
		})
	}
}

func TestTaskNumberFrom(t *testing.T) {
	cases := map[string]string{
		"004_PHASE/02_seq/03_task.md": "03",
		"03_task.md":                  "03",
		"10_task.md":                  "10",
		"task.md":                     "",
	}
	for taskID, want := range cases {
		if got := taskNumberFrom(taskID); got != want {
			t.Errorf("taskNumberFrom(%q) = %q, want %q", taskID, got, want)
		}
	}
}

// chdirTempForGit moves into a fresh empty directory that is not a git
// repository, so gitUserName sees only the pinned environment configuration.
// The restore falls back to the temp root because other tests in this package
// can leave the process cwd inside a directory that has already been removed.
func chdirTempForGit(t *testing.T) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		original = os.TempDir()
	}
	if err := os.Chdir(t.TempDir()); err != nil {
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

func TestGitUserNamePresentAndAbsent(t *testing.T) {
	t.Run("present", func(t *testing.T) {
		t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
		t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
		t.Setenv("GIT_CONFIG_COUNT", "1")
		t.Setenv("GIT_CONFIG_KEY_0", "user.name")
		t.Setenv("GIT_CONFIG_VALUE_0", "Fest Operator")
		chdirTempForGit(t)

		if got := gitUserName(t.Context()); got != "Fest Operator" {
			t.Errorf("gitUserName() = %q, want %q", got, "Fest Operator")
		}
	})

	t.Run("absent", func(t *testing.T) {
		t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
		t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
		t.Setenv("GIT_CONFIG_COUNT", "0")
		chdirTempForGit(t)

		if got := gitUserName(t.Context()); got != "" {
			t.Errorf("gitUserName() = %q, want empty when git has no user.name", got)
		}
	})
}

type fakeDeferrer struct {
	calls int
	err   error
	audit progress.DeferralAudit
}

func (f *fakeDeferrer) DeferBlocker(_ context.Context, _, _ string, audit progress.DeferralAudit) error {
	f.calls++
	f.audit = audit
	return f.err
}

type ledgerCall struct {
	festivalPath string
	taskID       string
	reason       string
	deferredBy   string
}

func captureDeferralLedger(t *testing.T) *[]ledgerCall {
	t.Helper()
	calls := &[]ledgerCall{}
	restore := deferralLedgerEmit
	deferralLedgerEmit = func(_ context.Context, festivalPath, taskID, reason, deferredBy string) {
		*calls = append(*calls, ledgerCall{festivalPath, taskID, reason, deferredBy})
	}
	t.Cleanup(func() { deferralLedgerEmit = restore })
	return calls
}

func TestApplyDeferralEmitsLedgerOnSuccessOnly(t *testing.T) {
	audit := &operatorAudit{
		Actor:        "operator",
		TTY:          true,
		AgentMarkers: operatorAgentMarkers,
		Ancestry:     []string{"zsh", "login"},
		DeferredBy:   "Fest Operator",
	}

	t.Run("success emits once", func(t *testing.T) {
		calls := captureDeferralLedger(t)
		mgr := &fakeDeferrer{}

		if err := applyDeferral(t.Context(), mgr, "/festivals/active/demo", "004_PHASE/02_seq/03_task.md", "provider is gone", audit); err != nil {
			t.Fatalf("applyDeferral() error = %v, want nil", err)
		}
		if mgr.calls != 1 {
			t.Fatalf("DeferBlocker called %d times, want 1", mgr.calls)
		}
		if mgr.audit.DeferredBy != "Fest Operator" || !mgr.audit.TTY || mgr.audit.Actor != "operator" {
			t.Errorf("audit passed to the manager = %+v, want the guard record", mgr.audit)
		}
		if strings.Join(mgr.audit.Ancestry, ",") != "zsh,login" {
			t.Errorf("Ancestry = %v, want the recorded chain", mgr.audit.Ancestry)
		}
		if strings.Join(mgr.audit.AgentMarkers, ",") != strings.Join(operatorAgentMarkers, ",") {
			t.Errorf("AgentMarkers = %v, want the checked markers", mgr.audit.AgentMarkers)
		}
		if len(*calls) != 1 {
			t.Fatalf("ledger emits = %d, want 1", len(*calls))
		}
		want := ledgerCall{"/festivals/active/demo", "004_PHASE/02_seq/03_task.md", "provider is gone", "Fest Operator"}
		if (*calls)[0] != want {
			t.Errorf("ledger emit = %+v, want %+v", (*calls)[0], want)
		}
	})

	t.Run("failed write emits nothing", func(t *testing.T) {
		calls := captureDeferralLedger(t)
		mgr := &fakeDeferrer{err: errors.Validation("only a blocked task can be deferred")}

		if err := applyDeferral(t.Context(), mgr, "/festivals/active/demo", "004_PHASE/02_seq/03_task.md", "provider is gone", audit); err == nil {
			t.Fatal("applyDeferral() error = nil, want the manager error")
		}
		if len(*calls) != 0 {
			t.Errorf("ledger emits = %d, want 0 after a failed write", len(*calls))
		}
	})
}

func TestEmitDeferralLedgerIgnoresEmptyFestivalPath(t *testing.T) {
	emitDeferralLedger(t.Context(), "", "004_PHASE/02_seq/03_task.md", "provider is gone", "Fest Operator")
}
