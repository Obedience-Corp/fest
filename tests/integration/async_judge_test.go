//go:build integration
// +build integration

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Async approval-judge coverage that exercises real process launch, re-exec of
// the fest binary as `workflow judge-exec`, and durable waiting-on-judge state
// MUST run in this container harness — never as host `go test` unit tests.
//
// The host process-storm that froze operator machines was caused by unit tests
// re-execing `workflow.test` as judge-exec under go test. Container isolation
// keeps any runaway children off the host.

const (
	asyncJudgeFestival = "/festivals/async-judge-test"
	asyncJudgePhase    = "/festivals/async-judge-test/001_INGEST"
	asyncJudgeBinDir   = "/opt/async-judge"
)

func TestAsyncJudge_ApproveAutoFireAndForgetCompletes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	container := GetSharedContainer(t)
	setupAsyncJudgeFestival(t, container, "instant-approve")

	// Fire-and-forget: parent returns while detached judge-exec runs.
	out, err := container.RunFestInDir(asyncJudgePhase, "workflow", "approve", "--auto")
	require.NoError(t, err, "approve --auto should launch: %s", out)
	require.Contains(t, out, "Judge launched", "expected async launch notice, got: %s", out)
	// Parent must not apply the verdict (only the detached runner does).
	require.NotContains(t, out, "Reason:",
		"async --auto must not print in-process verdict reason in the parent: %s", out)

	// Detached runner should finish quickly with the instant-approve judge.
	waitForJudgeSettled(t, container, 15*time.Second)

	jsonOut, err := container.RunFestInDir(asyncJudgePhase, "workflow", "status", "--json")
	require.NoError(t, err, "status after judge: %s", jsonOut)
	require.NotContains(t, jsonOut, `"waiting_on_judge": true`,
		"judge should have finished; still waiting: %s", jsonOut)
	require.Contains(t, jsonOut, `"judge_status": "approved"`,
		"expected approved judge outcome: %s", jsonOut)

	// The durable lease and PID claim must precede the verdict even when the
	// judge answers immediately. This is the fast-child interleaving contract.
	events, err := container.ReadFile(asyncJudgeFestival + "/.fest/progress_events.jsonl")
	require.NoError(t, err)
	startedAt := strings.Index(events, `"event":"wf_judge_started"`)
	claimedAt := strings.Index(events, `"event":"wf_judge_claimed"`)
	returnedAt := strings.Index(events, `"event":"wf_judge_returned"`)
	require.GreaterOrEqual(t, startedAt, 0, "missing judge start event: %s", events)
	require.Greater(t, claimedAt, startedAt, "PID claim must follow durable lease: %s", events)
	require.Greater(t, returnedAt, claimedAt, "verdict must follow PID claim: %s", events)

	// No leftover judge-exec children from the detached path.
	assertNoOrphanJudgeExec(t, container)
}

func TestAsyncJudge_ConcurrentLaunchCreatesSingleRunner(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	container := GetSharedContainer(t)
	setupAsyncJudgeFestival(t, container, "slow-approve")

	out, err := container.Exec("sh", "-c", `
cd `+asyncJudgePhase+`
/fest workflow approve --auto >/tmp/judge-a.out 2>&1 & a=$!
/fest workflow approve --auto >/tmp/judge-b.out 2>&1 & b=$!
wait "$a"; sa=$?
wait "$b"; sb=$?
printf 'status-a=%s\nstatus-b=%s\n' "$sa" "$sb"
cat /tmp/judge-a.out /tmp/judge-b.out
`)
	require.NoError(t, err, out)
	require.True(t,
		strings.Contains(out, "status-a=0\nstatus-b=1") || strings.Contains(out, "status-a=1\nstatus-b=0"),
		"exactly one concurrent launch must succeed: %s", out)
	require.Equal(t, 1, strings.Count(out, "Judge launched"), "only one detached runner may launch: %s", out)
	require.Contains(t, strings.ToLower(out), "already evaluating", out)

	waitForJudgeSettled(t, container, 20*time.Second)
	assertNoOrphanJudgeExec(t, container)
}

