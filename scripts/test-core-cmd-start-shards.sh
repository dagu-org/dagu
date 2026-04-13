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

setup_test_binary ./internal/cmd
trap cleanup_test_binary EXIT

start_bg "internal-cmd-start-positional" \
  run_filtered_tests \
  '^(TestCmdStart_PositionalParamValidation)$' \
  ''
start_bg "internal-cmd-start-other" \
  run_sharded_tests 3 \
  '^(Test(StartCommand|StartCommand_BuiltExecutablePreservesExplicitEnv|CmdStart_.*))' \
  '^(TestCmdStart_PositionalParamValidation)$'

wait_bg
