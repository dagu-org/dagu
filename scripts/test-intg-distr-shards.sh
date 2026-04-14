#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

mode="${1:-}"

if [[ -z "$mode" ]]; then
  echo "usage: $0 <a|b|c>" >&2
  exit 1
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

case "$mode" in
  a-core)
    setup_test_binary ./internal/intg/distr
    trap cleanup_test_binary EXIT
    echo "Starting intg-distr-baseconfig-status"
    run_filtered_tests \
      '^(Test(BaseConfig_.*|Coordinator_.*|Execution_(StatusPushing|LogStreaming|LargeOutput)))' \
      ''
    echo "Finished intg-distr-baseconfig-status"

    echo "Starting intg-distr-direct-metadata"
    run_filtered_tests \
      '^(TestExecution_(StartCommand|TagsPropagation|QueueLifecycle|QueuedCatchupHappyPath))' \
      ''
    echo "Finished intg-distr-direct-metadata"

    echo "Starting intg-distr-sharedfs-workdir"
    run_filtered_tests \
      '^(TestExecution_(SharedFSMode|WorkDir))' \
      ''
    echo "Finished intg-distr-sharedfs-workdir"
    ;;
  a-retry)
    setup_test_binary ./internal/intg/distr
    trap cleanup_test_binary EXIT
    echo "Starting intg-distr-retry-cancel"
    TEST_GO_PARALLEL=1 TEST_BINARY_TIMEOUT=12m run_filtered_tests \
      '^(Test(Cancellation_.*|Retry_.*|OneOffScheduleRunsDistributed))' \
      ''
    echo "Finished intg-distr-retry-cancel"
    ;;
  a)
    "$0" a-core
    "$0" a-retry
    ;;
  b)
    setup_test_binary ./internal/intg/distr
    trap cleanup_test_binary EXIT
    echo "Starting intg-distr-proc-heartbeat"
    TEST_GO_PARALLEL=1 run_filtered_tests \
      '^(TestExecution_ProcHeartbeat_.*)' \
      ''
    echo "Finished intg-distr-proc-heartbeat"

    start_bg "intg-distr-parallel" \
      run_filtered_tests \
      '^(Test(Parallel_.*))' \
      ''
    start_bg "intg-distr-params" \
      run_filtered_tests \
      '^(Test(Params_.*))' \
      ''
    wait_bg
    TEST_GO_PARALLEL=1 run_filtered_tests \
      '^(TestExecution_QueuedDispatch_(RecoversWhenWorkerRegistersLater|RecoversWhenMatchingWorkerRegistersLater))' \
      ''
    ;;
  c)
    setup_test_binary ./internal/intg/distr
    trap cleanup_test_binary EXIT
    start_bg "intg-distr-queued-dispatch-heavy" \
      run_filtered_tests \
      '^(TestExecution_QueuedDispatch_ConsumesOneThousandItems)' \
      ''
    start_bg "intg-distr-rest" \
      run_filtered_tests \
      '^(Test(CustomStepTypes_.*|SubDAG_.*|DistributedRun_.*))' \
      ''
    wait_bg
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 1
    ;;
esac
