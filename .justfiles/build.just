# Build recipes — local, cross-platform, and profile variants
binary_name := "fest"
main_path := "cmd/fest/main.go"
bin_dir := "bin"
build_tags := env_var_or_default("BUILD_TAGS", "")
BUILDTOOL := "go run ./internal/buildutil"
version_pkg := "github.com/Obedience-Corp/fest/internal/version"
version := env_var_or_default("VERSION", `git describe --tags --exact-match HEAD 2>/dev/null || echo "dev"`)
commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-s -w -X " + version_pkg + ".Version=" + version + " -X " + version_pkg + ".Commit=" + commit + " -X " + version_pkg + ".BuildDate=" + build_date

# Show available build commands
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# --- Local builds ---

# Build with visual dashboard (default)
[no-cd]
dashboard:
    @{{BUILDTOOL}} build

# Build only (fast, no vet)
[no-cd]
quick:
    @{{BUILDTOOL}} build-only

# Quick development build (no optimization)
[no-cd]
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        go build -tags '{{build_tags}}' -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}} {{main_path}}
    else
        go build -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}} {{main_path}}
    fi

# --- Profile variants ---

# Build in stable profile (default command surface)
[no-cd]
stable:
    BUILD_TAGS='' just build dashboard

# Build in dev profile (includes dev-only commands)
[no-cd]
dev-profile:
    BUILD_TAGS=dev just build dashboard

# Quick build in stable profile
[no-cd]
quick-stable:
    BUILD_TAGS='' just build quick

# Quick build in dev profile
[no-cd]
quick-dev:
    BUILD_TAGS=dev just build quick

# Show the command surface for stable/dev build profiles
[no-cd]
profile-commands profile='all':
    @{{BUILDTOOL}} profile-commands {{profile}}

# --- Cross-platform builds ---

# Build for Linux AMD64
[no-cd]
linux:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=linux GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}}-linux-amd64 {{main_path}}
    else
        GOOS=linux GOARCH=amd64 go build -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}}-linux-amd64 {{main_path}}
    fi

# Build for macOS Intel
[no-cd]
darwin:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=darwin GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}}-darwin-amd64 {{main_path}}
    else
        GOOS=darwin GOARCH=amd64 go build -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}}-darwin-amd64 {{main_path}}
    fi

# Build for macOS Apple Silicon
[no-cd]
darwin-arm64:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=darwin GOARCH=arm64 go build -tags '{{build_tags}}' -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}}-darwin-arm64 {{main_path}}
    else
        GOOS=darwin GOARCH=arm64 go build -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}}-darwin-arm64 {{main_path}}
    fi

# Build for Windows AMD64
[no-cd]
windows:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=windows GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}}-windows-amd64.exe {{main_path}}
    else
        GOOS=windows GOARCH=amd64 go build -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}}-windows-amd64.exe {{main_path}}
    fi

# Build for all platforms
all: linux darwin darwin-arm64 windows
    @echo "All platform builds complete"

# Cross-platform builds in stable profile
[no-cd]
all-stable:
    BUILD_TAGS='' just build all

# Cross-platform builds in dev profile
[no-cd]
all-dev:
    BUILD_TAGS=dev just build all

# Build Linux binary for container testing
[no-cd]
test-binary:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=linux GOARCH=$(go env GOARCH) go build -tags '{{build_tags}}' -ldflags '{{ldflags}}' -o {{bin_dir}}/linux/{{binary_name}} {{main_path}}
    else
        GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags '{{ldflags}}' -o {{bin_dir}}/linux/{{binary_name}} {{main_path}}
    fi
