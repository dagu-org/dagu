#!/usr/bin/env bash

set -euo pipefail

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

start_bg "core-cmd-start" ./scripts/test-core-cmd-start-shards.sh
start_bg "core-cmd-status" ./scripts/test-cmd-rest-shards.sh status
start_bg "core-cmd-rest" ./scripts/test-cmd-rest-shards.sh rest

wait_bg
