#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

mode="${1:-all}"

run_parallel_core() {
  TEST_GO_PARALLEL=1 run_sharded_tests 4 \
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

case "$mode" in
  core)
    run_parallel_core
    ;;
  extras)
    run_parallel_issues
    run_parallel_abort
    run_parallel_output_retry
    run_parallel_object_items
    ;;
  all)
    run_parallel_core
    run_parallel_issues
    run_parallel_abort
    run_parallel_output_retry
    run_parallel_object_items
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 1
    ;;
esac
