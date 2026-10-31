//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/moby/moby/api/pkg/stdcopy"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkflowNotifications(t *testing.T) {
	tc := GetSharedContainer(t)
	root := "/festivals/notification fixture $literal"
	phase := root + "/001_INGEST"
	configDir := "/tmp/fest-notification-config"
	setup := `
set -eu
mkdir -p "$1/001_INGEST" /festivals/.festival /tmp/fest-notification-config /tmp/fest-notification-bin
cat > "$1/FESTIVAL_OVERVIEW.md" <<'MD'
---
fest_type: festival
fest_id: NT0001
fest_name: notification-fixture
fest_status: active
---
# Notification fixture
A deterministic workflow notification fixture.
MD
cat > "$1/fest.yaml" <<'YAML'
version: "1.0"
metadata:
  id: NT0001
  name: notification-fixture
  status: active
  status_history:
    - status: active
      timestamp: 2026-10-30T00:00:00Z
YAML
cat > "$1/001_INGEST/PHASE_GOAL.md" <<'MD'
---
fest_type: phase_goal
fest_id: 001_INGEST
fest_mode: ingest
fest_phase_type: ingest
---
# Notification fixture
MD
cat > "$1/001_INGEST/WORKFLOW.md" <<'MD'
# Notification workflow

## Step 1: READ
**Goal:** Read inputs
**Actions:**
1. Read
**Output:** Notes
**Checkpoint:** None

## Step 2: PRESENT
**Goal:** Show review
**Actions:**
1. Show
**Output:** Review
**Checkpoint:** User approval required
**Approval:** human-required

## Step 3: FINALIZE
**Goal:** Finish
**Actions:**
1. Finish
**Output:** Done
**Checkpoint:** None
MD
cat > /tmp/fest-notification-bin/camp <<'SH'
#!/bin/sh
set -eu
[ "$1" = app ] && [ "$2" = open ] && [ "$3" = . ]
pwd >> /tmp/fest-notification-cwds
cat >> /tmp/fest-notification-calls
echo HOOK_STDOUT_MUST_NOT_LEAK
SH
chmod +x /tmp/fest-notification-bin/camp
printf '{}' > /tmp/fest-notification-config/config.json
: > /tmp/fest-notification-calls
: > /tmp/fest-notification-cwds
`
	_, err := tc.runCommand([]string{"sh", "-c", setup, "sh", root})
	require.NoError(t, err)
	run := func(args ...string) (string, error) {
		command := []string{"sh", "-c", `cd "$1"; shift; export PATH=/tmp/fest-notification-bin:$PATH; export FEST_CONFIG_DIR=/tmp/fest-notification-config; exec /fest "$@"`, "sh", phase}
		code, reader, err := tc.container.Exec(tc.ctx, append(command, args...))
		if err != nil {
			return "", err
		}
		var stdout, stderr bytes.Buffer
		if _, err = stdcopy.StdCopy(&stdout, &stderr, reader); err != nil {
			return "", err
		}
		out := stdout.String() + stderr.String()
		if code != 0 {
			return out, fmt.Errorf("fest exited %d: %s", code, out)
		}
		return out, nil
	}
	calls := func() []map[string]any {
		t.Helper()
		out, err := tc.runCommand([]string{"cat", "/tmp/fest-notification-calls"})
		require.NoError(t, err)
		var entries []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			if line == "" {
				continue
			}
			var entry map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &entry))
			entries = append(entries, entry)
		}
		return entries
	}
	writeConfig := func(command string) {
		t.Helper()
		def := map[string]any{"enabled": true, "command": command, "fail": "open", "timeout": "1s"}
		b, err := json.Marshal(map[string]any{"hooks": map[string]any{"definitions": map[string]any{"presentation_open": def, "approval_notify": def}}})
		require.NoError(t, err)
		_, err = tc.runCommand([]string{"sh", "-c", `printf '%s' "$1" > "$2/config.json"`, "sh", string(b), configDir})
		require.NoError(t, err)
	}
	// With no opt-in, reaching PRESENT has no side effect.
	out, err := run("workflow", "advance")
	require.NoError(t, err, out)
	require.Empty(t, calls())
	writeConfig("camp app open .")
	// Enabling on an existing PRESENT does not retroactively notify.
	for _, args := range [][]string{{"workflow", "status", "--json"}, {"workflow", "show"}, {"next"}} {
		out, err = run(args...)
		require.NoError(t, err, out)
		require.NotContains(t, out, "HOOK_STDOUT")
	}
	require.Empty(t, calls())
	out, err = run("workflow", "reset", "--force")
	require.NoError(t, err, out)
	out, err = run("workflow", "advance")
	require.NoError(t, err, out)
	require.NotContains(t, out, "HOOK_STDOUT")
	entries := calls()
	require.Len(t, entries, 2, out)
	require.Equal(t, "workflow_present", entries[0]["verb"])
	require.Equal(t, "workflow_approval_needed", entries[1]["verb"])
	require.Equal(t, float64(2), entries[1]["step"])
	require.Equal(t, float64(2), entries[0]["step"])
	require.Equal(t, root, entries[0]["festival_path"])
	cwd, err := tc.runCommand([]string{"cat", "/tmp/fest-notification-cwds"})
	require.NoError(t, err)
	require.Equal(t, root+"\n"+root, strings.TrimSpace(cwd))
	// Opening did not grant human approval. Non-TTY approval remains refused.
	out, err = run("workflow", "approve")
	require.Error(t, err)
	require.Len(t, calls(), 2)
	// Successful approval does not notify: both calls already happened while
	// the agent needed human approval. The human-required configuration stayed blocked.
	_, err = tc.runCommand([]string{"sh", "-c", `sed -i '/Approval:.*human-required/d' "$1/WORKFLOW.md"`, "sh", phase})
	require.NoError(t, err)
	out, err = run("workflow", "approve")
	require.NoError(t, err, out)
	entries = calls()
	require.Len(t, entries, 2, out)
	require.Equal(t, "workflow_approval_needed", entries[1]["verb"])
	require.Equal(t, float64(2), entries[1]["step"])
	out, err = run("workflow", "status", "--json")
	require.NoError(t, err, out)
	require.Len(t, calls(), 2)
	// A failing optional notification cannot undo a later entry.
	writeConfig("missing-fest-notification-command")
	out, err = run("workflow", "reset", "--force")
	require.NoError(t, err, out)
	out, err = run("workflow", "advance")
	require.NoError(t, err, out)
	require.Contains(t, out, "Warning: presentation_open")
	require.Contains(t, out, "workflow progress is saved")
	out, err = run("workflow", "status", "--json")
	require.NoError(t, err, out)
	require.True(t, json.Valid([]byte(out)), fmt.Sprintf("invalid JSON: %s", out))
	require.NotContains(t, out, "Warning:")
	// Malformed enabled notification fields stay out of ordinary hook resolution.
	_, err = tc.runCommand([]string{"sh", "-c", `printf '%s' '{"hooks":{"definitions":{"presentation_open":{"enabled":true,"command":"x","timeout":"bad"}}}}' > /tmp/fest-notification-config/config.json`})
	require.NoError(t, err)
	out, err = run("hooks", "list", "--json")
	require.NoError(t, err, out)
	require.True(t, json.Valid([]byte(out)))
	out, err = run("hooks", "notifications", "--json")
	require.NoError(t, err, out)
	require.Contains(t, out, "timeout must be positive")

	// First-step initialization is a real entry; subsequent status reads are not.
	writeConfig("camp app open .")
	previous := len(calls())
	newPhase := root + "/002_INGEST"
	_, err = tc.runCommand([]string{"sh", "-c", `mkdir -p "$1"; sed 's/001_INGEST/002_INGEST/g' "$2/PHASE_GOAL.md" > "$1/PHASE_GOAL.md"; printf '%s\n' '# Workflow' '## Step 1: PRESENT' '**Goal:** Show review' '**Actions:**' '1. Review' '**Output:** Review notes' '**Checkpoint:** User approval required' '**Approval:** human-required' > "$1/WORKFLOW.md"`, "sh", newPhase, phase})
	require.NoError(t, err)
	phase = newPhase
	out, err = run("workflow", "status", "--json")
	require.NoError(t, err, out)
	require.True(t, json.Valid([]byte(out)))
	require.Len(t, calls(), previous+2)
	out, err = run("workflow", "status", "--json")
	require.NoError(t, err, out)
	require.Len(t, calls(), previous+2)
	out, err = run("workflow", "reset", "--force")
	require.NoError(t, err, out)
	require.Len(t, calls(), previous+4)

	// An artifact checkpoint with a configured delegate does not need a human.
	// Use status initialization so this test never invokes the dummy judge.
	beforeJudge := len(calls())
	judgePhase := root + "/003_REVIEW"
	_, err = tc.runCommand([]string{"sh", "-c", `mkdir -p "$1"; sed 's/002_INGEST/003_REVIEW/g' "$2/PHASE_GOAL.md" > "$1/PHASE_GOAL.md"; printf '%s\n' '# Workflow' '## Step 1: REVIEW' '**Goal:** Inspect artifacts' '**Actions:**' '1. Inspect' '**Output:** Review notes' '**Checkpoint:** User approval required' '**Checkpoint class:** artifact_review' > "$1/WORKFLOW.md"; printf '%s\n' 'hooks:' '  definitions:' '    approval_judge:' '      command: fixture-judge-not-executed' '      enabled: true' >> "$3/fest.yaml"`, "sh", judgePhase, phase, root})
	require.NoError(t, err)
	phase = judgePhase
	out, err = run("workflow", "status", "--json")
	require.NoError(t, err, out)
	require.Len(t, calls(), beforeJudge)
	// An explicitly human-required checkpoint still notifies despite that judge.
	_, err = tc.runCommand([]string{"sh", "-c", `printf '%s\n' '**Approval:** human-required' >> "$1/WORKFLOW.md"`, "sh", phase})
	require.NoError(t, err)
	out, err = run("workflow", "reset", "--force")
	require.NoError(t, err, out)
	entries = calls()
	require.Len(t, entries, beforeJudge+1)
	require.Equal(t, "workflow_approval_needed", entries[len(entries)-1]["verb"])
	require.Equal(t, float64(1), entries[len(entries)-1]["step"])
	out, err = run("workflow", "status", "--json")
	require.NoError(t, err, out)
	require.Len(t, calls(), beforeJudge+1)
}
