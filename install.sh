#!/bin/bash
# fest is not distributed as a standalone per-repo release binary; it ships
# as part of the Festival packaging repo (camp + fest bundled together).
# This script does not download or install anything itself - it points you
# at the supported, checksum-verified install paths.

set -euo pipefail

cat <<'EOF'
fest ships as part of the Festival packaging repo, not as a standalone
fest-only release binary. Install it one of these ways:

Prebuilt binaries (macOS/Linux, checksum-verified):
  brew install --cask Obedience-Corp/tap/festival

  or run the Festival installer directly:
  curl -fsSL https://raw.githubusercontent.com/Obedience-Corp/festival/main/install.sh | bash

  See https://github.com/Obedience-Corp/festival for the full package list
  (npm, deb, rpm, apk, Arch, checksums.txt).

From source (fest only):
  go install github.com/Obedience-Corp/fest/cmd/fest@latest
EOF

exit 0
