# Install recipes — install fest with different profiles
binary_name := "fest"
bin_dir := "bin"
gobin := env_var_or_default("GOBIN", `go env GOPATH` + "/bin")

# Show available install commands
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Install stable profile (default command surface)
[no-cd]
stable:
    #!/usr/bin/env bash
    set -euo pipefail
    just build quick-stable
    just install _copy

# Install dev profile (includes dev-only commands)
[no-cd]
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    just build quick-dev
    just install _copy

# Install from current bin/ (whatever was last built)
[no-cd]
current:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ ! -f {{bin_dir}}/{{binary_name}} ]; then
        echo "Error: no binary found at {{bin_dir}}/{{binary_name}}"
        echo "Run 'just build quick' or 'just build quick-dev' first"
        exit 1
    fi
    just install _copy

# Copy binary to $GOBIN and sign on macOS
[no-cd, private]
_copy:
    #!/usr/bin/env bash
    set -euo pipefail
    echo "Installing fest..."
    mkdir -p {{gobin}}
    cp {{bin_dir}}/{{binary_name}} {{gobin}}/{{binary_name}}
    if [[ "$(uname)" == "Darwin" ]]; then
        echo "Signing fest binary for macOS..."
        codesign --force --sign - {{gobin}}/{{binary_name}} 2>/dev/null || \
        echo "Warning: Could not sign binary (non-fatal)"
    fi
    echo "fest installed to {{gobin}}/{{binary_name}}"
