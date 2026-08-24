#!/usr/bin/env bash
set -euo pipefail

demo_root=${1:?usage: setup.sh DEMO_ROOT}
case "$demo_root" in
    "" | / | "$HOME")
        echo "refusing unsafe demo root: $demo_root" >&2
        exit 1
        ;;
esac

command -v fest >/dev/null 2>&1 || {
    echo "fest binary not on PATH" >&2
    exit 1
}

workspace="$demo_root/workspace"
rm -rf "$workspace" "$demo_root/config" "$demo_root/home"
mkdir -p "$demo_root/config" "$demo_root/home" "$workspace"

export HOME="$demo_root/home"
export FEST_CONFIG_DIR="$demo_root/config"

cd "$workspace"
git init -q
git config user.email "fest@demo"
git config user.name "fest"

fest create workflow demo --steps '{
  "title": "Leaveable demo",
  "description": "Two steps a caller-supplied worker can finish.",
  "steps": [
    {
      "name": "ALIGN",
      "goal": "Write the goal.",
      "actions": ["Name the outcome."],
      "checkpoint": "none"
    },
    {
      "name": "DO",
      "goal": "Do the work.",
      "actions": ["Make a note."],
      "checkpoint": "none"
    }
  ]
}' >/dev/null

cat >worker <<'EOF'
#!/bin/sh
cat >> notes.md
printf '\n' >> notes.md
exit 0
EOF
chmod +x worker

git add WORKFLOW.md worker .workflow
git commit -q -m "start demo"
