#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 || $# -gt 3 ]]; then
  echo "usage: $0 <package> [include-regex] [exclude-regex]" >&2
  exit 1
fi

package="$1"
include_pattern="${2:-}"
exclude_pattern="${3:-}"

if [[ -z "$include_pattern" && -z "$exclude_pattern" ]]; then
  exec go test -v -race "$package"
fi

tests=()
while IFS= read -r test_name; do
  tests+=("$test_name")
done < <(
  go test -list '^Test' "$package" \
    | rg '^Test' \
    | { if [[ -n "$include_pattern" ]]; then rg "$include_pattern"; else cat; fi; } \
    | { if [[ -n "$exclude_pattern" ]]; then rg -v "$exclude_pattern"; else cat; fi; }
)

if [[ ${#tests[@]} -eq 0 ]]; then
  echo "no tests selected for $package" >&2
  exit 1
fi

regex="^($(printf '%s\n' "${tests[@]}" | paste -sd'|' -))$"

echo "Selected ${#tests[@]} tests for $package"
printf '%s\n' "${tests[@]}"

exec go test -v -race "$package" -run "$regex"
