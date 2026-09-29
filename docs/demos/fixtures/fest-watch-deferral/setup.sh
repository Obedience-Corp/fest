#!/usr/bin/env bash
# Build the deterministic festival the fest watch deferral evidence records.
#
# Usage: setup.sh DEMO_ROOT [blocked|deferred]
#
# The shape is the one 01_containerised_acceptance_test.md uses: three tasks in
# one implementation sequence, 02 blocked. In the deferred state a
# blocker_deferred event is appended, which is how the deferral is produced
# here: fest task defer refuses an agent session by design and there is no flag
# to bypass it, so the event the verb would write is written directly and the
# refusal itself is recorded alongside as evidence that the guard held.
#
# Every timestamp is fixed so both recordings differ only in what the change
# under test changes.
set -euo pipefail

demo_root=${1:?usage: setup.sh DEMO_ROOT [blocked|deferred]}
state=${2:-blocked}
case "$demo_root" in
    "" | / | "$HOME")
        echo "refusing unsafe demo root: $demo_root" >&2
        exit 1
        ;;
esac

workspace="$demo_root/workspace"
festivals="$workspace/festivals"
fest_dir="$festivals/active/deferral-evidence-DE0001"
seq_dir="$fest_dir/001_IMPLEMENTATION/01_core_work"

rm -rf "$workspace" "$demo_root/config"
mkdir -p "$demo_root/config" "$workspace/.campaign" \
    "$festivals/.festival/.state" "$festivals/planning" "$seq_dir" "$fest_dir/.fest"

printf '%s\n' '{"workspace":"vhs-demo","registered":"2026-07-28T00:00:00Z"}' \
    >"$festivals/.festival/.state/.workspace"

cat >"$workspace/.campaign/campaign.yaml" <<'EOF'
id: fest-watch-deferral-vhs
name: Fest Watch Deferral VHS
EOF

cat >"$fest_dir/fest.yaml" <<'EOF'
version: "1.0"
name: deferral-evidence
id: DE0001
metadata:
  id: DE0001
  name: deferral-evidence
  status_history:
    - status: active
      timestamp: 2026-09-01T00:00:00Z
auto_link:
  enabled: false
EOF

cat >"$fest_dir/FESTIVAL_GOAL.md" <<'EOF'
# Deferral Evidence

## Goal

Prove a deferred blocker is drawn exactly like a blocked one.
EOF

cat >"$fest_dir/FESTIVAL_RULES.md" <<'EOF'
# Festival Rules

- A deferred blocker still reads as blocked.
EOF

cat >"$fest_dir/001_IMPLEMENTATION/PHASE_GOAL.md" <<'EOF'
---
fest_type: phase
fest_id: 001_IMPLEMENTATION
fest_phase_type: implementation
---

# Implementation

## Objective

Ship the pipeline.
EOF

cat >"$seq_dir/SEQUENCE_GOAL.md" <<'EOF'
---
fest_type: sequence
fest_id: 01_core_work
fest_parent: 001_IMPLEMENTATION
fest_order: 1
fest_tracking: true
---

# Core Work

Deliver the pipeline end to end.
EOF

write_task() {
    cat >"$seq_dir/$1.md" <<EOF
---
fest_type: task
fest_id: $1
fest_parent: 01_core_work
fest_tracking: true
---

# Task: $1

Work.
EOF
}
write_task 01_extract
write_task 02_transform
write_task 03_load

write_gate() {
    cat >"$seq_dir/$1.md" <<EOF
---
fest_type: gate
fest_id: $1.md
fest_name: $2
fest_parent: 01_core_work
fest_order: $3
fest_gate_id: $2
fest_gate_type: $2
fest_managed: true
fest_tracking: true
---

# Gate: $2
EOF
}
write_gate 04_testing testing 4
write_gate 05_review review 5
write_gate 06_iterate iterate 6
write_gate 07_fest_commit commit 7

events="$fest_dir/.fest/progress_events.jsonl"
task_key="001_IMPLEMENTATION/01_core_work/02_transform.md"
{
    printf '{"ts":"2026-09-20T09:00:00Z","event":"completed","task":"001_IMPLEMENTATION/01_core_work/01_extract.md"}\n'
    printf '{"ts":"2026-09-20T10:00:00Z","event":"blocked","task":"%s","reason":"the required API no longer exists","attempts":["checked the vendor changelog"]}\n' "$task_key"
} >"$events"

if [[ "$state" == "deferred" ]]; then
    printf '{"ts":"2026-09-21T11:00:00Z","event":"blocker_deferred","task":"%s","reason":"the required API no longer exists","attempts":["checked the vendor changelog"],"deferral_reason":"API removed upstream; revisit after the rest lands","deferred_by":"Ada Lovelace","actor":"operator","tty":true}\n' "$task_key" >>"$events"
fi

echo "$fest_dir"
