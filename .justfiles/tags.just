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

# Bump version, tag, and push (patch|minor|major)
[no-cd]
next level="patch":
    #!/usr/bin/env bash
    set -euo pipefail
    suffix="alpha-stable"
    level="{{level}}"

    latest=$(git tag -l "v*-${suffix}" --sort=-version:refname | head -1)

    if [ -z "$latest" ]; then
        new="v0.1.0-${suffix}"
    else
        nums="${latest#v}"
        nums="${nums%-${suffix}}"
        IFS='.' read -r major minor patch <<< "$nums"

        case "$level" in
            major) major=$((major + 1)); minor=0; patch=0 ;;
            minor) minor=$((minor + 1)); patch=0 ;;
            patch) patch=$((patch + 1)) ;;
            *) echo "Unknown level: $level (use patch, minor, or major)"; exit 1 ;;
        esac
        new="v${major}.${minor}.${patch}-${suffix}"
    fi

    git tag -a "$new" -m "$new"
    git push origin "$new"
    gh release create "$new" --title "$new" --generate-notes --prerelease
    echo "Released $new"
