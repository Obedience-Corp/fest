# VHS recording helpers for Fest demos and integration fixtures.
#
# Record a tape with: just vhs record path/to/demo.tape
# Create a PR-ready Gist with: just vhs gist out/demo.gif

termcast := env_var_or_default("TERMCAST_BIN", "termcast")

[private]
default:
    @just --list --justfile {{source_file()}}

# Verify the binaries required by VHS are available.
doctor:
    #!/usr/bin/env bash
    set -euo pipefail
    for tool in vhs ttyd ffmpeg; do
        command -v "$tool" >/dev/null 2>&1 || {
            echo "vhs: missing required tool: $tool" >&2
            exit 1
        }
    done
    vhs --version

# Record one VHS tape. The tape owns its output path and setup commands.
record tape:
    vhs "{{tape}}"

# Create a GitHub Gist and print the PR-ready Markdown embed.
# Additional arguments are passed to termcast gist (for example, --public).
gist gif *args:
    {{termcast}} gist "{{gif}}" {{args}}
