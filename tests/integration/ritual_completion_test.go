//go:build integration
// +build integration

package integration

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/stretchr/testify/require"
)

// TestRitualRunCompletion exercises the ValidArgsFunction wired into
// `fest ritual run` end-to-end through cobra's hidden __complete subcommand.
// Per repo policy (see CLAUDE.md and feedback memory), filesystem-mutating
// tests must run in the container harness, not against host t.TempDir().
func TestRitualRunCompletion(t *testing.T) {
	container := GetSharedContainer(t)

	const (
		workspaceRoot = "/workspace"
		festivalsRoot = workspaceRoot + "/festivals"
		ritualRoot    = festivalsRoot + "/ritual"
	)

	// Set up a workspace marker so workspace.FindFestivals resolves.
	_, err := container.runCommand([]string{
		"sh", "-c",
		fmt.Sprintf("mkdir -p %s/.festival/.state %s/active %s",
			festivalsRoot, festivalsRoot, ritualRoot),
	})
	require.NoError(t, err, "should create workspace dirs")
	writeContainerFile(t, container,
		festivalsRoot+"/.festival/.state/.workspace",
		`{"workspace": "workspace", "registered": "2024-01-01T00:00:00Z"}`)

	rituals := []string{
		"daily-cleanup-RI-DC0001",
		"weekly-review-RI-WR0001",
		"monthly-audit-RI-MA0001",
	}
	for _, r := range rituals {
		_, err := container.runCommand([]string{
			"sh", "-c",
			fmt.Sprintf("mkdir -p %s/%s", ritualRoot, r),
		})
		require.NoError(t, err, "should create ritual %s", r)
	}
	// A directory without a valid ID suffix must be ignored by the navigator.
	_, err = container.runCommand([]string{
		"sh", "-c",
		fmt.Sprintf("mkdir -p %s/not-a-ritual", ritualRoot),
	})
	require.NoError(t, err, "should create non-festival dir")

	t.Run("empty input returns all rituals", func(t *testing.T) {
		got := completeRitualRun(t, container, workspaceRoot, "")
		require.ElementsMatch(t, rituals, got)
	})

	t.Run("prefix match", func(t *testing.T) {
		got := completeRitualRun(t, container, workspaceRoot, "daily")
		require.Equal(t, []string{"daily-cleanup-RI-DC0001"}, got)
	})

	t.Run("case-insensitive substring match", func(t *testing.T) {
		got := completeRitualRun(t, container, workspaceRoot, "REVIEW")
		require.Equal(t, []string{"weekly-review-RI-WR0001"}, got)
	})

	t.Run("ID suffix substring match", func(t *testing.T) {
		got := completeRitualRun(t, container, workspaceRoot, "MA0001")
		require.Equal(t, []string{"monthly-audit-RI-MA0001"}, got)
	})

	t.Run("no matches returns empty", func(t *testing.T) {
		got := completeRitualRun(t, container, workspaceRoot, "nonexistent")
		require.Empty(t, got)
	})

	t.Run("does not complete past first positional arg", func(t *testing.T) {
		// ExactArgs(1) rejects a second argument; ValidArgsFunction must
		// stop offering completions once the slot is filled.
		got := completeRitualRunMulti(t, container, workspaceRoot,
			[]string{"daily-cleanup-RI-DC0001", ""})
		require.Empty(t, got, "expected no completions once first arg is set")
	})

	t.Run("missing workspace returns empty", func(t *testing.T) {
		// Run from /tmp where there is no festivals/ tree — completion
		// should degrade silently to no file completion.
		got := completeRitualRun(t, container, "/tmp", "")
		require.Empty(t, got)
	})
}

// completeRitualRun invokes `fest __complete ritual run <toComplete>` from the
// given working directory inside the container and returns the parsed
// completion suggestions (directives and stderr stripped).
func completeRitualRun(t *testing.T, tc *TestContainer, dir, toComplete string) []string {
	t.Helper()
	return completeRitualRunMulti(t, tc, dir, []string{toComplete})
}

// completeRitualRunMulti is like completeRitualRun but accepts multiple
// trailing args, useful for verifying ValidArgsFunction guards on filled args.
func completeRitualRunMulti(t *testing.T, tc *TestContainer, dir string, trailing []string) []string {
	t.Helper()

	parts := []string{"__complete", "ritual", "run"}
	parts = append(parts, trailing...)
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, shellQuote(p))
	}
	cmd := fmt.Sprintf("cd %s && /fest %s", shellQuote(dir), strings.Join(quoted, " "))

	exitCode, reader, err := tc.container.Exec(tc.ctx, []string{"sh", "-c", cmd})
	require.NoError(t, err, "exec __complete failed")

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, reader)
	require.NoError(t, err, "demux __complete output")
	require.Equal(t, 0, exitCode,
		"__complete exited %d:\nstdout=%s\nstderr=%s",
		exitCode, stdout.String(), stderr.String())

	var suggestions []string
	for _, line := range strings.Split(stdout.String(), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		// Strip cobra's optional "\tdescription" suffix.
		if tab := strings.IndexByte(line, '\t'); tab >= 0 {
			line = line[:tab]
		}
		suggestions = append(suggestions, line)
	}
	sort.Strings(suggestions)
	return suggestions
}
