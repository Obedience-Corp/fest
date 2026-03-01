//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type manifestPayload struct {
	Commands []manifestCommand `json:"commands"`
}

type manifestCommand struct {
	Path         string `json:"path"`
	AgentAllowed bool   `json:"agent_allowed"`
	Reason       string `json:"reason"`
	Interactive  bool   `json:"interactive"`
}

func TestWorkflowSkip_ManifestAndAgentContextDenied(t *testing.T) {
	container := GetSharedContainer(t)

	output, err := container.RunFest("__manifest")
	require.NoError(t, err)

	var manifest manifestPayload
	require.NoError(t, json.Unmarshal([]byte(output), &manifest))

	entry, found := findManifestEntry(manifest.Commands, "workflow skip")
	require.True(t, found, "workflow skip should be present in manifest")
	require.False(t, entry.AgentAllowed, "workflow skip must be denied to agents")
	require.True(t, entry.Interactive, "workflow skip must be marked interactive")
	require.NotEmpty(t, entry.Reason, "workflow skip denial reason should be present")

	_, phasePath := setupWorkflowSkipFestival(t, container)
	denyOutput, denyErr := container.RunFestInDir(phasePath, "workflow", "skip", "--reason", "already_done_externally")
	require.Error(t, denyErr, "non-TTY execution should be denied")
	require.Contains(t, strings.ToLower(denyOutput), "interactive tty")
}

func TestWorkflowSkip_HumanTTYExecution(t *testing.T) {
	container := GetSharedContainer(t)

	_, phasePath := setupWorkflowSkipFestival(t, container)

	skipOutput, err := container.RunFestInDirTTY(phasePath, "workflow", "skip", "--reason", "already_done_externally", "--as", "skipped")
	require.NoError(t, err, "TTY human path should succeed")
	require.Contains(t, skipOutput, "Applied operator override")

	statusOutput, err := container.RunFestInDir(phasePath, "workflow", "status")
	require.NoError(t, err)
	require.Contains(t, statusOutput, "⤼")
	require.Contains(t, strings.ToLower(statusOutput), "note:")
}

func findManifestEntry(commands []manifestCommand, path string) (manifestCommand, bool) {
	for _, command := range commands {
		if command.Path == path {
			return command, true
		}
	}
	return manifestCommand{}, false
}

func setupWorkflowSkipFestival(t *testing.T, container *TestContainer) (string, string) {
	t.Helper()

	festPath := "/festivals/workflow-skip-test"
	phasePath := festPath + "/001_INGEST"
	script := `
set -eu
mkdir -p /festivals/.festival
mkdir -p "` + phasePath + `"
cat > "` + festPath + `/fest.yaml" <<'EOF'
name: workflow-skip-test
id: WF-SKIP-001
EOF
cat > "` + phasePath + `/PHASE_GOAL.md" <<'EOF'
---
fest_type: phase_goal
fest_id: 001_INGEST
fest_mode: ingest
fest_phase_type: ingest
---

# Phase Goal
Test workflow skip integration path.
EOF
cat > "` + phasePath + `/WORKFLOW.md" <<'EOF'
---
fest_type: workflow
fest_id: WF-SKIP-001
fest_parent: 001_INGEST
---

# Workflow Skip Test

## Step 1: REVIEW — Review prior work

**Goal:** Review externally completed work.

**Actions:**
1. Confirm artifacts exist
2. Capture summary

**Output:** Summary notes

**Checkpoint:** None

---

## Step 2: FINALIZE — Close phase

**Goal:** Close out phase records.

**Actions:**
1. Update status tracking
2. Confirm completion

**Output:** Completion note

**Checkpoint:** None
EOF
`
	_, err := container.runCommand([]string{"sh", "-c", script})
	require.NoError(t, err)

	return festPath, phasePath
}
