package task

import (
	"slices"
	"testing"
)

func TestBlockedTriedFlag(t *testing.T) {
	t.Run("repeatable and ordered", func(t *testing.T) {
		cmd := newBlockedCmd()
		args := []string{
			"--reason", "provider API removed",
			"--tried", "checked the changelog",
			"--tried", "asked the vendor",
			"--tried", "pinned the old version",
		}
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatalf("ParseFlags() error = %v", err)
		}
		want := []string{"checked the changelog", "asked the vendor", "pinned the old version"}
		if !slices.Equal(blockedTried, want) {
			t.Errorf("blockedTried = %q, want %q", blockedTried, want)
		}
	})

	t.Run("does not split on commas", func(t *testing.T) {
		cmd := newBlockedCmd()
		const attempt = "checked the vendor docs, then the changelog"
		if err := cmd.ParseFlags([]string{"--reason", "x", "--tried", attempt}); err != nil {
			t.Fatalf("ParseFlags() error = %v", err)
		}
		if len(blockedTried) != 1 {
			t.Fatalf("blockedTried = %q, want one entry", blockedTried)
		}
		if blockedTried[0] != attempt {
			t.Errorf("blockedTried[0] = %q, want %q", blockedTried[0], attempt)
		}
		if got := cmd.Flags().Lookup("tried").Value.Type(); got != "stringArray" {
			t.Errorf("tried flag type = %q, want stringArray", got)
		}
	})

	t.Run("optional and reset between constructions", func(t *testing.T) {
		first := newBlockedCmd()
		if err := first.ParseFlags([]string{"--reason", "x", "--tried", "a"}); err != nil {
			t.Fatalf("ParseFlags() error = %v", err)
		}
		if len(blockedTried) != 1 {
			t.Fatalf("setup: blockedTried = %q, want one entry", blockedTried)
		}

		second := newBlockedCmd()
		if blockedTried != nil {
			t.Errorf("blockedTried = %q, want nil after rebuilding the command", blockedTried)
		}
		if err := second.ParseFlags([]string{"--reason", "x"}); err != nil {
			t.Fatalf("ParseFlags() error = %v", err)
		}
		if blockedTried != nil {
			t.Errorf("blockedTried = %q, want nil with no --tried", blockedTried)
		}
	})
}
