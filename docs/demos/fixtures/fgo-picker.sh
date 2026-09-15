#!/usr/bin/env bash
# Disposable navigation fixture; no user config or workspace is modified.
set -euo pipefail

fixture_root="$(mktemp -d "${TMPDIR:-/tmp}/fest-fgo-picker.XXXXXX")"
mkdir -p "$fixture_root/bin" "$fixture_root/config" "$fixture_root/workspace/festivals/.festival"
cp "${FEST_BIN:?set FEST_BIN to the built fest binary}" "$fixture_root/bin/fest"
printf '{"tui":{"theme":"dark"}}\n' > "$fixture_root/config/config.json"
for name in festival-app-FA0023 festival-activity-FA0024 festival-api-FA0025 festival-adoption-FA0026; do
    mkdir -p "$fixture_root/workspace/festivals/active/$name"
    printf '# Festival goal\n\nDemonstrate shell navigation.\n' > "$fixture_root/workspace/festivals/active/$name/FESTIVAL_GOAL.md"
done
printf 'export FGO_FIXTURE=%q\n' "$fixture_root"
printf 'export FEST_CONFIG_DIR=%q\n' "$fixture_root/config"
printf 'export PATH=%q:$PATH\n' "$fixture_root/bin"
