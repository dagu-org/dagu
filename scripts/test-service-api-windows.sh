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

setup_test_binary ./internal/service/frontend/api/v1
trap cleanup_test_binary EXIT

start_bg "core-service-api-main" \
  run_sharded_tests 4 \
  '' \
  '^(Test(ApproveDAGRunStep(|WithInputs|MissingRequired|NotWaiting)|RejectDAGRunStep(|NotWaiting)|ExecuteDAGSync(|Timeout|WithWaitingStatus|Singleton)|GetSubDAGRunSpec|Webhooks_RequiresDeveloperOrAbove|Webhooks_TriggerInvalidToken))$'
start_bg "core-service-api-slow" \
  run_filtered_tests \
  '^(Test(ApproveDAGRunStep(|WithInputs|MissingRequired|NotWaiting)|RejectDAGRunStep(|NotWaiting)|Webhooks_RequiresDeveloperOrAbove|Webhooks_TriggerInvalidToken))$' \
  ''
start_bg "core-service-api-sync-subdag" \
  run_filtered_tests \
  '^(Test(ExecuteDAGSync(|Timeout|WithWaitingStatus|Singleton)|GetSubDAGRunSpec))$' \
  ''

wait_bg
