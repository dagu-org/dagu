#!/usr/bin/env bash

set -euo pipefail

source "$(dirname "$0")/test-shard-lib.sh"

mode="${1:-all}"

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

start_bg_run_test_binary() {
  local name="$1"
  local timeout="$2"
  local regex="$3"

  echo "Starting $name"
  (
    set -euo pipefail
    TEST_BINARY_TIMEOUT="$timeout" run_test_binary "$regex"
  ) &
  pids+=("$!")
  names+=("$name")
}

case "$mode" in
  base-a-early)
    setup_test_binary ./internal/runtime
    trap cleanup_test_binary EXIT
    start_bg "runtime-runner-early" \
      run_test_binary '^TestRunner$/(SequentialStepsSuccess|SequentialStepsWithFailure|ParallelSteps|ParallelStepsWithFailure|ComplexCommand|ContinueOnFailure|ContinueOnSkip|ContinueOnExitCode|ContinueOnOutputStdout|ContinueOnOutputStderr|ContinueOnOutputRegexp|ContinueOnMarkSuccess|Cancel|Timeout)$'
    start_bg "runtime-runner-repeat-cancel" \
      run_test_binary '^TestRunner_RepeatPolicyWithCancel$'
    wait_bg
    ;;
  base-a-rest)
    setup_test_binary ./internal/runtime
    trap cleanup_test_binary EXIT
    run_sharded_tests 6 \
      '' \
      '^(TestRunner$|TestRunner_ErrorHandling$|TestRunner_DAGPreconditions$|TestRunner_StatusDefersForcedStatusUntilTerminal$|TestRunner_SignalHandling$|TestRunner_ComplexDependencyChains$|TestRunner_EdgeCases$|TestRunner_ComplexRetryScenarios$|TestRunner_StepRetryExecution$|TestRunner_StepIDAccess$|TestRunner_EventHandlerStepIDAccess$|TestRunnerPartialSuccess$|TestRunner_ChatMessagesHandler$|TestSetupPushBackConversation$|TestWaitStep$|TestRunner_RepeatPolicyWithCancel$)$'
    ;;
  base-a-early-rest)
    "$0" base-a-early
    "$0" base-a-rest
    ;;
  base-a-retry-output)
    setup_test_binary ./internal/runtime
    trap cleanup_test_binary EXIT
    start_bg "runtime-runner-repeat" \
      run_test_binary '^TestRunner$/(Repeat|RepeatFail|StopRepetitiveTaskGracefully)$'
    start_bg "runtime-runner-retry-handlers" \
      run_test_binary '^TestRunner$/(RetryPolicyFail|RetryWithScript|RetryPolicySuccess|OnExitHandler|OnExitHandlerFail|OnAbortHandler|OnSuccessHandler|OnFailureHandler|CancelOnSignal|WorkingDirNoExist)$'
    start_bg "runtime-runner-preconditions" \
      run_test_binary '^TestRunner$/(PreconditionMatch|PreconditionNotMatch|PreconditionWithCommandMet|PreconditionWithCommandNotMet)$'
    start_bg "runtime-runner-output-specialvars" \
      run_test_binary '^TestRunner$/(OutputVariables|OutputInheritance|OutputJSONReference|HandlingJSONWithSpecialChars|SpecialVarsDAGRUNLOGFILE|SpecialVarsDAGRUNSTEPSTDOUTFILE|SpecialVarsDAGRUNSTEPSTDERRFILE|SpecialVarsDAGRUNID|SpecialVarsDAGNAME|SpecialVarsDAGRUNSTEPNAME|StdoutPathExpandsStepNameBeforePrepare|StdoutPathExpandsStepEnvBeforePrepare|StdoutPathExpandsUpstreamStepRefBeforePrepare|DAGRunStatusNotAvailableToMainSteps)$'
    wait_bg
    ;;
  base-b-policies-advanced)
    setup_test_binary ./internal/runtime
    trap cleanup_test_binary EXIT
    echo "Starting runtime-runner-advanced-parents"
    run_test_binary '^(TestRunner_ErrorHandling|TestRunner_DAGPreconditions|TestRunner_StatusDefersForcedStatusUntilTerminal|TestRunner_SignalHandling|TestRunner_ComplexDependencyChains|TestRunner_EdgeCases)$'
    echo "Finished runtime-runner-advanced-parents"

    start_bg_run_test_binary "runtime-runner-repeat-conditions" 8m \
      '^TestRunner$/(RepeatPolicyRepeatsUntilCommandConditionMatchesExpected|RepeatPolicyRepeatWhileConditionExits0|RepeatPolicyRepeatsWhileCommandExitCodeMatches)$'
    start_bg_run_test_binary "runtime-runner-repeat-sources" 8m \
      '^TestRunner$/(RepeatPolicyRepeatsUntilFileConditionMatchesExpected|RepeatPolicyRepeatsUntilOutputVarConditionMatchesExpected)$'
    start_bg_run_test_binary "runtime-runner-repeat-timeouts" 8m \
      '^TestRunner$/(RetryPolicyWithOutputCapture|FailedStepWithOutputCapture|RetryPolicySubDAGRunWithOutputCapture|SingleStepTimeoutFailsStep|TimeoutPreemptsRetriesAndMarksFailed|ParallelStepsTimeoutFailIndividually|StepLevelTimeoutOverridesLongDAGTimeoutAndFails|RejectedTakesPrecedenceOverWaiting)$'
    wait_bg

    start_bg_run_test_binary "runtime-runner-complex-retry-basic" 8m \
      '^TestRunner_ComplexRetryScenarios/(RetryWithSignalTermination|RetryWithSpecificExitCodes|RepeatPolicyBooleanTrueRepeatsWhileStepSucceeds|RepeatPolicyBooleanTrueWithFailureStopsOnFailure|RepeatPolicyUntilModeWithoutConditionRepeatsOnFailure|RepeatPolicyLimit)$'
    start_bg_run_test_binary "runtime-runner-complex-retry-conditional" 8m \
      '^TestRunner_ComplexRetryScenarios/(RepeatPolicyWhileWithConditionRepeatsWhileConditionSucceeds|RepeatPolicyWhileWithConditionAndExpectedRepeatsWhileMatches|RepeatPolicyUntilWithConditionRepeatsUntilConditionSucceeds|RepeatPolicyUntilWithConditionAndExpectedRepeatsUntilMatches|RepeatPolicyUntilWithExitCodeRepeatsUntilExitCodeMatches|RepeatPolicyOutputVariablesReloadedBeforeConditionEval)$'
    wait_bg
    ;;
  base-b-agent)
    start_bg "runtime-agent-subpackage" \
      env TEST_GO_PARALLEL=1 ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/internal/runtime/agent$'
    wait_bg
    ;;
  base-b-refs)
    setup_test_binary ./internal/runtime
    trap cleanup_test_binary EXIT
    start_bg "runtime-runner-refs" \
      run_test_binary '^(TestRunner_StepRetryExecution|TestRunner_StepIDAccess|TestRunner_EventHandlerStepIDAccess|TestRunnerPartialSuccess)$'
    start_bg "runtime-runner-chat" \
      run_test_binary '^(TestRunner_ChatMessagesHandler|TestSetupPushBackConversation)$'
    start_bg "runtime-runner-waitstep" \
      run_test_binary '^TestWaitStep$'
    wait_bg
    ;;
  base-b-refs-chatwait)
    "$0" base-b-agent
    "$0" base-b-refs
    ;;
  base-a)
    "$0" base-a-early-rest
    "$0" base-a-retry-output
    ;;
  base-b)
    "$0" base-b-policies-advanced
    "$0" base-b-refs-chatwait
    ;;
  all)
    "$0" base-a
    "$0" base-b
    ;;
  *)
    echo "unknown mode: $mode" >&2
    exit 1
    ;;
esac
