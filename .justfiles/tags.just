# Git tag management and release versioning recipes
# Two channels: dev (pre-release) and stable (public)

# Show available tag commands (default)
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# --- Release channels ---

# Create a dev pre-release tag (bumps dev counter, or pass patch|minor|major to bump base version)
[no-cd]
dev level="":
    #!/usr/bin/env bash
    set -euo pipefail

    level="{{level}}"
    latest=$(git tag -l "v*-dev.*" --sort=-version:refname | head -1)

    if [ -z "$latest" ]; then
        # No dev tags — migrate from alpha-stable or start fresh
        alpha=$(git tag -l "v*-alpha-stable" --sort=-version:refname | head -1)
        if [ -n "$alpha" ]; then
            nums="${alpha#v}"
            nums="${nums%-alpha-stable}"
            IFS='.' read -r major minor patch <<< "$nums"
            minor=$((minor + 1))
            patch=0
        else
            major=0; minor=1; patch=0
        fi
        new="v${major}.${minor}.${patch}-dev.1"
    else
        # Parse: v0.2.0-dev.3
        base="${latest%-dev.*}"
        dev_num="${latest##*-dev.}"
        nums="${base#v}"
        IFS='.' read -r major minor patch <<< "$nums"

        if [ -z "$level" ]; then
            # Just bump dev counter
            dev_num=$((dev_num + 1))
            new="v${major}.${minor}.${patch}-dev.${dev_num}"
        else
            # Bump base version, reset dev counter
            case "$level" in
                major) major=$((major + 1)); minor=0; patch=0 ;;
                minor) minor=$((minor + 1)); patch=0 ;;
                patch) patch=$((patch + 1)) ;;
                *) echo "Unknown level: $level (use patch, minor, or major)"; exit 1 ;;
            esac
            new="v${major}.${minor}.${patch}-dev.1"
        fi
    fi

    echo "Creating dev release: $new"
    git tag -a "$new" -m "$new"
    git push origin "$new"
    gh release create "$new" --title "$new" --generate-notes --prerelease
    echo "Released $new"

# Create a stable release tag (patch|minor|major)
[no-cd]
stable level="patch":
    #!/usr/bin/env bash
    set -euo pipefail

    level="{{level}}"

    # Find latest stable tag (no pre-release suffix — matches vX.Y.Z exactly)
    latest=$(git tag -l --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)

    if [ -z "$latest" ]; then
        # No stable tags — derive from latest dev tag base or start fresh
        dev=$(git tag -l "v*-dev.*" --sort=-version:refname | head -1)
        if [ -n "$dev" ]; then
            base="${dev%-dev.*}"
            new="$base"
        else
            new="v0.2.0"
        fi
    else
        nums="${latest#v}"
        IFS='.' read -r major minor patch <<< "$nums"

        case "$level" in
            major) major=$((major + 1)); minor=0; patch=0 ;;
            minor) minor=$((minor + 1)); patch=0 ;;
            patch) patch=$((patch + 1)) ;;
            *) echo "Unknown level: $level (use patch, minor, or major)"; exit 1 ;;
        esac
        new="v${major}.${minor}.${patch}"
    fi

    echo "Creating stable release: $new"
    git tag -a "$new" -m "$new"
    git push origin "$new"
    gh release create "$new" --title "$new" --generate-notes --latest
    echo "Released $new"

# Promote latest dev version to stable (strips -dev.N suffix)
[no-cd]
promote:
    #!/usr/bin/env bash
    set -euo pipefail

    latest_dev=$(git tag -l "v*-dev.*" --sort=-version:refname | head -1)
    if [ -z "$latest_dev" ]; then
        echo "No dev tags found. Create one first with: just tags dev"
        exit 1
    fi

    # Extract base version
    base="${latest_dev%-dev.*}"

    # Check if stable already exists
    if git tag -l "$base" | grep -q .; then
        echo "Stable tag $base already exists"
        exit 1
    fi

    echo "Promoting $latest_dev -> $base"
    git tag -a "$base" -m "Promote $latest_dev to stable $base"
    git push origin "$base"
    gh release create "$base" --title "$base" --generate-notes --latest
    echo "Released $base"

# Show current version for each channel
[no-cd]
current:
    #!/usr/bin/env bash
    dev=$(git tag -l "v*-dev.*" --sort=-version:refname | head -1)
    stable=$(git tag -l --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
    echo "dev:    ${dev:-none}"
    echo "stable: ${stable:-none}"

# --- Tag utilities ---

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
    echo "Created tag {{version}}"

# Delete a tag locally
[no-cd]
delete version:
    git tag -d "{{version}}"
    @echo "Deleted local tag {{version}}"

# Push tag to remote
[no-cd]
push version:
    git push origin "{{version}}"
    @echo "Pushed tag {{version}} to origin"

# Push all tags to remote
[no-cd]
push-all:
    git push origin --tags
    @echo "Pushed all tags to origin"

# Delete tag from remote
[no-cd]
delete-remote version:
    git push origin --delete "{{version}}"
    @echo "Deleted tag {{version}} from origin"
