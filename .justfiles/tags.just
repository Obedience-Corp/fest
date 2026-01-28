# Git tag management recipes

# Show available tag commands (default)
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# List all tags (sorted by version)
[no-cd]
list:
    @git tag -l --sort=-version:refname

# Show tag details
[no-cd]
show version:
    git show "{{version}}" --quiet

# Create a git tag with message
[no-cd]
create version message="":
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -z "{{message}}" ]; then
        git tag -a "{{version}}" -m "{{version}}"
    else
        git tag -a "{{version}}" -m "{{message}}"
    fi
    echo "✅ Created tag {{version}}"

# Delete a tag locally
[no-cd]
delete version:
    git tag -d "{{version}}"
    @echo "✅ Deleted local tag {{version}}"

# Push tag to remote
[no-cd]
push version:
    git push origin "{{version}}"
    @echo "✅ Pushed tag {{version}} to origin"

# Push all tags to remote
[no-cd]
push-all:
    git push origin --tags
    @echo "✅ Pushed all tags to origin"

# Delete tag from remote
[no-cd]
delete-remote version:
    git push origin --delete "{{version}}"
    @echo "✅ Deleted tag {{version}} from origin"
