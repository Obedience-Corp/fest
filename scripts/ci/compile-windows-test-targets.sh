#!/usr/bin/env bash
# Cross-compile fest test packages for Windows amd64 (compile-only; no test execution).
# Enumerates packages with go list, surfaces list/compile failures, and prints counts.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$repo_root"

export GOOS=windows
export GOARCH=amd64
export CGO_ENABLED=0

out_dir="$(mktemp -d)"
trap 'rm -rf "$out_dir"' EXIT

gate_failed=0
total_list_errors=0
total_packages=0
total_compiled=0
total_compile_failures=0

echo "=== windows test compile gate ==="
echo "GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=${CGO_ENABLED}"
echo "output dir (temp): ${out_dir}"
echo ""

safe_name() {
  echo "$1" | tr '/.' '__'
}

compile_profile() {
  local profile_name="$1"
  local tags="${2:-}"

  local -a tag_args=()
  local tags_label="(none)"
  if [[ -n "$tags" ]]; then
    tag_args=(-tags="$tags")
    tags_label="$tags"
  fi

  echo "--- profile: ${profile_name} tags=${tags_label} ---"

  local list_errors=0
  local -a compile_pkgs=()

  local listed
  if ! listed="$(go list -e "${tag_args[@]}" -f \
      '{{if .Error}}err{{"\t"}}{{.ImportPath}}{{"\t"}}{{.Error}}{{else if or (len .TestGoFiles) (len .XTestGoFiles)}}pkg{{"\t"}}{{.ImportPath}}{{end}}' \
      ./...)"; then
    echo "  LIST ERROR: go list failed for profile ${profile_name}" >&2
    gate_failed=1
    total_list_errors=$((total_list_errors + 1))
    return
  fi

  local line kind import_path list_err
  while IFS=$'\t' read -r kind import_path list_err || [[ -n "$kind" ]]; do
    [[ -z "$kind" ]] && continue
    case "$kind" in
      err)
        list_errors=$((list_errors + 1))
        echo "  LIST ERROR: ${import_path}: ${list_err}" >&2
        gate_failed=1
        ;;
      pkg)
        compile_pkgs+=("$import_path")
        ;;
    esac
  done <<< "$listed"

  local pkg_count="${#compile_pkgs[@]}"
  total_list_errors=$((total_list_errors + list_errors))
  total_packages=$((total_packages + pkg_count))

  echo "  packages with tests: ${pkg_count}"
  echo "  go list errors: ${list_errors}"

  if [[ "$pkg_count" -eq 0 ]]; then
    echo "  LIST ERROR: no Windows test packages found for profile ${profile_name}" >&2
    gate_failed=1
    total_list_errors=$((total_list_errors + 1))
    return
  fi

  local compile_ok=0
  local compile_fail=0

  for import_path in "${compile_pkgs[@]}"; do
    local out_file="${out_dir}/$(safe_name "$import_path").test.exe"
    local log_file
    log_file="$(mktemp)"
    if go test -c "${tag_args[@]}" -o "$out_file" "$import_path" >"$log_file" 2>&1 && [[ -s "$out_file" ]]; then
      compile_ok=$((compile_ok + 1))
    else
      compile_fail=$((compile_fail + 1))
      gate_failed=1
      echo "  COMPILE ERROR: ${import_path}" >&2
      sed 's/^/    /' "$log_file" >&2
    fi
    rm -f "$log_file"
  done

  total_compiled=$((total_compiled + compile_ok))
  total_compile_failures=$((total_compile_failures + compile_fail))

  echo "  compile ok: ${compile_ok}"
  echo "  compile failed: ${compile_fail}"
  echo ""
}

compile_profile stable ""
compile_profile dev "dev"
compile_profile integration "integration"
compile_profile integration-dev "integration,dev"

echo "=== summary ==="
echo "profiles: 4 (stable, dev, integration, integration-dev)"
echo "GOOS=${GOOS} GOARCH=${GOARCH}"
echo "test packages enumerated: ${total_packages}"
echo "go list errors: ${total_list_errors}"
echo "compiled ok: ${total_compiled}"
echo "compile failures: ${total_compile_failures}"

if [[ "$gate_failed" -ne 0 ]]; then
  echo "FAIL: windows test compile gate" >&2
  exit 1
fi

echo "PASS: windows test compile gate"
