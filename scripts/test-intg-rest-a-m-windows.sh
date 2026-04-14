#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

pids=()
names=()
status=0

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
  local wait_status=0

  for i in "${!pids[@]}"; do
    if ! wait "${pids[$i]}"; then
      echo "::error::${names[$i]} failed"
      wait_status=1
    else
      echo "Finished ${names[$i]}"
    fi
  done

  return "$wait_status"
}

run_fg() {
  local name="$1"
  shift

  echo "Starting $name"
  if ! "$@"; then
    echo "::error::${name} failed"
    status=1
  else
    echo "Finished ${name}"
  fi
}

run_fg_with_windows_timeout() {
  local name="$1"
  local regex="$2"

  echo "Starting $name"
  if ! (
    TEST_BINARY_TIMEOUT=12m
    TEST_GO_PARALLEL=1
    run_test_binary "$regex"
  ); then
    echo "::error::${name} failed"
    status=1
  else
    echo "Finished ${name}"
  fi
}

setup_test_binary ./internal/intg
trap cleanup_test_binary EXIT

start_bg "intg-rest-dag-execution" \
  run_filtered_tests \
  '^TestDAGExecution$' \
  ''
start_bg "intg-rest-heavy-logic" \
  run_filtered_tests \
  '^(Test(ComplexDependencies|HandlerOn))$' \
  ''
start_bg "intg-rest-remainder" \
  run_sharded_tests 4 \
  '^Test([A-M].*)' \
  '^(Test(WaitStepApproval|ApprovalField|HistoryCommand_|DockerExecutor(_.*)?|DAGLevelContainer|StepLevelContainer|Container.*|DAGLevelRedis|MinIOContainer_.*|SFTPExecutorIntegration|SSHExecutorIntegration|RouterExecutor|RouterComplexScenarios|RouterStepStatus|RouterValidation|Server_.*|MailConfigEnvExpansion|WebhookPayloadEnv|JQExecutor|ShellExecution|SQLExecutor_.*|TemplateExecutor|WorkingDirectoryResolution|CallSubDAG|NestedThreeLevelDAG|LargeOutput_128KB|OutputsCollection(_.*)?|OutputValidation_.*|ParamValidation_.*|NoSchema_NoRegression|FullPipeline_ParamAndOutputValidation|Params_.*|InlineParams_.*|Issue1182_.*|Issue1252_.*|ParallelExecution_.*|Issue1274_.*|Issue1658_.*|Issue1790_ParallelCallPathItemResolution|DAGExecution|Env|ComplexDependencies|InlineSubDAG|ExternalSubDAG|HandlerOn(_.*)?))'

run_fg_with_windows_timeout "intg-rest-env-core" \
  '^TestEnv$/(EnvVariables|Derivatives|ShellFallbacks|DAGRunWorkDir)$'
run_fg_with_windows_timeout "intg-rest-env-refs" \
  '^TestEnv$/(DAGRunWorkDirWithExplicitWorkingDir|EnvReferencesParams|EnvReferencesParamsChained|StepOutputSubstrings)$'
run_fg_with_windows_timeout "intg-rest-inline-subdag-simple" \
  '^TestInlineSubDAG/SimpleExecution$'
run_fg_with_windows_timeout "intg-rest-inline-subdag-nesting" \
  '^TestInlineSubDAG/(TwoLevelNesting|ThreeLevelNesting|ThreeLevelNestingWithOutputPassing)$'
run_fg_with_windows_timeout "intg-rest-inline-subdag-flow" \
  '^TestInlineSubDAG/(ParallelExecution|ConditionalExecution|OutputPassingBetweenDAGs)$'
run_fg_with_windows_timeout "intg-rest-inline-subdag-outcomes" \
  '^TestInlineSubDAG/(NonExistentReference|ComplexDependencies|PartialSuccessParallel|PartialSuccessSubDAG)$'
run_fg_with_windows_timeout "intg-rest-subdag-external-retry" \
  '^(TestExternalSubDAG|TestRetryPolicy)$'
run_fg_with_windows_timeout "intg-rest-handler-env-core" \
  '^TestHandlerOn_EnvironmentVariables/(InitHandler_BaseEnvVars|InitHandler_DAGRunStatus_IsRunning|SuccessHandler_AllEnvVars|FailureHandler_AllEnvVars)$'
run_fg_with_windows_timeout "intg-rest-handler-env-handlers" \
  '^TestHandlerOn_EnvironmentVariables/(ExitHandler_AllEnvVars_OnSuccess|ExitHandler_AllEnvVars_OnFailure|AbortHandler_AllEnvVars|StepOutputVars_NotAvailableInHandlers|Handlers_CanAccessStepOutputVariables|InitHandler_CannotAccessStepOutputVariables)$'
run_fg_with_windows_timeout "intg-rest-handler-env-wait" \
  '^TestHandlerOn_EnvironmentVariables/(WaitHandler_DAG_WAITING_STEPS_EnvVar|WaitHandler_EnvVarFormat|SuccessHandler_StdoutPathExpandsDAGRunStatus|WaitHandler_StdoutPathExpandsWaitingSteps)$'

if ! wait_bg; then
  status=1
fi

exit "$status"
