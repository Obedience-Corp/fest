#!/usr/bin/env bash
set -euo pipefail

demo_root=${1:?usage: setup.sh DEMO_ROOT}
case "$demo_root" in
    "" | / | "$HOME")
        echo "refusing unsafe demo root: $demo_root" >&2
        exit 1
        ;;
esac

workspace="$demo_root/workspace"
festivals="$workspace/festivals"

rm -rf "$workspace" "$demo_root/config"
mkdir -p \
    "$demo_root/config" \
    "$workspace/.campaign" \
    "$festivals/.festival/.state" \
    "$festivals/planning" \
    "$festivals/ritual" \
    "$festivals/dungeon/completed" \
    "$festivals/dungeon/archived" \
    "$festivals/dungeon/someday"

printf '%s\n' '{"workspace":"vhs-demo","registered":"2026-07-28T00:00:00Z"}' \
    >"$festivals/.festival/.state/.workspace"

write_festival() {
    local path=$1
    local id=$2
    local name=$3
    local status=$4
    local goal=$5

    mkdir -p "$path/001_IMPLEMENT/01_core"
    cat >"$path/fest.yaml" <<EOF
version: "1.0"
metadata:
  id: $id
  name: $name
  status_history:
    - status: $status
      timestamp: 2026-07-28T00:00:00Z
auto_link:
  enabled: false
EOF
    cat >"$path/FESTIVAL_GOAL.md" <<EOF
# $name

## Goal

$goal
EOF
    cat >"$path/FESTIVAL_RULES.md" <<'EOF'
# Festival Rules

- Keep the live status board calm and readable.
EOF
    cat >"$path/001_IMPLEMENT/PHASE_GOAL.md" <<'EOF'
---
fest_type: phase
fest_phase_type: implementation
---

# Implement

## Objective

Ship the focused lifecycle improvement.
EOF
    cat >"$path/001_IMPLEMENT/01_core/SEQUENCE_GOAL.md" <<'EOF'
# Core

Deliver the user-visible behavior.
EOF
    cat >"$path/001_IMPLEMENT/01_core/01_finish.md" <<'EOF'
# Finish

## Definition of Done

- [ ] The festival reaches its next lifecycle status.
EOF
}

write_festival \
    "$festivals/active/launch-readiness-LW0001" \
    "LW0001" \
    "launch-readiness" \
    "active" \
    "Show that the board refreshes only when lifecycle status changes."

write_festival \
    "$festivals/ready/docs-polish-LW0002" \
    "LW0002" \
    "docs-polish" \
    "ready" \
    "Keep the launch documentation ready for review."
