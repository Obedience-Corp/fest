#!/usr/bin/env just --justfile
# fest CLI build and development tasks

set dotenv-load := true

# Configuration
binary_name := "fest"
bin_dir := "bin"
gobin := env_var_or_default("GOBIN", `go env GOPATH` + "/bin")
BUILDTOOL := "go run ./internal/buildutil"

# Version injection
version_pkg := "github.com/Obedience-Corp/fest/internal/version"
version := env_var_or_default("VERSION", "dev")
commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-X " + version_pkg + ".Version=" + version + " -X " + version_pkg + ".Commit=" + commit + " -X " + version_pkg + ".BuildDate=" + build_date

# Modules
[doc('Cross-platform builds')]
mod xbuild '.justfiles/build.just'

[doc('Testing (unit, integration, coverage)')]
mod test '.justfiles/test.just'

[doc('Release packaging')]
mod release '.justfiles/release.just'

[doc('Git tag management')]
mod tags '.justfiles/tags.just'


[private]
default:
    #!/usr/bin/env bash
    echo "fest CLI - Festival Methodology Tool"
    echo ""
    just --list --unsorted

# Build fest binary with visual dashboard
build:
    @{{BUILDTOOL}} build

# Build fest binary only (fast, no vet)
build-only:
    @echo "Building fest..."
    @mkdir -p {{bin_dir}}
    go build -ldflags '{{ldflags}}' -o {{bin_dir}}/{{binary_name}} ./cmd/fest
    @echo "Built {{bin_dir}}/{{binary_name}}"

# Format Go code
fmt:
    go fmt ./...

# Run go vet
vet:
    go vet ./...

# Run formatting and vetting
lint: fmt vet
    @echo "✅ Linting complete"

# Clean build artifacts with visual dashboard
clean:
    @{{BUILDTOOL}} clean

# Update and tidy dependencies
deps:
    go get -u ./...
    go mod tidy

# Install fest to $GOBIN
install:
    #!/usr/bin/env bash
    set -euo pipefail
    just build-only
    echo "Installing fest..."
    mkdir -p {{gobin}}
    cp bin/{{binary_name}} {{gobin}}/{{binary_name}}
    if [[ "$(uname)" == "Darwin" ]]; then
        echo "Signing fest binary for macOS..."
        codesign --force --sign - {{gobin}}/{{binary_name}} 2>/dev/null || \
        echo "Warning: Could not sign binary (non-fatal)"
    fi
    echo "✅ fest installed to {{gobin}}/{{binary_name}}"

# Generate CLI reference docs
docs: build-only
    ./{{bin_dir}}/{{binary_name}} gendocs --output docs/cli-reference --format markdown --single

# Uninstall fest from $GOBIN
uninstall:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Uninstalling fest..."
    if [ -f {{gobin}}/{{binary_name}} ]; then
        rm {{gobin}}/{{binary_name}}
        echo "fest uninstalled from {{gobin}}"
    else
        echo "fest not found in {{gobin}}"
    fi


