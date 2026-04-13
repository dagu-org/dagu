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
    start_bg "core-entrypoints" \
      ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/(api/v1|proto/coordinator/v1|proto/index/v1|skills)$|^github.com/dagucloud/dagu/internal/(agent/schema|agentoauth|agentsnapshot|auth($|/tokensecret$)|clicontext|license|output|proto/convert|remotenode|test($|util$)|tunnel|upgrade|workspace)$'
    start_bg "core-cmn" \
      ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/internal/cmn(/|$)'
    wait_bg
    ;;
  b)
    start_bg "core-core-llm" \
      ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/internal/(core(/|$)|gitsync$|llm(/|$))'
    start_bg "core-persis" \
      ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/internal/persis(/|$)'
    wait_bg
    ;;
  c)
    start_bg "core-runtime-subpackages" \
      ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/internal/runtime/(executor|remote|transform)$'
    start_bg "core-runtime-builtin" \
      ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/internal/runtime/builtin(/|$)'
    start_bg "core-service-terminal" \
      ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/internal/service/frontend/terminal$'
    wait_bg
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 1
    ;;
esac
