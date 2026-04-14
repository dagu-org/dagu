#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

mode="${1:-}"

if [[ -z "$mode" ]]; then
  echo "usage: $0 <general|service-docker|data-subdag|parallel-issues|parallel-issue-1274|parallel-issue-1658|parallel-issue-1790>" >&2
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
    setup_test_binary ./internal/intg
    trap cleanup_test_binary EXIT
    start_bg "intg-approval" \
      run_filtered_tests \
      '^(Test(WaitStepApproval|ApprovalField))' \
      ''
    start_bg "intg-history" \
      run_filtered_tests \
      '^(TestHistoryCommand_)' \
      ''
    start_bg "intg-output-core" \
      run_filtered_tests \
      '^(Test(LargeOutput_128KB|OutputsCollection_MixedOutputConfigurations|OutputsCollection_FailedDAG|OutputValidation_.*))' \
      ''
    start_bg "intg-output-camelcase" \
      run_sharded_tests 4 \
      '^(TestOutputsCollection_CamelCaseConversion_.*)$' \
      ''
    start_bg "intg-params" \
      run_sharded_tests 2 \
      '^(Test(Params_.*|InlineParams_.*|Issue1182_.*|Issue1252_.*))' \
      ''
    wait_bg
    TEST_GO_PARALLEL=1 ./scripts/test-shard-split.sh ./internal/intg/queue 1
    ;;
  service-docker)
    setup_test_binary ./internal/intg
    trap cleanup_test_binary EXIT
    start_bg "intg-router" \
      run_filtered_tests \
      '^(Test(RouterExecutor|RouterComplexScenarios|RouterStepStatus|RouterValidation))' \
      ''
    start_bg "intg-docker-core" \
      run_filtered_tests \
      '^(Test(DockerExecutor(_.*)?|DAGLevelContainer|StepLevelContainer|Container.*))' \
      ''
    start_bg "intg-storage-remote" \
      run_filtered_tests \
      '^(Test(DAGLevelRedis|MinIOContainer_.*|SFTPExecutorIntegration|SSHExecutorIntegration))' \
      ''
    wait_bg
    TEST_GO_PARALLEL=1 run_filtered_tests \
      '^(Test(Server_.*|MailConfigEnvExpansion|WebhookPayloadEnv))' \
      ''
    ;;
  data-subdag)
    setup_test_binary ./internal/intg
    trap cleanup_test_binary EXIT
    start_bg "intg-data-shell" \
      run_filtered_tests \
      '^(Test(JQExecutor|ShellExecution|SQLExecutor_.*|TemplateExecutor|WorkingDirectoryResolution))' \
      ''
    start_bg "intg-subdag" \
      run_filtered_tests \
      '^(Test(CallSubDAG|NestedThreeLevelDAG|ParamValidation_.*|NoSchema_NoRegression|FullPipeline_ParamAndOutputValidation))' \
      ''
    wait_bg
    ;;
  parallel-issues)
    setup_test_binary ./internal/intg
    trap cleanup_test_binary EXIT
    run_sharded_tests 4 \
      '^(Test(Issue1274_.*|Issue1658_.*|Issue1790_ParallelCallPathItemResolution))' \
      ''
    ;;
  parallel-issue-1274)
    setup_test_binary ./internal/intg
    trap cleanup_test_binary EXIT
    run_filtered_tests '^(TestIssue1274_.*)$' ''
    ;;
  parallel-issue-1658)
    setup_test_binary ./internal/intg
    trap cleanup_test_binary EXIT
    run_filtered_tests '^(TestIssue1658_.*)$' ''
    ;;
  parallel-issue-1790)
    setup_test_binary ./internal/intg
    trap cleanup_test_binary EXIT
    run_filtered_tests '^(TestIssue1790_ParallelCallPathItemResolution)$' ''
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 1
    ;;
esac
