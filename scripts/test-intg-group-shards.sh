#!/usr/bin/env bash

set -euo pipefail

mode="${1:-}"

if [[ -z "$mode" ]]; then
  echo "usage: $0 <general|service-docker|data-subdag|parallel-issues>" >&2
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
  general)
    start_bg "intg-approval" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(Test(WaitStepApproval|ApprovalField))' \
      ''
    start_bg "intg-history" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(TestHistoryCommand_)' \
      ''
    start_bg "intg-output" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(Test(LargeOutput_128KB|OutputsCollection(_.*)?|OutputValidation_.*))' \
      ''
    start_bg "intg-params" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(Test(Params_.*|InlineParams_.*|Issue1182_.*|Issue1252_.*))' \
      ''
    start_bg "intg-queue" \
      ./scripts/test-shard-split.sh ./internal/intg/queue 2
    wait_bg
    ;;
  service-docker)
    start_bg "intg-router" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(Test(RouterExecutor|RouterComplexScenarios|RouterStepStatus|RouterValidation))' \
      ''
    start_bg "intg-server-notify" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(Test(Server_.*|MailConfigEnvExpansion|WebhookPayloadEnv))' \
      ''
    start_bg "intg-docker-storage" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(Test(DockerExecutor(_.*)?|DAGLevelContainer|StepLevelContainer|Container.*|DAGLevelRedis|MinIOContainer_.*|SFTPExecutorIntegration|SSHExecutorIntegration))' \
      ''
    wait_bg
    ;;
  data-subdag)
    start_bg "intg-data-shell" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(Test(JQExecutor|ShellExecution|SQLExecutor_.*|TemplateExecutor|WorkingDirectoryResolution))' \
      ''
    start_bg "intg-subdag" \
      ./scripts/test-shard.sh ./internal/intg \
      '^(Test(CallSubDAG|NestedThreeLevelDAG|ParamValidation_.*|NoSchema_NoRegression|FullPipeline_ParamAndOutputValidation))' \
      ''
    wait_bg
    ;;
  parallel-issues)
    ./scripts/test-shard-split.sh ./internal/intg 3 \
      '^(Test(Issue1274_.*|Issue1658_.*|Issue1790_ParallelCallPathItemResolution))' \
      ''
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 1
    ;;
esac
