#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 || $# -gt 3 ]]; then
  echo "usage: $0 <package> [include-regex] [exclude-regex]" >&2
  exit 1
fi

package="$1"
include_pattern="${2:-}"
exclude_pattern="${3:-}"

package_dir="$(go list -f '{{.Dir}}' "$package")"
tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

goexe="$(go env GOEXE)"
binary="$tmp_dir/$(basename "$package").test${goexe}"

go test -c -race -o "$binary" "$package"

run_binary() {
  local regex="${1:-}"

  cd "$package_dir"
  if [[ -n "$regex" ]]; then
    exec "$binary" -test.v -test.timeout=10m -test.run "$regex"
  fi
  exec "$binary" -test.v -test.timeout=10m
}

if [[ -z "$include_pattern" && -z "$exclude_pattern" ]]; then
  run_binary
fi

if [[ -n "$include_pattern" && -z "$exclude_pattern" ]]; then
  run_binary "$include_pattern"
fi

if command -v rg >/dev/null 2>&1; then
  match_filter() {
    rg "$1" || true
  }
  exclude_filter() {
    rg -v "$1" || true
  }
  list_filter() {
    rg '^Test' || true
  }
else
  match_filter() {
    grep -E "$1" || true
  }
  exclude_filter() {
    grep -Ev "$1" || true
  }
  list_filter() {
    grep -E '^Test' || true
  }
fi

tests=()
while IFS= read -r test_name; do
	tests+=("$test_name")
done < <(
  (
    cd "$package_dir"
    "$binary" -test.list '^Test'
  ) \
    | list_filter \
    | { if [[ -n "$include_pattern" ]]; then match_filter "$include_pattern"; else cat; fi; } \
    | { if [[ -n "$exclude_pattern" ]]; then exclude_filter "$exclude_pattern"; else cat; fi; }
)

if [[ ${#tests[@]} -eq 0 ]]; then
  echo "no tests selected for $package" >&2
  exit 1
fi

regex="^($(printf '%s\n' "${tests[@]}" | paste -sd'|' -))$"

echo "Selected ${#tests[@]} tests for $package"
printf '%s\n' "${tests[@]}"

run_binary "$regex"
