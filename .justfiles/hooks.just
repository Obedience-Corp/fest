# Git hook recipes

# Show available hook commands
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Point this checkout's git hooks at .githooks (run once per clone or worktree)
[no-cd]
install:
    #!/usr/bin/env bash
    set -euo pipefail
    git config core.hooksPath .githooks
    echo "core.hooksPath set to .githooks"
    echo "pre-push now runs: just check"
    echo "Bypass a single push with: git push --no-verify"

# Restore git's default hook path for this checkout
[no-cd]
uninstall:
    #!/usr/bin/env bash
    set -euo pipefail
    git config --unset core.hooksPath || true
    echo "core.hooksPath unset; git hooks are back to .git/hooks"

# Report whether this checkout is using .githooks
[no-cd]
status:
    #!/usr/bin/env bash
    set -euo pipefail
    path="$(git config --get core.hooksPath || true)"
    if [ -z "$path" ]; then
        echo "core.hooksPath is unset. Run: just hooks install"
    else
        echo "core.hooksPath = $path"
    fi
