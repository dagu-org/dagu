#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

if [[ $# -lt 2 || $# -gt 4 ]]; then
  echo "usage: $0 <package> <shard-count> [include-regex] [exclude-regex]" >&2
  exit 1
fi

package="$1"
shard_count="$2"
include_pattern="${3:-}"
exclude_pattern="${4:-}"

setup_test_binary "$package"
trap cleanup_test_binary EXIT

run_sharded_tests "$shard_count" "$include_pattern" "$exclude_pattern"
