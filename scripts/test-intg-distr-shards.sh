#!/usr/bin/env bash

set -euo pipefail

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
  a)
    start_bg "intg-distr-baseconfig-status" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(Test(BaseConfig_.*|Coordinator_.*|Execution_(StatusPushing|LogStreaming|LargeOutput)))' \
      ''
    start_bg "intg-distr-direct-queue" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(TestExecution_(StartCommand|TagsPropagation|SharedFSMode|WorkDir|QueueLifecycle|QueuedCatchupHappyPath))' \
      ''
    start_bg "intg-distr-retry-cancel" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(Test(Cancellation_.*|Retry_.*|OneOffScheduleRunsDistributed))' \
      ''
    wait_bg
    ;;
  b)
    start_bg "intg-distr-parallel" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(Test(Parallel_.*))' \
      ''
    start_bg "intg-distr-params" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(Test(Params_.*))' \
      ''
    start_bg "intg-distr-proc-heartbeat" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(TestExecution_ProcHeartbeat_.*)' \
      ''
    start_bg "intg-distr-queued-dispatch" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(TestExecution_QueuedDispatch_(RecoversWhenWorkerRegistersLater|RecoversWhenMatchingWorkerRegistersLater))' \
      ''
    wait_bg
    ;;
  c)
    start_bg "intg-distr-queued-dispatch-heavy" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(TestExecution_QueuedDispatch_ConsumesOneThousandItems)' \
      ''
    start_bg "intg-distr-rest" \
      ./scripts/test-shard.sh ./internal/intg/distr \
      '^(Test(CustomStepTypes_.*|SubDAG_.*|DistributedRun_.*))' \
      ''
    wait_bg
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 1
    ;;
esac
