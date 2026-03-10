# Release packaging and versioning recipes
# Three channels: dev (pre-release), rc (release candidate), and stable (public)
binary_name := "fest"
main_path := "cmd/fest/main.go"
bin_dir := "bin"
build_tags := env_var_or_default("BUILD_TAGS", "")

# Show available release commands (default)
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# --- Build & packaging ---

# Build for macOS Intel
[private, no-cd]
build-darwin:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=darwin GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-amd64 {{main_path}}
    else
        GOOS=darwin GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-amd64 {{main_path}}
    fi

# Build for macOS Apple Silicon
[private, no-cd]
build-darwin-arm64:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=darwin GOARCH=arm64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-arm64 {{main_path}}
    else
        GOOS=darwin GOARCH=arm64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-arm64 {{main_path}}
    fi

# Build for Linux AMD64
[private, no-cd]
build-linux:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=linux GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-linux-amd64 {{main_path}}
    else
        GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-linux-amd64 {{main_path}}
    fi

# Build for Windows AMD64
[private, no-cd]
build-windows:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=windows GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-windows-amd64.exe {{main_path}}
    else
        GOOS=windows GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-windows-amd64.exe {{main_path}}
    fi

# Sign macOS release binaries
[no-cd]
sign: build-darwin build-darwin-arm64
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Signing macOS release binaries..."
    if [ -f ./scripts/sign-macos.sh ]; then
        ./scripts/sign-macos.sh {{bin_dir}}/{{binary_name}}-darwin-amd64
        ./scripts/sign-macos.sh {{bin_dir}}/{{binary_name}}-darwin-arm64
    else
        echo "Warning: scripts/sign-macos.sh not found, using fallback signing"
        codesign --force --sign - {{bin_dir}}/{{binary_name}}-darwin-amd64 2>/dev/null || true
        codesign --force --sign - {{bin_dir}}/{{binary_name}}-darwin-arm64 2>/dev/null || true
    fi

# Create macOS release packages
[no-cd]
darwin: build-darwin sign
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Creating macOS release package..."
    mkdir -p dist
    tar -czf dist/{{binary_name}}-darwin-amd64.tar.gz -C {{bin_dir}} {{binary_name}}-darwin-amd64
    tar -czf dist/{{binary_name}}-darwin-arm64.tar.gz -C {{bin_dir}} {{binary_name}}-darwin-arm64
    echo "Release packages created in dist/"

# Create Linux release package
[no-cd]
linux: build-linux
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Creating Linux release package..."
    mkdir -p dist
    tar -czf dist/{{binary_name}}-linux-amd64.tar.gz -C {{bin_dir}} {{binary_name}}-linux-amd64

# Create Windows release package
[no-cd]
windows: build-windows
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Creating Windows release package..."
    mkdir -p dist
    cd {{bin_dir}} && zip -q ../dist/{{binary_name}}-windows-amd64.zip {{binary_name}}-windows-amd64.exe

# Create all release packages
[no-cd]
release: darwin linux windows
    #!/usr/bin/env bash
    set -euo pipefail
    echo "✅ All release packages created in dist/"
    ls -la dist/

# --- Versioning ---

# Bump version, tag, and push (aliases to dev channel bump)
[no-cd]
next level="":
    @just --justfile {{source_file()}} dev {{level}}

# Create a dev pre-release tag (bumps dev counter, or pass patch|minor|major to bump base version)
[no-cd]
dev level="":
    #!/usr/bin/env bash
    set -euo pipefail

    level="{{level}}"
    latest=$(git tag -l "v*-dev.*" --sort=-version:refname | head -1)

    if [ -z "$latest" ]; then
        major=0; minor=1; patch=0
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

# Create an RC release tag from the current branch
[no-cd]
rc version n="1":
    #!/usr/bin/env bash
    set -euo pipefail

    tag="v{{version}}-rc.{{n}}"

    if git tag -l "$tag" | grep -q .; then
        echo "RC tag $tag already exists locally"
        exit 1
    fi
    if [ -n "$(git ls-remote --tags origin "refs/tags/${tag}")" ]; then
        echo "RC tag $tag already exists on origin"
        exit 1
    fi

    echo "Creating rc release: $tag"
    git tag -a "$tag" -m "$tag"
    git push origin "$tag"
    gh release create "$tag" --title "$tag" --generate-notes --prerelease
    echo "Released $tag"

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
            new="v0.1.0"
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

# Promote the latest RC for a version to stable by stripping the -rc.N suffix
[no-cd]
promote-rc version="":
    #!/usr/bin/env bash
    set -euo pipefail

    if [ -n "{{version}}" ]; then
        latest_rc=$(git tag -l "v{{version}}-rc.*" --sort=-version:refname | head -1)
    else
        latest_rc=$(git tag -l "v*-rc.*" --sort=-version:refname | head -1)
    fi

    if [ -z "$latest_rc" ]; then
        echo "No rc tags found. Create one first with: just release rc <version> <n>"
        exit 1
    fi

    stable_tag="${latest_rc%-rc.*}"

    if git tag -l "$stable_tag" | grep -q .; then
        echo "Stable tag $stable_tag already exists"
        exit 1
    fi

    echo "Promoting $latest_rc -> $stable_tag"
    git tag -a "$stable_tag" -m "Promote $latest_rc to stable $stable_tag"
    git push origin "$stable_tag"
    gh release create "$stable_tag" --title "$stable_tag" --generate-notes --latest
    echo "Released $stable_tag"

# Promote latest dev version to stable (strips -dev.N suffix)
[no-cd]
promote:
    #!/usr/bin/env bash
    set -euo pipefail

    latest_dev=$(git tag -l "v*-dev.*" --sort=-version:refname | head -1)
    if [ -z "$latest_dev" ]; then
        echo "No dev tags found. Create one first with: just release dev"
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
    rc=$(git tag -l "v*-rc.*" --sort=-version:refname | head -1)
    stable=$(git tag -l --sort=-version:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -1)
    echo "dev:    ${dev:-none}"
    echo "rc:     ${rc:-none}"
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
