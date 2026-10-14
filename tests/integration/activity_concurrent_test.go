//go:build integration
// +build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const activityConcurrentFestival = "/festivals/activity-concurrent-test"

// TestActivityConcurrentFestProcesses verifies multiple real fest processes
// append complete validate.ran JSONL lines to one festival activity log.
func TestActivityConcurrentFestProcesses(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	setupActivityConcurrentFestival(t, container)

	const runs = 8
	script := fmt.Sprintf(`
set -eu
fest=%q
for i in $(seq 1 %d); do
  /fest validate "$fest" --json >/tmp/act-val-$i.out 2>&1 &
done
wait
`, activityConcurrentFestival, runs)

	out, err := container.Exec("sh", "-c", script)
	require.NoError(t, err, out)

	activityPath := activityConcurrentFestival + "/.fest/activity.jsonl"
	raw, err := container.ReadFile(activityPath)
	require.NoError(t, err, "activity log must exist after concurrent validates")

	lines := nonEmptyLines(raw)
	require.GreaterOrEqual(t, len(lines), runs, "expected at least %d activity lines, got %d", runs, len(lines))

	validateCount := 0
	for _, line := range lines {
		var ev map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &ev), "line must be parseable JSON: %s", line)
		if ev["event"] == "validate.ran" {
			validateCount++
			result, _ := ev["result"].(map[string]any)
			require.NotNil(t, result, "validate.ran must include result")
		}
	}
	require.Equal(t, runs, validateCount, "each fest process must emit validate.ran once")
}

// TestActivityConcurrentFestProcessesAfterRotation fills the canonical log to a
// lowered rotation threshold, then runs concurrent fest processes so rotation
// happens during real validate writes rather than via a pre-built archive.
func TestActivityConcurrentFestProcessesAfterRotation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	setupActivityConcurrentFestival(t, container)

	const threshold = 8192
	const runs = 4
	script := fmt.Sprintf(`
set -eu
export FEST_ACTIVITY_ROTATION_THRESHOLD=%d
fest=%q
act="$fest/.fest/activity.jsonl"
n=0
: > "$act"
while [ "$(wc -c < "$act")" -lt "$FEST_ACTIVITY_ROTATION_THRESHOLD" ]; do
  printf '{"v":1,"event":"pad","result":{"ok":true},"n":%%s}\n' "$n" >> "$act"
  n=$((n+1))
done
for i in $(seq 1 %d); do
  /fest validate "$fest" --json >/tmp/act-rot-$i.out 2>&1 &
done
wait
`, threshold, activityConcurrentFestival, runs)

	out, err := container.Exec("sh", "-c", script)
	require.NoError(t, err, out)

	activityPath := activityConcurrentFestival + "/.fest/activity.jsonl"
	raw, err := container.ReadFile(activityPath)
	require.NoError(t, err)
	lines := nonEmptyLines(raw)
	require.Equal(t, runs, len(lines), "canonical log should contain only post-rotate validate events")

	rotatedPath := activityConcurrentFestival + "/.fest/activity.1.jsonl"
	rotated, err := container.ReadFile(rotatedPath)
	require.NoError(t, err, "rotated archive must exist after concurrent validates crossed the threshold")
	require.Contains(t, rotated, `"event":"pad"`, "pre-rotate padding must live in archive")
	require.NotContains(t, rotated, `"event":"validate.ran"`, "post-rotate writes must not land in rotated archive")

	for _, line := range lines {
		var ev map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &ev))
		require.Equal(t, "validate.ran", ev["event"])
	}
	for _, line := range nonEmptyLines(rotated) {
		var ev map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &ev), "archive line must be valid JSON")
	}
}

