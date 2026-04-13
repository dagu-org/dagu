#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 2 || $# -gt 4 ]]; then
  echo "usage: $0 <package> <shard-count> [include-regex] [exclude-regex]" >&2
  exit 1
fi

package="$1"
shard_count="$2"
include_pattern="${3:-}"
exclude_pattern="${4:-}"

package_dir="$(go list -f '{{.Dir}}' "$package")"
tmp_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

goexe="$(go env GOEXE)"
binary="$tmp_dir/$(basename "$package").test${goexe}"

go test -c -race -o "$binary" "$package"

if ! [[ "$shard_count" =~ ^[1-9][0-9]*$ ]]; then
  echo "invalid shard count: $shard_count" >&2
  exit 1
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

if (( shard_count > ${#tests[@]} )); then
  shard_count="${#tests[@]}"
fi

pids=()
names=()

start_bg() {
  local name="$1"
  shift

  echo "Starting $name"
  (
    set -euo pipefail
    "$@"
  ) &
  pids+=("$!")
  names+=("$name")
}

wait_bg() {
  local status=0

  for i in "${!pids[@]}"; do
    if ! wait "${pids[$i]}"; then
      echo "::error::${names[$i]} failed"
      status=1
    else
      echo "Finished ${names[$i]}"
    fi
  done

  return "$status"
}

groups=()
group_counts=()
for ((i = 0; i < shard_count; i++)); do
  groups+=("")
  group_counts+=(0)
done

for i in "${!tests[@]}"; do
  shard=$(( i % shard_count ))
  if [[ -n "${groups[$shard]}" ]]; then
    groups[$shard]+='|'
  fi
  groups[$shard]+="${tests[$i]}"
  group_counts[$shard]=$(( group_counts[$shard] + 1 ))
done

echo "Selected ${#tests[@]} tests for $package across $shard_count shard(s)"
printf '%s\n' "${tests[@]}"

for i in "${!groups[@]}"; do
  if [[ -z "${groups[$i]}" ]]; then
    continue
  fi
  start_bg "shard-$(( i + 1 ))/${#groups[@]} (${group_counts[$i]} tests)" \
    bash -lc "cd \"$package_dir\" && \"$binary\" -test.v -test.timeout=10m -test.run '^(${groups[$i]})$'"
done

wait_bg
