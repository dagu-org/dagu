#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

mode="${1:-all}"

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

  pids=()
  names=()
  return "$status"
}

setup_test_binary ./internal/cmd
trap cleanup_test_binary EXIT

case "$mode" in
  status)
    start_bg "internal-cmd-status" \
      run_filtered_tests \
      '^(TestStatusCommand)$' \
      ''
    wait_bg
    ;;
  rest)
    start_bg "cmd-package" go test -v -race ./cmd
    start_bg "internal-cmd-cleanup" \
      run_filtered_tests \
      '^(Test(CleanupCommand|CleanupCommandDirectStore|RecordEarlyFailure))$' \
      ''
    start_bg "internal-cmd-restart" \
      run_filtered_tests \
      '^(Test(RestartCommand|RestartCommand_BuiltExecutableRestoresExplicitEnv))$' \
      ''
    start_bg "internal-cmd-retry" \
      run_filtered_tests \
      '^(Test(RetryCommandAcceptsDefaultWorkingDirFlag|RetryCommand|RetryCommand_BuiltExecutableRestoresExplicitEnv))$' \
      ''
    wait_bg

    start_bg "internal-cmd-rest" \
      run_sharded_tests 6 \
      '' \
      '^(Test(StartCommand|StartCommand_BuiltExecutablePreservesExplicitEnv|CmdStart_.*|CleanupCommand|CleanupCommandDirectStore|RecordEarlyFailure|RetryCommandAcceptsDefaultWorkingDirFlag|RestartCommand|RestartCommand_BuiltExecutableRestoresExplicitEnv|RetryCommand|RetryCommand_BuiltExecutableRestoresExplicitEnv|StatusCommand))$'
    wait_bg
    ;;
  all)
    "$0" rest
    "$0" status
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 1
    ;;
esac