// TestActivityConcurrentLockContention verifies a fest process waits for an
// external lock holder and still writes a complete JSON line afterward.
func TestActivityConcurrentLockContention(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	container := GetSharedContainer(t)
	setupActivityConcurrentFestival(t, container)

	activityPath := activityConcurrentFestival + "/.fest/activity.jsonl"
	script := fmt.Sprintf(`
set -eu
act=%q
fest=%q
mkdir -p "$(dirname "$act")"
: >> "$act"
(
  flock -x 200
  sleep 2
) 200>"$act" &
holder=$!
sleep 0.2
before=$(wc -l < "$act")
/fest validate "$fest" --json >/tmp/act-lock.out 2>&1
validate_exit=$?
after=$(wc -l < "$act")
if [ "$validate_exit" -ne 0 ]; then
  echo "validate failed with exit $validate_exit:" >&2
  cat /tmp/act-lock.out >&2
  exit "$validate_exit"
fi
if ! wait "$holder"; then
  echo "lock holder exited non-zero" >&2
  exit 1
fi
printf 'before=%%s after=%%s\n' "$before" "$after"
`, activityPath, activityConcurrentFestival)

	out, err := container.Exec("sh", "-c", script)
	if err != nil {
		lockOut, _ := container.ReadFile("/tmp/act-lock.out")
		t.Fatalf("lock contention script failed: %v\nscript out: %q\nvalidate out: %q", err, out, lockOut)
	}
	require.Contains(t, out, "after=", out)
	require.NotContains(t, out, "after=0", "fest must append activity while lock was held")

	lockOut, err := container.ReadFile("/tmp/act-lock.out")
	require.NoError(t, err)
	var validateResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(lockOut)), &validateResult))
	require.Equal(t, true, validateResult["valid"], "validate must pass after lock contention")

	raw, err := container.ReadFile(activityPath)
	require.NoError(t, err)
	for _, line := range nonEmptyLines(raw) {
		var ev map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &ev), "partial/interleaved line: %s", line)
	}
}

func ensureFlock(t *testing.T, container *TestContainer) {
	t.Helper()
	_, err := container.Exec("sh", "-c", "command -v flock >/dev/null 2>&1 || apk add --no-cache util-linux >/dev/null")
	require.NoError(t, err, "flock must be available for lock contention proof")
}

func setupActivityConcurrentFestival(t *testing.T, container *TestContainer) {
	t.Helper()
	ensureFlock(t, container)

	script := fmt.Sprintf(`
set -eu
mkdir -p /festivals/.festival/.state /festivals/active
cat > /festivals/.festival/.state/.workspace <<'EOF'
{"workspace": "/", "registered": "2026-01-01T00:00:00Z"}
EOF

fest=%q
mkdir -p "$fest/.fest" "$fest/001_PHASE/01_seq"

cat > "$fest/fest.yaml" <<'EOF'
version: "1.0"
name: activity-concurrent-test
id: ACT-001
metadata:
  id: ACT-001
  status_history:
    - status: active
      timestamp: 2026-02-10T00:00:00Z
EOF

cat > "$fest/FESTIVAL_OVERVIEW.md" <<'EOF'
# Overview
Activity concurrent integration fixture.
EOF

cat > "$fest/FESTIVAL_RULES.md" <<'EOF'
# Rules
Integration-only fixture.
EOF

cat > "$fest/001_PHASE/PHASE_GOAL.md" <<'EOF'
---
fest_type: phase_goal
fest_id: 001_PHASE
fest_mode: implementation
fest_phase_type: implementation
---
# Phase
Fixture.
EOF

cat > "$fest/001_PHASE/01_seq/SEQUENCE_GOAL.md" <<'EOF'
# Sequence
Fixture.
EOF

cat > "$fest/001_PHASE/01_seq/01_task.md" <<'EOF'
# Task
Fixture.
EOF
`, activityConcurrentFestival)

	_, err := container.Exec("sh", "-c", script)
	require.NoError(t, err)

	require.NoError(t, ensureQualityGatesInFestivalConfig(container, activityConcurrentFestival))
	ensureGateTemplates(t, container, activityConcurrentFestival)
	_, err = container.RunFestInDir(activityConcurrentFestival, "gates", "apply", "--approve")
	require.NoError(t, err, "quality gates required for validate exit 0 in lock contention proof")
}

func nonEmptyLines(raw string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