func TestAsyncJudge_ManualRejectSupersedesRunningVerdict(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	container := GetSharedContainer(t)
	setupAsyncJudgeFestival(t, container, "slow-approve")

	out, err := container.RunFestInDir(asyncJudgePhase, "workflow", "approve", "--auto")
	require.NoError(t, err, out)
	require.Contains(t, out, "Judge launched")

	rejectOut, err := container.RunFestInDir(asyncJudgePhase, "workflow", "reject", "--reason", "operator-override")
	require.NoError(t, err, rejectOut)
	require.Contains(t, rejectOut, "operator-override")

	waitForJudgeSettled(t, container, 20*time.Second)
	jsonOut, err := container.RunFestInDir(asyncJudgePhase, "workflow", "status", "--json")
	require.NoError(t, err, jsonOut)
	require.Contains(t, jsonOut, `"status": "blocked"`, "manual rejection must remain authoritative: %s", jsonOut)
	require.Contains(t, jsonOut, `"feedback": "operator-override"`, jsonOut)
	require.Contains(t, jsonOut, `"judge_status": "canceled"`, jsonOut)
	require.NotContains(t, jsonOut, `"judge_status": "approved"`, "stale approval overwrote operator decision: %s", jsonOut)
	assertNoOrphanJudgeExec(t, container)
}

func TestAsyncJudge_WaitingOnJudgeSurfacedWhileRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	container := GetSharedContainer(t)
	setupAsyncJudgeFestival(t, container, "slow-approve")

	out, err := container.RunFestInDir(asyncJudgePhase, "workflow", "approve", "--auto")
	require.NoError(t, err, "approve --auto should launch: %s", out)
	require.Contains(t, out, "Judge launched", out)

	// While the slow judge sleeps, status/show/json must report waiting-on-judge.
	status, err := container.RunFestInDir(asyncJudgePhase, "workflow", "status")
	require.NoError(t, err, status)
	require.Contains(t, strings.ToLower(status), "waiting on judge",
		"text status should show purple waiting-on-judge while runner alive: %s", status)

	show, err := container.RunFestInDir(asyncJudgePhase, "workflow", "show")
	require.NoError(t, err, show)
	require.Contains(t, strings.ToLower(show), "waiting",
		"show should surface judge waiting state: %s", show)

	jsonOut, err := container.RunFestInDir(asyncJudgePhase, "workflow", "status", "--json")
	require.NoError(t, err, jsonOut)
	require.Contains(t, jsonOut, `"waiting_on_judge": true`,
		"json status should set waiting_on_judge: %s", jsonOut)
	require.Contains(t, jsonOut, `"judge_status": "running"`, jsonOut)

	// fest workflow approve --auto must refuse a duplicate launch while alive.
	// (fest next may still hit festival validation before auto-delegate; the
	// approve path is the direct contract for duplicate refusal vs soft wait.)
	dupOut, dupErr := container.RunFestInDir(asyncJudgePhase, "workflow", "approve", "--auto")
	require.Error(t, dupErr, "duplicate approve --auto should fail while judge is alive: %s", dupOut)
	require.Contains(t, strings.ToLower(dupOut+errString(dupErr)), "already evaluating",
		"expected duplicate-launch refusal: %s / %v", dupOut, dupErr)

	// Soft waiting report is emitted by AutoDelegate on fest next when the
	// festival is valid; re-check after ensuring docs exist (setup already does).
	nextOut, nextErr := container.RunFestInDir(asyncJudgePhase, "next")
	combinedNext := strings.ToLower(nextOut + errString(nextErr))
	require.True(t,
		strings.Contains(combinedNext, "still running") ||
			strings.Contains(combinedNext, "waiting") ||
			strings.Contains(nextOut, "⚖") ||
			strings.Contains(combinedNext, "already evaluating"),
		"fest next should acknowledge an in-flight judge: %s / %v", nextOut, nextErr)

	waitForJudgeSettled(t, container, 20*time.Second)
	assertNoOrphanJudgeExec(t, container)
}

func TestAsyncJudge_WaitFlagBlocksUntilVerdict(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	container := GetSharedContainer(t)
	setupAsyncJudgeFestival(t, container, "instant-approve")

	out, err := container.RunFestInDir(asyncJudgePhase, "workflow", "approve", "--auto", "--wait")
	require.NoError(t, err, "approve --auto --wait: %s", out)
	// Blocking path applies the verdict in-process.
	require.True(t,
		strings.Contains(strings.ToLower(out), "approved") ||
			strings.Contains(strings.ToLower(out), "reason"),
		"wait path should report the verdict: %s", out)
	require.NotContains(t, out, "Judge launched",
		"--wait must not take the detached launch path: %s", out)

	assertNoOrphanJudgeExec(t, container)
}

