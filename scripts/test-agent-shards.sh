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

setup_test_binary ./internal/agent
trap cleanup_test_binary EXIT

start_bg "agent-api" \
  run_filtered_tests \
  '^(Test(NewAPI|API_|FormatMessageWithContexts|FormatContextLine|SelectModel|GetUserIDFromContext|ShouldCompactMessages_|ShouldCancelStuckSession))' \
  ''
start_bg "agent-runtime" \
  run_filtered_tests \
  '^(Test(RequestCommandApprovalWithTimeout|AskUserRun.*|FormatUserResponse|BashTool_.*|ResolveTimeout|BuildOutput|TruncateOutput|DelegateTool_.*|FilterOutTool|Truncate$|Hooks_.*|TruncateUTF8Bytes_.*|BuildChatInputSpillWrapper_.*|NewLoop|Loop_.*|StartHeartbeatPump|NavigateTool_.*|PatchTool_.*|CountLines|IsDAGFile|ValidateIfDAGFile|ResolveToolPolicy_.*|ValidateToolPolicy|EvaluateBashPolicy|SplitShellCommandSegments|HasUnsupportedShellConstructs|GetModelPresets.*|ProviderCache_.*|HashLLMConfig_.*|CreateLLMProvider_.*|ReadTool_.*|FormatFileContent|TruncateResult|AppendRejectionSummary|RemoteURL|NewRemoteAgentTool_.*|NewListContextsTool_.*))' \
  ''
start_bg "agent-session" \
  run_filtered_tests \
  '' \
  '^(Test(NewAPI|API_|FormatMessageWithContexts|FormatContextLine|SelectModel|GetUserIDFromContext|ShouldCompactMessages_|ShouldCancelStuckSession|RequestCommandApprovalWithTimeout|AskUserRun.*|FormatUserResponse|BashTool_.*|ResolveTimeout|BuildOutput|TruncateOutput|DelegateTool_.*|FilterOutTool|Truncate$|Hooks_.*|TruncateUTF8Bytes_.*|BuildChatInputSpillWrapper_.*|NewLoop|Loop_.*|StartHeartbeatPump|NavigateTool_.*|PatchTool_.*|CountLines|IsDAGFile|ValidateIfDAGFile|ResolveToolPolicy_.*|ValidateToolPolicy|EvaluateBashPolicy|SplitShellCommandSegments|HasUnsupportedShellConstructs|GetModelPresets.*|ProviderCache_.*|HashLLMConfig_.*|CreateLLMProvider_.*|ReadTool_.*|FormatFileContent|TruncateResult|AppendRejectionSummary|RemoteURL|NewRemoteAgentTool_.*|NewListContextsTool_.*))'

wait_bg
