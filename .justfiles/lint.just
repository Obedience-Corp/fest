# Lint recipes

# Show available lint commands
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Run golangci-lint on all packages
[no-cd]
all:
    golangci-lint run ./...

# Run go vet on all packages
[no-cd]
vet:
    go vet ./...

# Run gopls diagnostics on a specific package (e.g., just lint gopls internal/commands/next)
[no-cd]
gopls pkg:
    #!/usr/bin/env bash
    set -euo pipefail
    files=$(find "{{pkg}}" -maxdepth 1 -name '*.go' -not -name '*_test.go' 2>/dev/null)
    if [ -z "$files" ]; then
        echo "No .go files found in {{pkg}}"
        exit 1
    fi
    echo "=== gopls check: {{pkg}} ==="
    echo "$files" | xargs gopls check

# Run gopls diagnostics on a specific package including test files
[no-cd]
gopls-tests pkg:
    #!/usr/bin/env bash
    set -euo pipefail
    files=$(find "{{pkg}}" -maxdepth 1 -name '*.go' 2>/dev/null)
    if [ -z "$files" ]; then
        echo "No .go files found in {{pkg}}"
        exit 1
    fi
    echo "=== gopls check (with tests): {{pkg}} ==="
    echo "$files" | xargs gopls check

# Run gopls diagnostics on ALL packages (full scan)
[no-cd]
gopls-all:
    #!/usr/bin/env bash
    set -euo pipefail
    failed=0
    for pkg_dir in $(go list -f '{{ '{{' }}.Dir{{ '}}' }}' ./...); do
        rel="${pkg_dir#$PWD/}"
        files=$(find "$pkg_dir" -maxdepth 1 -name '*.go' 2>/dev/null)
        [ -z "$files" ] && continue
        echo "=== gopls check: $rel ==="
        if ! echo "$files" | xargs gopls check; then
            failed=$((failed + 1))
        fi
    done
    if [ $failed -gt 0 ]; then
        echo ""
        echo "$failed package(s) had diagnostics"
        exit 1
    fi
    echo "All packages clean"

# List all packages (useful for picking a target for gopls)
[no-cd]
packages:
    @go list -f '{{ '{{' }}.ImportPath{{ '}}' }}' ./... | sed 's|github.com/Obedience-Corp/fest/||'

# Run gopls on packages matching a pattern (e.g., just lint grep-gopls commands)
[no-cd]
grep-gopls pattern:
    #!/usr/bin/env bash
    set -euo pipefail
    failed=0
    for pkg_dir in $(go list -f '{{ '{{' }}.Dir{{ '}}' }}' ./... | grep "{{pattern}}"); do
        rel="${pkg_dir#$PWD/}"
        files=$(find "$pkg_dir" -maxdepth 1 -name '*.go' 2>/dev/null)
        [ -z "$files" ] && continue
        echo "=== gopls check: $rel ==="
        if ! echo "$files" | xargs gopls check; then
            failed=$((failed + 1))
        fi
    done
    if [ $failed -gt 0 ]; then
        echo "$failed package(s) had diagnostics"
        exit 1
    fi
    echo "Matching packages clean"

# Install required linting tools
[no-cd]
install-tools:
    go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
