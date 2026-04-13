#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

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

setup_test_binary ./internal/intg
trap cleanup_test_binary EXIT

start_bg "intg-parallel-core" \
  run_sharded_tests 8 \
  '^(Test(ParallelExecution_.*))' \
  ''
start_bg "intg-parallel-issues" \
  run_sharded_tests 4 \
  '^(Test(Issue1274_.*|Issue1658_.*|Issue1790_ParallelCallPathItemResolution))' \
  ''

wait_bg
