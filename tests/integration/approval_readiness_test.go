//go:build integration
// +build integration

package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Filesystem containment must be exercised inside the container harness, not
// with host t.TempDir workflows.
func TestApprovalReadinessRejectsEvidenceSymlinkOutsidePhase(t *testing.T) {
	container := GetSharedContainer(t)
	setupWorkspace(t, container, "/")

	festivalPath := "/festivals/active/approval-readiness-test"
	phasePath := festivalPath + "/001_INGEST"
	script := `
set -eu
mkdir -p "` + phasePath + `/output_specs" /outside
# Configure the approval judge via operator-owned .festival/config.yaml. Passing
# --judge-command is TTY-gated (PR #292) and unusable in this non-interactive
# container, so the operator-configured hook is the path that reaches readiness.
cat > /festivals/.festival/config.yaml <<'EOF'
version: "1.0"
hooks:
  approval_judge:
    command: /bin/false
EOF
cat > "` + festivalPath + `/fest.yaml" <<'EOF'
version: "1.0"
name: approval-readiness-test
id: AR-TEST-001
metadata:
  id: AR-TEST-001
  status_history:
    - status: active
      timestamp: 2026-07-13T00:00:00Z
EOF
cat > "` + phasePath + `/PHASE_GOAL.md" <<'EOF'
---
fest_type: phase_goal
fest_id: 001_INGEST
fest_mode: ingest
fest_phase_type: ingest
---

# Phase Goal
Verify readiness containment.
EOF
cat > "` + phasePath + `/WORKFLOW.md" <<'EOF'
---
fest_type: workflow
fest_id: AR-TEST-001
fest_parent: 001_INGEST
---

# Approval Readiness Test

## Step 1: PRESENT — Review evidence

**Goal:** Review the presentation artifact.

**Actions:**
1. Inspect the evidence

**Output:** Summary presented to user

**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Wait for user response
EOF
printf '# unrelated outside file\n' > /outside/presentation.md
ln -s /outside/presentation.md "` + phasePath + `/output_specs/PRESENTATION.md"
`
	_, err := container.runCommand([]string{"sh", "-c", script})
	require.NoError(t, err)

	output, err := container.RunFestInDir(
		phasePath,
		"workflow", "approve", "--auto",
	)
	require.Error(t, err, "out-of-phase evidence symlink must fail readiness")
	require.Contains(t, strings.ToLower(output), "escapes the phase directory")
	require.NotContains(t, output, "Judge launched", "readiness must fail before detached judge launch")
}
