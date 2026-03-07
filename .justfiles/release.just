# Release recipes
binary_name := "fest"
main_path := "cmd/fest/main.go"
bin_dir := "bin"
build_tags := env_var_or_default("BUILD_TAGS", "")

# Show available release commands (default)
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

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

# Bump version, tag, and push (delegates to tags dev)
[no-cd]
next level="":
    @just tags dev {{level}}
