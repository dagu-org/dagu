#!/usr/bin/env bash

if command -v rg >/dev/null 2>&1; then
  shard_match_filter() {
    rg "$1" || true
  }
  shard_exclude_filter() {
    rg -v "$1" || true
  }
  shard_list_filter() {
    rg '^Test' || true
  }
else
  shard_match_filter() {
    grep -E "$1" || true
  }
  shard_exclude_filter() {
    grep -Ev "$1" || true
  }
  shard_list_filter() {
    grep -E '^Test' || true
  }
fi

setup_test_binary() {
  TEST_SHARD_PACKAGE="${1:?package is required}"
  TEST_SHARD_PACKAGE_DIR="$(go list -f '{{.Dir}}' "$TEST_SHARD_PACKAGE")"
  TEST_SHARD_TMP_DIR="$(mktemp -d)"

  local goexe
  goexe="$(go env GOEXE)"
  TEST_SHARD_BINARY="$TEST_SHARD_TMP_DIR/$(basename "$TEST_SHARD_PACKAGE").test${goexe}"

  go test -c -race -o "$TEST_SHARD_BINARY" "$TEST_SHARD_PACKAGE"
}

cleanup_test_binary() {
  if [[ -n "${TEST_SHARD_TMP_DIR:-}" ]]; then
    rm -rf "$TEST_SHARD_TMP_DIR"
  fi
}

list_selected_tests() {
  local include_pattern="${1:-}"
  local exclude_pattern="${2:-}"

  (
    cd "$TEST_SHARD_PACKAGE_DIR"
    "$TEST_SHARD_BINARY" -test.list '^Test'
  ) \
    | shard_list_filter \
    | { if [[ -n "$include_pattern" ]]; then shard_match_filter "$include_pattern"; else cat; fi; } \
    | { if [[ -n "$exclude_pattern" ]]; then shard_exclude_filter "$exclude_pattern"; else cat; fi; }
}

run_test_binary() {
  local regex="${1:-}"
  local timeout="${TEST_BINARY_TIMEOUT:-10m}"
  local parallel="${TEST_GO_PARALLEL:-}"
  local args=(
    -test.v
    "-test.timeout=${timeout}"
  )

  if [[ -n "$parallel" ]]; then
    args+=("-test.parallel=${parallel}")
  fi

  if [[ -n "$regex" ]]; then
    args+=(-test.run "$regex")
  fi

  (
    cd "$TEST_SHARD_PACKAGE_DIR"
    exec "$TEST_SHARD_BINARY" "${args[@]}"
  )
}

run_filtered_tests() {
  local include_pattern="${1:-}"
  local exclude_pattern="${2:-}"

  if [[ -z "$include_pattern" && -z "$exclude_pattern" ]]; then
    run_test_binary
    return
  fi

  local tests=()
  while IFS= read -r test_name; do
    tests+=("$test_name")
  done < <(list_selected_tests "$include_pattern" "$exclude_pattern")

  if [[ ${#tests[@]} -eq 0 ]]; then
    echo "no tests selected for $TEST_SHARD_PACKAGE" >&2
    return 1
  fi

  local regex
  regex="^($(printf '%s\n' "${tests[@]}" | paste -sd'|' -))$"

  echo "Selected ${#tests[@]} tests for $TEST_SHARD_PACKAGE"
  printf '%s\n' "${tests[@]}"

  run_test_binary "$regex"
}

run_sharded_tests() {
  local shard_count="${1:?shard count is required}"
  local include_pattern="${2:-}"
  local exclude_pattern="${3:-}"

  if ! [[ "$shard_count" =~ ^[1-9][0-9]*$ ]]; then
    echo "invalid shard count: $shard_count" >&2
    return 1
  fi

  local tests=()
  while IFS= read -r test_name; do
    tests+=("$test_name")
  done < <(list_selected_tests "$include_pattern" "$exclude_pattern")

  if [[ ${#tests[@]} -eq 0 ]]; then
    echo "no tests selected for $TEST_SHARD_PACKAGE" >&2
    return 1
  fi

  if (( shard_count > ${#tests[@]} )); then
    shard_count="${#tests[@]}"
  fi

  local groups=()
  local group_counts=()
  local pids=()
  local names=()
  local i shard regex status=0

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

  echo "Selected ${#tests[@]} tests for $TEST_SHARD_PACKAGE across $shard_count shard(s)"
  printf '%s\n' "${tests[@]}"

  for i in "${!groups[@]}"; do
    if [[ -z "${groups[$i]}" ]]; then
      continue
    fi

    names+=("shard-$(( i + 1 ))/${#groups[@]} (${group_counts[$i]} tests)")
    echo "Starting ${names[$i]}"
    regex="^(${groups[$i]})$"
    (
      set -euo pipefail
      run_test_binary "$regex"
    ) &
    pids+=("$!")
  done

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
