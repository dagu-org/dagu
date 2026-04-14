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

setup_test_binary ./internal/intg
trap cleanup_test_binary EXIT

start_bg "intg-rest-dag-execution" \
  run_filtered_tests \
  '^TestDAGExecution$' \
  ''
start_bg "intg-rest-env-core" \
  env TEST_BINARY_TIMEOUT=12m run_test_binary \
  '^TestEnv$/(EnvVariables|Derivatives|ShellFallbacks|DAGRunWorkDir)$'
start_bg "intg-rest-env-refs" \
  env TEST_BINARY_TIMEOUT=12m run_test_binary \
  '^TestEnv$/(DAGRunWorkDirWithExplicitWorkingDir|EnvReferencesParams|EnvReferencesParamsChained|StepOutputSubstrings)$'
start_bg "intg-rest-inline-subdag-simple" \
  env TEST_BINARY_TIMEOUT=12m run_test_binary \
  '^TestInlineSubDAG/SimpleExecution$'
start_bg "intg-rest-inline-subdag-nesting" \
  env TEST_BINARY_TIMEOUT=12m run_test_binary \
  '^TestInlineSubDAG/(TwoLevelNesting|ThreeLevelNesting|ThreeLevelNestingWithOutputPassing)$'
start_bg "intg-rest-inline-subdag-flow" \
  env TEST_BINARY_TIMEOUT=12m run_test_binary \
  '^TestInlineSubDAG/(ParallelExecution|ConditionalExecution|OutputPassingBetweenDAGs)$'
start_bg "intg-rest-inline-subdag-outcomes" \
  env TEST_BINARY_TIMEOUT=12m run_test_binary \
  '^TestInlineSubDAG/(NonExistentReference|ComplexDependencies|PartialSuccessParallel|PartialSuccessSubDAG)$'
start_bg "intg-rest-subdag-external-retry" \
  env TEST_BINARY_TIMEOUT=12m run_test_binary \
  '^(TestExternalSubDAG|TestRetryPolicy)$'
start_bg "intg-rest-heavy-logic" \
  run_filtered_tests \
  '^(Test(ComplexDependencies|HandlerOn))$' \
  ''
start_bg "intg-rest-handler-env-core" \
  env TEST_BINARY_TIMEOUT=12m TEST_GO_PARALLEL=1 run_test_binary \
  '^TestHandlerOn_EnvironmentVariables/(InitHandler_BaseEnvVars|InitHandler_DAGRunStatus_IsRunning|SuccessHandler_AllEnvVars|FailureHandler_AllEnvVars)$'
start_bg "intg-rest-handler-env-handlers" \
  env TEST_BINARY_TIMEOUT=12m TEST_GO_PARALLEL=1 run_test_binary \
  '^TestHandlerOn_EnvironmentVariables/(ExitHandler_AllEnvVars_OnSuccess|ExitHandler_AllEnvVars_OnFailure|AbortHandler_AllEnvVars|StepOutputVars_NotAvailableInHandlers|Handlers_CanAccessStepOutputVariables|InitHandler_CannotAccessStepOutputVariables)$'
start_bg "intg-rest-handler-env-wait" \
  env TEST_BINARY_TIMEOUT=12m TEST_GO_PARALLEL=1 run_test_binary \
  '^TestHandlerOn_EnvironmentVariables/(WaitHandler_DAG_WAITING_STEPS_EnvVar|WaitHandler_EnvVarFormat|SuccessHandler_StdoutPathExpandsDAGRunStatus|WaitHandler_StdoutPathExpandsWaitingSteps)$'
start_bg "intg-rest-remainder" \
  run_sharded_tests 4 \
  '^Test([A-M].*)' \
  '^(Test(WaitStepApproval|ApprovalField|HistoryCommand_|DockerExecutor(_.*)?|DAGLevelContainer|StepLevelContainer|Container.*|DAGLevelRedis|MinIOContainer_.*|SFTPExecutorIntegration|SSHExecutorIntegration|RouterExecutor|RouterComplexScenarios|RouterStepStatus|RouterValidation|Server_.*|MailConfigEnvExpansion|WebhookPayloadEnv|JQExecutor|ShellExecution|SQLExecutor_.*|TemplateExecutor|WorkingDirectoryResolution|CallSubDAG|NestedThreeLevelDAG|LargeOutput_128KB|OutputsCollection(_.*)?|OutputValidation_.*|ParamValidation_.*|NoSchema_NoRegression|FullPipeline_ParamAndOutputValidation|Params_.*|InlineParams_.*|Issue1182_.*|Issue1252_.*|ParallelExecution_.*|Issue1274_.*|Issue1658_.*|Issue1790_ParallelCallPathItemResolution|DAGExecution|Env|ComplexDependencies|InlineSubDAG|ExternalSubDAG|HandlerOn(_.*)?))'

wait_bg
