# Build recipes
binary_name := "fest"
main_path := "cmd/fest/main.go"
bin_dir := "bin"
build_tags := env_var_or_default("BUILD_TAGS", "")

# Show available build commands
[private]
default:
    @just --list --unsorted --justfile {{source_file()}}

# Quick development build (no optimization)
[no-cd]
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        go build -tags '{{build_tags}}' -o {{bin_dir}}/{{binary_name}} {{main_path}}
    else
        go build -o {{bin_dir}}/{{binary_name}} {{main_path}}
    fi

# Build for Linux AMD64
[no-cd]
linux:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=linux GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-linux-amd64 {{main_path}}
    else
        GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-linux-amd64 {{main_path}}
    fi

# Build for macOS Intel
[no-cd]
darwin:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=darwin GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-amd64 {{main_path}}
    else
        GOOS=darwin GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-amd64 {{main_path}}
    fi

# Build for macOS Apple Silicon
[no-cd]
darwin-arm64:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=darwin GOARCH=arm64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-arm64 {{main_path}}
    else
        GOOS=darwin GOARCH=arm64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-darwin-arm64 {{main_path}}
    fi

# Build for Windows AMD64
[no-cd]
windows:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=windows GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-windows-amd64.exe {{main_path}}
    else
        GOOS=windows GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/{{binary_name}}-windows-amd64.exe {{main_path}}
    fi

# Build for all platforms
all: linux darwin darwin-arm64 windows
    @echo "✅ All platform builds complete"

# Build Linux binary for container testing
[no-cd]
test-binary:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -n "{{build_tags}}" ]; then
        GOOS=linux GOARCH=amd64 go build -tags '{{build_tags}}' -ldflags '-s -w' -o {{bin_dir}}/linux/{{binary_name}} {{main_path}}
    else
        GOOS=linux GOARCH=amd64 go build -ldflags '-s -w' -o {{bin_dir}}/linux/{{binary_name}} {{main_path}}
    fi