func TestAsyncJudge_NextAutoDelegatesFireAndForget(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	container := GetSharedContainer(t)
	// Step 1 is the blocking checkpoint so fest next auto-delegates immediately.
	setupAsyncJudgeFestivalAtStep(t, container, "instant-approve", true)

	out, err := container.RunFestInDir(asyncJudgePhase, "next")
	// Auto-delegate launches and returns; checkpoint may still be open briefly.
	combined := out + errString(err)
	require.True(t,
		strings.Contains(combined, "Judge launched") ||
			strings.Contains(combined, "Checkpoint delegated") ||
			strings.Contains(strings.ToLower(combined), "approval judge"),
		"fest next should auto-delegate to the judge: %s", combined)

	waitForJudgeSettled(t, container, 15*time.Second)
	assertNoOrphanJudgeExec(t, container)
}

func TestAsyncJudge_NextLeavesOperatorAttestationForHuman(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	container := GetSharedContainer(t)
	setupAsyncJudgeFestivalAtStep(t, container, "instant-approve", true)

	_, err := container.Exec("sh", "-c", "sed -i 's/artifact_review/operator_attestation/' "+asyncJudgePhase+"/WORKFLOW.md")
	require.NoError(t, err)

	out, nextErr := container.RunFestInDir(asyncJudgePhase, "next")
	combined := out + errString(nextErr)
	require.NotContains(t, combined, "Judge launched", combined)
	require.Contains(t, strings.ToLower(combined), "operator", combined)
	assertNoOrphanJudgeExec(t, container)
}

// --- helpers ---

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func setupAsyncJudgeFestival(t *testing.T, container *TestContainer, judgeKind string) {
	t.Helper()
	setupAsyncJudgeFestivalAtStep(t, container, judgeKind, false)
}

// setupAsyncJudgeFestivalAtStep builds a minimal active festival under
// /festivals with workspace marker + approval_judge hook. When checkpointFirst
// is false, step 1 is non-blocking and step 2 is the approval checkpoint
// (matches the unit-test fixture); the helper advances to step 2.
func setupAsyncJudgeFestivalAtStep(t *testing.T, container *TestContainer, judgeKind string, checkpointFirst bool) {
	t.Helper()

	// Kill any leftover judge-exec / judge children from a prior test in this
	// shared container before rebuilding fixtures.
	killAsyncJudgeProcesses(t, container)
	installAsyncJudgeScripts(t, container)

	judgeCmd := asyncJudgeBinDir + "/" + judgeKind
	// Advance to checkpoint step when it is step 2.
	checkpointStep1 := `**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Wait for user`
	checkpointStep2 := `**Checkpoint:** None`
	if !checkpointFirst {
		checkpointStep1 = `**Checkpoint:** None`
		checkpointStep2 = `**Checkpoint class:** artifact_review

**Checkpoint:** APPROVAL REQUIRED — Wait for user`
	}

	script := fmt.Sprintf(`
set -eu
mkdir -p /festivals/.festival/.state /festivals/active
mkdir -p "%[1]s"

# Workspace marker so festivalsRoot / hooks resolve from the phase path.
cat > /festivals/.festival/.state/.workspace <<'EOF'
{"workspace": "/", "registered": "2026-01-01T00:00:00Z"}
EOF

cat > /festivals/.festival/config.yaml <<EOF
version: "1.0"
hooks:
  approval_judge:
    command: %[2]s
EOF

cat > "%[1]s/fest.yaml" <<'EOF'
version: "1.0"
name: async-judge-test
id: AJ-001
metadata:
  id: AJ-001
  status_history:
    - status: active
      timestamp: 2026-02-10T00:00:00Z
EOF

cat > "%[1]s/FESTIVAL_OVERVIEW.md" <<'EOF'
# Overview
Async judge integration fixture festival.
EOF

cat > "%[1]s/FESTIVAL_RULES.md" <<'EOF'
# Rules
Integration-only fixture.
EOF

mkdir -p "%[3]s"
cat > "%[3]s/PHASE_GOAL.md" <<'EOF'
---
fest_type: phase_goal
fest_id: 001_INGEST
fest_mode: ingest
fest_phase_type: ingest
---

# Phase Goal
Async judge integration fixture.
EOF

cat > "%[3]s/WORKFLOW.md" <<EOF
---
fest_type: workflow
fest_id: AJ-WF
fest_parent: 001_INGEST
---

# Async Judge Workflow

## Step 1: PREP — Prepare

**Goal:** Prepare for the gate.

**Actions:**
1. Prep work

**Output:** Prep notes

%[4]s

---

## Step 2: GATE — Approval gate

**Goal:** Judge the checkpoint.

**Actions:**
1. Review evidence

**Output:** Decision

%[5]s

---

## Step 3: COMPLETE — Done

**Goal:** Finish.

**Actions:**
1. Wrap up

**Output:** Done

**Checkpoint:** None
EOF
`, asyncJudgeFestival, judgeCmd, asyncJudgePhase, checkpointStep1, checkpointStep2)

	_, err := container.runCommand([]string{"sh", "-c", script})
	require.NoError(t, err, "setup async judge festival")

	// Ensure workflow state exists and current step is the blocking checkpoint.
	// Initializing via status is enough to load/create state at step 1.
	_, _ = container.RunFestInDir(asyncJudgePhase, "workflow", "status")

	if !checkpointFirst {
		// Advance past the non-blocking prep step onto the approval checkpoint.
		out, err := container.RunFestInDir(asyncJudgePhase, "workflow", "advance")
		require.NoError(t, err, "advance to checkpoint: %s", out)
	}
}

