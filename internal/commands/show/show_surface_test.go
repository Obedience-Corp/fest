package show

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestShowStatusSubcommandsHiddenAndDeprecated(t *testing.T) {
	cmd := NewShowCommand()
	want := map[string]string{
		"active":    "fest list active",
		"planning":  "fest list planning",
		"completed": "fest list completed",
		"dungeon":   "fest list dungeon",
		"all":       "fest list all",
	}
	seen := map[string]bool{}
	for _, sub := range cmd.Commands() {
		wantHint, tracked := want[sub.Name()]
		if !tracked {
			continue
		}
		seen[sub.Name()] = true
		if !sub.Hidden {
			t.Errorf("subcommand %q should be hidden", sub.Name())
		}
		if !strings.Contains(sub.Deprecated, wantHint) {
			t.Errorf("subcommand %q deprecation = %q, want it to point at %q", sub.Name(), sub.Deprecated, wantHint)
		}
	}
	for name := range want {
		if !seen[name] {
			t.Errorf("expected status subcommand %q to still be registered", name)
		}
	}
}

func TestShowActiveAliasFunctionalAndDeprecated(t *testing.T) {
	campaignRoot, _ := makeShowPickerCampaign(t)
	restore := chdirForShowTest(t, campaignRoot)
	defer restore()

	cmd := NewShowCommand()
	var deprecationOut bytes.Buffer
	cmd.SetOut(&deprecationOut)
	cmd.SetArgs([]string{"active", "--json"})

	jsonOut := captureShowStdout(t, func() error { return cmd.Execute() })

	if !strings.Contains(jsonOut, `"status": "active"`) || !strings.Contains(jsonOut, `"festivals"`) {
		t.Fatalf("`fest show active --json` lost its output shape:\n%s", jsonOut)
	}
	if !strings.Contains(jsonOut, "launch-readiness-LR0001") {
		t.Fatalf("alias should still list the active festival:\n%s", jsonOut)
	}
	if !strings.Contains(deprecationOut.String(), "deprecated") ||
		!strings.Contains(deprecationOut.String(), "fest list active") {
		t.Fatalf("deprecation notice should point to `fest list active`, got %q", deprecationOut.String())
	}
}

func TestShowPositionalCompletionReturnsFestivalSelectors(t *testing.T) {
	campaignRoot, _ := makeShowPickerCampaign(t)
	restore := chdirForShowTest(t, campaignRoot)
	defer restore()

	got, _ := completeShowFestivalSelector(nil, nil, "")
	found := false
	for _, name := range got {
		if name == "launch-readiness-LR0001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("positional completion should offer festival selectors, got %v", got)
	}
}

func TestShowPositionalCompletionSuppressedAfterFirstArg(t *testing.T) {
	got, _ := completeShowFestivalSelector(nil, []string{"already-picked"}, "")
	if len(got) != 0 {
		t.Errorf("no completion expected past the first positional, got %v", got)
	}
}

func TestShowCompletionsListsFestivals(t *testing.T) {
	campaignRoot, _ := makeShowPickerCampaign(t)
	restore := chdirForShowTest(t, campaignRoot)
	defer restore()

	plain := captureShowStdout(t, func() error { return runShowCompletions(context.Background(), false) })
	if !strings.Contains(plain, "launch-readiness-LR0001") {
		t.Fatalf("plain completions missing festival name:\n%s", plain)
	}

	colored := captureShowStdout(t, func() error { return runShowCompletions(context.Background(), true) })
	name, _, hasTab := strings.Cut(strings.TrimSpace(colored), "\t")
	if !hasTab || name != "launch-readiness-LR0001" {
		t.Fatalf("colored completions should be value<TAB>display pairs, got %q", colored)
	}
}

func TestShowPick_NonTTYExitsNonZeroWithEmptyStdout(t *testing.T) {
	campaignRoot, _ := makeShowPickerCampaign(t)
	restore := chdirForShowTest(t, campaignRoot)
	defer restore()

	out, err := captureShowStdoutAllowError(t, func() error {
		return runShowPick(context.Background(), false)
	})
	if err == nil {
		t.Fatal("non-tty `fest show pick` should exit non-zero")
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("non-tty `fest show pick` must not print to stdout, got %q", out)
	}
}
