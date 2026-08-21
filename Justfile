#!/usr/bin/env just --justfile
# fest CLI build and development tasks

set dotenv-load := true

# Configuration
binary_name := "fest"
bin_dir := "bin"
gobin := env_var_or_default("GOBIN", `go env GOPATH` + "/bin")

# Version injection
version_pkg := "github.com/Obedience-Corp/fest/internal/version"
version := env_var_or_default("VERSION", `git describe --tags --always --dirty 2>/dev/null || echo "dev"`)
commit := `git rev-parse --short HEAD 2>/dev/null || echo "unknown"`
build_date := `date -u +"%Y-%m-%dT%H:%M:%SZ"`
ldflags := "-X " + version_pkg + ".Version=" + version + " -X " + version_pkg + ".Commit=" + commit + " -X " + version_pkg + ".BuildDate=" + build_date

# Modules
[doc('Build variants (local, cross-platform, profiles)')]
mod build '.justfiles/build.just'

[doc('Testing (unit, integration, coverage)')]
mod test '.justfiles/test.just'

[doc('Release packaging and versioning')]
mod release '.justfiles/release.just'

[doc('Install fest to $GOBIN (stable, dev, current)')]
mod install '.justfiles/install.just'

[doc('Linting (golangci-lint, gopls, vet)')]
mod lint '.justfiles/lint.just'

[doc('Record terminal workflows with VHS')]
mod vhs '.justfiles/vhs.just'

[private]
default:
    #!/usr/bin/env bash
    echo "fest CLI - Festival Methodology Tool"
    echo ""
    just --list --unsorted

# Format Go code (whole-module scope, incl. build-tagged integration files)
fmt:
    gofmt -w .

# Run the full release gate before creating any channel tag.
# Covers stable/dev command surfaces plus the containerized integration suite.
gate:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "=== gate: whitespace ==="
    git diff --check
    echo "=== gate: stable build ==="
    just build quick-stable
    echo "=== gate: dev build ==="
    just build quick-dev
    echo "=== gate: vet stable ==="
    just lint vet
    echo "=== gate: vet dev ==="
    go vet -tags=dev ./...
    echo "=== gate: vet integration ==="
    go vet -tags=integration ./...
    echo "=== gate: lint ==="
    just lint all
    echo "=== gate: docs ==="
    just docs-check
    echo "=== gate: stable tests ==="
    just test all
    echo "=== gate: dev unit tests ==="
    go test -short -tags=dev ./...
    echo "=== gate: PASSED ==="

# Clean build artifacts with visual dashboard
clean:
    @go run ./internal/buildutil clean

# Update and tidy dependencies
deps:
    go get -u ./...
    go mod tidy


# Generate CLI reference docs
docs:
    #!/usr/bin/env bash
    set -euo pipefail
    just build quick
    ./{{bin_dir}}/{{binary_name}} gendocs --output docs/cli-reference --format markdown --single

# Fail when the committed CLI reference no longer matches the code.
# Generates into a temp dir rather than the working tree, so running the gate
# never leaves modified files behind.
docs-check:
    #!/usr/bin/env bash
    set -euo pipefail
    just build quick
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT
    ./{{bin_dir}}/{{binary_name}} gendocs --output "$tmp" --format markdown --single
    if ! diff -ru docs/cli-reference "$tmp" >/dev/null 2>&1; then
        echo "FAIL: docs/cli-reference is stale. Run 'just docs' and commit the result." >&2
        diff -ru docs/cli-reference "$tmp" | head -40 >&2
        exit 1
    fi
    echo "docs/cli-reference is up to date"

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
