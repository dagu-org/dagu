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

start_bg "agent-api" \
  ./scripts/test-shard.sh ./internal/agent \
  '^(Test(NewAPI|API_|FormatMessageWithContexts|FormatContextLine|SelectModel|GetUserIDFromContext|ShouldCompactMessages_|ShouldCancelStuckSession))' \
  ''
start_bg "agent-runtime" \
  ./scripts/test-shard.sh ./internal/agent \
  '^(Test(RequestCommandApprovalWithTimeout|AskUserRun.*|FormatUserResponse|BashTool_.*|ResolveTimeout|BuildOutput|TruncateOutput|DelegateTool_.*|FilterOutTool|Truncate$|Hooks_.*|TruncateUTF8Bytes_.*|BuildChatInputSpillWrapper_.*|NewLoop|Loop_.*|StartHeartbeatPump|NavigateTool_.*|PatchTool_.*|CountLines|IsDAGFile|ValidateIfDAGFile|ResolveToolPolicy_.*|ValidateToolPolicy|EvaluateBashPolicy|SplitShellCommandSegments|HasUnsupportedShellConstructs|GetModelPresets.*|ProviderCache_.*|HashLLMConfig_.*|CreateLLMProvider_.*|ReadTool_.*|FormatFileContent|TruncateResult|AppendRejectionSummary|RemoteURL|NewRemoteAgentTool_.*|NewListContextsTool_.*))' \
  ''
start_bg "agent-session" \
  ./scripts/test-shard.sh ./internal/agent \
  '' \
  '^(Test(NewAPI|API_|FormatMessageWithContexts|FormatContextLine|SelectModel|GetUserIDFromContext|ShouldCompactMessages_|ShouldCancelStuckSession|RequestCommandApprovalWithTimeout|AskUserRun.*|FormatUserResponse|BashTool_.*|ResolveTimeout|BuildOutput|TruncateOutput|DelegateTool_.*|FilterOutTool|Truncate$|Hooks_.*|TruncateUTF8Bytes_.*|BuildChatInputSpillWrapper_.*|NewLoop|Loop_.*|StartHeartbeatPump|NavigateTool_.*|PatchTool_.*|CountLines|IsDAGFile|ValidateIfDAGFile|ResolveToolPolicy_.*|ValidateToolPolicy|EvaluateBashPolicy|SplitShellCommandSegments|HasUnsupportedShellConstructs|GetModelPresets.*|ProviderCache_.*|HashLLMConfig_.*|CreateLLMProvider_.*|ReadTool_.*|FormatFileContent|TruncateResult|AppendRejectionSummary|RemoteURL|NewRemoteAgentTool_.*|NewListContextsTool_.*))'
wait_bg