func installAsyncJudgeScripts(t *testing.T, container *TestContainer) {
	t.Helper()
	// Reset wipes /tmp and /festivals but not /opt — reinstall scripts each test
	// for hermetic commands.
	script := `
set -eu
mkdir -p ` + asyncJudgeBinDir + `
cat > ` + asyncJudgeBinDir + `/instant-approve <<'EOF'
#!/bin/sh
# Drain stdin (judge request JSON) and approve immediately.
cat >/dev/null
printf '%s\n' '{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"container instant approve"}'
EOF
cat > ` + asyncJudgeBinDir + `/slow-approve <<'EOF'
#!/bin/sh
# Hold the checkpoint in waiting-on-judge long enough for status assertions.
cat >/dev/null
sleep 4
printf '%s\n' '{"schema_version":"fest.approval.judge/v1","decision":"approve","reason":"container slow approve"}'
EOF
chmod 755 ` + asyncJudgeBinDir + `/instant-approve ` + asyncJudgeBinDir + `/slow-approve
`
	_, err := container.runCommand([]string{"sh", "-c", script})
	require.NoError(t, err, "install judge scripts")
}

func waitForJudgeSettled(t *testing.T, container *TestContainer, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		// Prefer JSON for a stable signal.
		out, err := container.RunFestInDir(asyncJudgePhase, "workflow", "status", "--json")
		last = out + errString(err)
		if err == nil && !strings.Contains(out, `"waiting_on_judge": true`) {
			// A canceled run may still be winding down after an operator override;
			// do not return until its detached process has actually exited.
			canceled := strings.Contains(out, `"judge_status": "canceled"`)
			if strings.Contains(out, `"judge_status": "approved"`) ||
				strings.Contains(out, `"judge_status": "failed"`) ||
				strings.Contains(out, `"judge_status": "rejected"`) ||
				(!canceled && !strings.Contains(out, `"judge_status": "running"`)) {
				return
			}
		}
		// Also treat absence of running judge processes as settled after a grace period.
		ps, _ := container.Exec("sh", "-c", "ps w 2>/dev/null | grep -E 'judge-exec|slow-approve|instant-approve' | grep -v grep || true")
		if strings.TrimSpace(ps) == "" && time.Now().After(deadline.Add(-timeout+2*time.Second)) {
			// processes gone; re-check once more after brief settle
			time.Sleep(200 * time.Millisecond)
			out, err := container.RunFestInDir(asyncJudgePhase, "workflow", "status", "--json")
			last = out + errString(err)
			if err == nil && !strings.Contains(out, `"waiting_on_judge": true`) {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("judge did not settle within %s; last status: %s", timeout, last)
}

func assertNoOrphanJudgeExec(t *testing.T, container *TestContainer) {
	t.Helper()
	// Give the process table a moment to reap zombies.
	time.Sleep(200 * time.Millisecond)
	ps, err := container.Exec("sh", "-c", "ps w 2>/dev/null | grep -E '[j]udge-exec|[s]low-approve|[i]nstant-approve' || true")
	require.NoError(t, err)
	require.Empty(t, strings.TrimSpace(ps), "orphan judge processes left in container:\n%s", ps)
}

func killAsyncJudgeProcesses(t *testing.T, container *TestContainer) {
	t.Helper()
	// Best-effort cleanup of prior detached runners in the shared container.
	// Isolation boundary is the container; still keep each test hermetic.
	_, _ = container.Exec("sh", "-c", `
pkill -f 'workflow judge-exec' 2>/dev/null || true
pkill -f '/opt/async-judge/' 2>/dev/null || true
sleep 0.2
`)
}
