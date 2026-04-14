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

run_parallel_core() {
  TEST_GO_PARALLEL=2 run_sharded_tests 12 \
    '^(Test(ParallelExecution_.*))' \
    '^(Test(ParallelExecution_(AbortSuppressesPendingRetry|OutputCaptureWithRetry|ObjectItemProperties)))$'
}

run_parallel_issues() {
  TEST_GO_PARALLEL=1 run_sharded_tests 2 \
    '^(Test(Issue1274_.*|Issue1658_.*|Issue1790_ParallelCallPathItemResolution))' \
    ''
}

run_parallel_abort() {
  TEST_GO_PARALLEL=1 run_test_binary '^TestParallelExecution_AbortSuppressesPendingRetry$'
}

run_parallel_output_retry() {
  TEST_GO_PARALLEL=1 run_test_binary '^TestParallelExecution_OutputCaptureWithRetry$'
}

run_parallel_object_items() {
  TEST_GO_PARALLEL=1 run_test_binary '^TestParallelExecution_ObjectItemProperties$'
}

setup_test_binary ./internal/intg
trap cleanup_test_binary EXIT

start_bg "intg-parallel-core" \
  run_parallel_core
start_bg "intg-parallel-issues" \
  run_parallel_issues
start_bg "intg-parallel-abort" \
  run_parallel_abort
start_bg "intg-parallel-output-retry" \
  run_parallel_output_retry
start_bg "intg-parallel-object-items" \
  run_parallel_object_items

wait_bg
