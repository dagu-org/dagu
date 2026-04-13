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

start_bg "core-packages-a" ./scripts/test-core-package-shards.sh a
start_bg "core-packages-b" ./scripts/test-core-package-shards.sh b
start_bg "core-packages-c" ./scripts/test-core-package-shards.sh c

wait_bg
