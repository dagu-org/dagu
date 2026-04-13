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

start_bg "core-service" \
  ./scripts/test-package-shard.sh \
  '^github.com/dagucloud/dagu/internal/service(/|$)' \
  '^github.com/dagucloud/dagu/internal/service/frontend/(api/v1|terminal)$'
start_bg "agent" ./scripts/test-agent-shards.sh

wait_bg
