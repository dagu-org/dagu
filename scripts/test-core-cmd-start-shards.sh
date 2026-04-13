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

start_bg "internal-cmd-start-positional" \
  ./scripts/test-shard.sh ./internal/cmd \
  '^(TestCmdStart_PositionalParamValidation)$' \
  ''
start_bg "internal-cmd-start-other" \
  ./scripts/test-shard.sh ./internal/cmd \
  '^(Test(StartCommand|StartCommand_BuiltExecutablePreservesExplicitEnv|CmdStart_.*))' \
  '^(TestCmdStart_PositionalParamValidation)$'
wait_bg
