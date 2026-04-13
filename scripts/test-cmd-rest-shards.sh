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

  pids=()
  names=()
  return "$status"
}

start_bg "cmd-package" go test -v -race ./cmd
start_bg "internal-cmd-cleanup" \
  ./scripts/test-shard.sh ./internal/cmd \
  '^(Test(CleanupCommand|CleanupCommandDirectStore|RecordEarlyFailure))$' \
  ''
start_bg "internal-cmd-retry-restart" \
  ./scripts/test-shard.sh ./internal/cmd \
  '^(Test(RetryCommandAcceptsDefaultWorkingDirFlag|RestartCommand|RestartCommand_BuiltExecutableRestoresExplicitEnv|RetryCommand|RetryCommand_BuiltExecutableRestoresExplicitEnv))$' \
  ''
wait_bg

start_bg "internal-cmd-status" \
  ./scripts/test-shard.sh ./internal/cmd \
  '^(TestStatusCommand)$' \
  ''
start_bg "internal-cmd-rest" \
  ./scripts/test-shard.sh ./internal/cmd \
  '' \
  '^(Test(StartCommand|StartCommand_BuiltExecutablePreservesExplicitEnv|CmdStart_.*|CleanupCommand|CleanupCommandDirectStore|RecordEarlyFailure|RetryCommandAcceptsDefaultWorkingDirFlag|RestartCommand|RestartCommand_BuiltExecutableRestoresExplicitEnv|RetryCommand|RetryCommand_BuiltExecutableRestoresExplicitEnv|StatusCommand))$'
wait_bg
