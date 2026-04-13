#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

if [[ $# -lt 1 || $# -gt 3 ]]; then
  echo "usage: $0 <package> [include-regex] [exclude-regex]" >&2
  exit 1
fi

package="$1"
include_pattern="${2:-}"
exclude_pattern="${3:-}"

setup_test_binary "$package"
trap cleanup_test_binary EXIT

run_filtered_tests "$include_pattern" "$exclude_pattern"
