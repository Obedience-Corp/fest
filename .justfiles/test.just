# Test recipes
BUILDTOOL := "go run ./internal/buildutil"
version_pkg := "github.com/Obedience-Corp/fest/internal/version"
version := env_var_or_default("VERSION", `git describe --tags --always --dirty 2>/dev/null || echo "dev"`)
commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-s -w -X " + version_pkg + ".Version=" + version + " -X " + version_pkg + ".Commit=" + commit + " -X " + version_pkg + ".BuildDate=" + build_date

# Show available test commands
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Run all tests (unit + integration) with visual dashboard
[no-cd]
all:
    @{{BUILDTOOL}} all

# Run unit tests with visual dashboard
[no-cd]
unit:
    @{{BUILDTOOL}} test

# Run unit tests with verbose output
[no-cd]
unit-verbose:
    @{{BUILDTOOL}} -v test

# Run integration tests with visual dashboard
[no-cd]
integration:
    @{{BUILDTOOL}} integration

# Report whether this machine can run the integration suite: which Docker
# daemon the suite would use, whether it answers fast enough, what else is
# running on it, and whether another run holds the suite lock. Read-only.
[no-cd]
integration-doctor:
    @{{BUILDTOOL}} integration-doctor

# Create or boot the dedicated fest-itest Colima profile the suite runs on.
# The suite does this on demand too; run it by hand to pay the VM boot before
# a timed run, or to repair the profile after stopping it.
[no-cd]
daemon-start:
    @{{BUILDTOOL}} integration-doctor start

# Run integration tests with verbose output
[no-cd]
integration-verbose:
    @{{BUILDTOOL}} -v integration

# Run tests with coverage (using go test directly)
[no-cd]
coverage:
    go test -cover ./...

# Generate HTML coverage report
[no-cd]
coverage-html:
    go test -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html

# ============================================
# Fallback commands (bypass buildutil)
# ============================================

# Run unit tests directly (fallback)
[no-cd]
unit-raw:
    go test -short -timeout 30s ./...

# Run unit tests with verbose output (fallback)
[no-cd]
unit-raw-verbose:
    go test -v -short -timeout 30s ./...

# Run integration tests directly (fallback)
[no-cd]
integration-raw:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building Linux binary..."
    GOOS=linux GOARCH=$(go env GOARCH) go build -ldflags '{{ldflags}}' -o bin/linux/fest ./cmd/fest
    echo "Running integration tests..."
    TESTCONTAINERS_RYUK_DISABLED=true go test -v -tags=integration -timeout 5m ./tests/integration/...

# Run dev-profile fest watch integration tests through the container harness
[no-cd]
integration-dev-watch:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building dev Linux binary..."
    BUILD_TAGS=dev just build test-binary
    echo "Running fest watch dev integration tests..."
    TESTCONTAINERS_RYUK_DISABLED=true go test -v -tags "integration dev" -run TestFestWatch -timeout 5m ./tests/integration/...

# Run dev-profile fest list --watch integration tests through the container harness
[no-cd]
integration-dev-list-watch:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Building dev Linux binary..."
    BUILD_TAGS=dev just build test-binary
    echo "Running fest list --watch dev integration tests..."
    TESTCONTAINERS_RYUK_DISABLED=true go test -v -tags "integration dev" -run TestFestListWatch -timeout 5m ./tests/integration/...

# Run all tests directly (fallback)
[no-cd]
all-raw: unit-raw integration-raw
    @echo "✅ All tests passed (raw mode)"
