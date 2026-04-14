#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <include-regex> [exclude-regex]" >&2
  exit 1
fi

include_pattern="$1"
exclude_pattern="${2:-}"

if command -v rg >/dev/null 2>&1; then
  match_filter() {
    rg "$1" || true
  }
  exclude_filter() {
    rg -v "$1" || true
  }
else
  match_filter() {
    grep -E "$1" || true
  }
  exclude_filter() {
    grep -Ev "$1" || true
  }
fi

packages=()
while IFS= read -r package; do
  packages+=("$package")
done < <(
  go list ./... \
    | match_filter "$include_pattern" \
    | { if [[ -n "$exclude_pattern" ]]; then exclude_filter "$exclude_pattern"; else cat; fi; }
)

if [[ ${#packages[@]} -eq 0 ]]; then
  echo "no packages selected" >&2
  exit 1
fi

echo "Selected ${#packages[@]} packages"
printf '%s\n' "${packages[@]}"

args=(
  -v
  -race
)

if [[ -n "${TEST_GO_PARALLEL:-}" ]]; then
  args+=(-parallel "${TEST_GO_PARALLEL}")
fi

exec go test "${args[@]}" "${packages[@]}"
