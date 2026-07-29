# VHS recording helpers for Fest demos and integration fixtures.
#
# Record a tape with: just vhs record path/to/demo.tape
# Build and validate its evidence with: just vhs bundle <evidence-dir> <binary> <tape> <fixture-id> <states...>
# Create a secret-Gist handoff with: just vhs gist out/demo.gif

set positional-arguments

[private]
default:
    @just --list --justfile {{source_file()}}

# Verify the binaries required by VHS are available.
doctor:
    #!/usr/bin/env bash
    set -euo pipefail
    for tool in vhs ttyd ffmpeg ffprobe python3 gh git curl fest; do
        command -v "$tool" >/dev/null 2>&1 || {
            echo "vhs: missing required tool: $tool" >&2
            exit 1
        }
    done
    vhs --version

# Record one VHS tape. The tape owns its output path and setup commands.
record tape:
    #!/usr/bin/env bash
    set -euo pipefail
    repo_root="$(git rev-parse --show-toplevel)"
    tape="{{tape}}"
    [[ "$tape" = /* ]] || tape="$repo_root/$tape"
    cd "$repo_root"
    exec vhs "$tape"

# Optimize, scan, and validate one complete evidence bundle. PTY metadata,
# pyte snapshots, dependencies.txt, and pty-transcript.txt must already exist
# in evidence_dir; raw GIFs stay in that evidence directory and are not uploaded.
bundle evidence_dir binary tape fixture_id +states:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -n "${VHS_PRIVATE_GIST_ROOT:-}" ]]; then
        skill_root="$VHS_PRIVATE_GIST_ROOT"
    else
        campaign_root="${CAMP_ROOT:-$(camp root 2>/dev/null || true)}"
        [[ -n "$campaign_root" ]] || {
            echo "vhs: campaign root not found; set CAMP_ROOT or VHS_PRIVATE_GIST_ROOT" >&2
            exit 1
        }
        skill_root="$campaign_root/.campaign/skills/vhs-private-gist"
    fi
    validator="$skill_root/scripts/validate-evidence.py"
    optimizer="$skill_root/scripts/optimize-gif.sh"
    metadata="$skill_root/scripts/collect-artifact-metadata.py"
    privacy="$skill_root/scripts/scan-privacy.py"
    for required in "$validator" "$optimizer" "$metadata" "$privacy"; do
        [[ -f "$required" ]] || { echo "vhs: missing shared helper: $required" >&2; exit 1; }
    done

    repo_root="$(git rev-parse --show-toplevel)"
    evidence_dir="{{evidence_dir}}"
    binary="{{binary}}"
    tape="{{tape}}"
    [[ "$evidence_dir" = /* ]] || evidence_dir="$repo_root/$evidence_dir"
    [[ "$binary" = /* ]] || binary="$repo_root/$binary"
    [[ "$tape" = /* ]] || tape="$repo_root/$tape"
    fixture_id="{{fixture_id}}"
    mkdir -p "$evidence_dir"
    "$optimizer" "$evidence_dir/raw.gif" "$evidence_dir/optimized.gif"
    python3 "$metadata" \
        --raw "$evidence_dir/raw.gif" \
        --optimized "$evidence_dir/optimized.gif" \
        --output "$evidence_dir/artifact-metadata.json"
    python3 "$privacy" \
        --repo-root "$repo_root" \
        --output "$evidence_dir/privacy-scan-pre.txt" \
        --path "$tape" \
        --path "$evidence_dir/raw.gif" \
        --path "$evidence_dir/optimized.gif" \
        --path "$evidence_dir/pty-transcript.txt" \
        --path "$evidence_dir/pty-metadata.json" \
        --path "$evidence_dir/screen-snapshots.json" \
        --path "$evidence_dir/artifact-metadata.json"

    expected=()
    for state in {{states}}; do
        expected+=(--expect-state "$state")
    done
    recorded_at=()
    if [[ -n "${VHS_RECORDED_AT:-}" ]]; then
        recorded_at=(--recorded-at "$VHS_RECORDED_AT")
    fi
    validate_bundle() {
        local privacy_scan=$1
        python3 "$validator" \
            --evidence-dir "$evidence_dir" \
            --repository fest \
            --journey "${VHS_JOURNEY:-$(basename "$tape" .tape)}" \
            --source-revision "$(git rev-parse HEAD)" \
            --fixture-id "$fixture_id" \
            --binary "$binary" \
            --tape "$tape" \
            --dependencies "$evidence_dir/dependencies.txt" \
            --privacy-scan "$privacy_scan" \
            "${expected[@]}" \
            "${recorded_at[@]}" \
            --validation-command "just vhs bundle <evidence-dir> <binary> <tape> <fixture-id>" \
            --repo-root "$repo_root"
    }
    validate_bundle "$evidence_dir/privacy-scan-pre.txt"
    python3 "$privacy" \
        --repo-root "$repo_root" \
        --output "$evidence_dir/privacy-scan.txt" \
        --path "$tape" \
        --path "$evidence_dir/raw.gif" \
        --path "$evidence_dir/optimized.gif" \
        --path "$evidence_dir/pty-transcript.txt" \
        --path "$evidence_dir/pty-metadata.json" \
        --path "$evidence_dir/screen-snapshots.json" \
        --path "$evidence_dir/artifact-metadata.json" \
        --path "$evidence_dir/manifest.json"
    validate_bundle "$evidence_dir/privacy-scan.txt"

# Create a secret/unlisted GitHub Gist and print the PR-ready Markdown embed.
# The shared helper rejects --public; publication never defaults to public.
gist gif *args:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ -n "${VHS_PRIVATE_GIST_ROOT:-}" ]]; then
        helper="$VHS_PRIVATE_GIST_ROOT/scripts/publish-secret-gist.sh"
    else
        campaign_root="${CAMP_ROOT:-$(camp root 2>/dev/null || true)}"
        [[ -n "$campaign_root" ]] || { echo "vhs: campaign root not found" >&2; exit 1; }
        helper="$campaign_root/.campaign/skills/vhs-private-gist/scripts/publish-secret-gist.sh"
    fi
    [[ -x "$helper" ]] || { echo "vhs: missing shared Gist helper: $helper" >&2; exit 1; }
    repo_root="$(git rev-parse --show-toplevel)"
    gif="{{gif}}"
    [[ "$gif" = /* ]] || gif="$repo_root/$gif"
    exec "$helper" "$gif" "${@:2}"
