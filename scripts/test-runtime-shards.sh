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

case "$mode" in
  base-a-early-rest)
    setup_test_binary ./internal/runtime
    trap cleanup_test_binary EXIT
    start_bg "runtime-rest" \
      run_sharded_tests 6 \
      '' \
      '^(TestRunner$|TestRunner_ErrorHandling$|TestRunner_DAGPreconditions$|TestRunner_StatusDefersForcedStatusUntilTerminal$|TestRunner_SignalHandling$|TestRunner_ComplexDependencyChains$|TestRunner_EdgeCases$|TestRunner_ComplexRetryScenarios$|TestRunner_StepRetryExecution$|TestRunner_StepIDAccess$|TestRunner_EventHandlerStepIDAccess$|TestRunnerPartialSuccess$|TestRunner_ChatMessagesHandler$|TestSetupPushBackConversation$|TestWaitStep$)$'
    start_bg "runtime-runner-early" \
      run_test_binary '^TestRunner$/(SequentialStepsSuccess|SequentialStepsWithFailure|ParallelSteps|ParallelStepsWithFailure|ComplexCommand|ContinueOnFailure|ContinueOnSkip|ContinueOnExitCode|ContinueOnOutputStdout|ContinueOnOutputStderr|ContinueOnOutputRegexp|ContinueOnMarkSuccess|Cancel|Timeout)$'
    wait_bg
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
    start_bg "runtime-runner-repeat-policies" \
      run_test_binary '^TestRunner$/(RepeatPolicyRepeatsUntilCommandConditionMatchesExpected|RepeatPolicyRepeatWhileConditionExits0|RepeatPolicyRepeatsWhileCommandExitCodeMatches|RepeatPolicyRepeatsUntilFileConditionMatchesExpected|RepeatPolicyRepeatsUntilOutputVarConditionMatchesExpected|RetryPolicyWithOutputCapture|FailedStepWithOutputCapture|RetryPolicySubDAGRunWithOutputCapture|SingleStepTimeoutFailsStep|TimeoutPreemptsRetriesAndMarksFailed|ParallelStepsTimeoutFailIndividually|StepLevelTimeoutOverridesLongDAGTimeoutAndFails|RejectedTakesPrecedenceOverWaiting)$'
    start_bg "runtime-runner-advanced-parents" \
      run_test_binary '^(TestRunner_ErrorHandling|TestRunner_DAGPreconditions|TestRunner_StatusDefersForcedStatusUntilTerminal|TestRunner_SignalHandling|TestRunner_ComplexDependencyChains|TestRunner_EdgeCases)$'
    start_bg "runtime-runner-complex-retry" \
      run_test_binary '^TestRunner_ComplexRetryScenarios$'
    wait_bg
    ;;
  base-b-refs-chatwait)
    setup_test_binary ./internal/runtime
    trap cleanup_test_binary EXIT
    start_bg "runtime-agent-subpackage" \
      ./scripts/test-package-shard.sh \
      '^github.com/dagucloud/dagu/internal/runtime/agent$'
    start_bg "runtime-runner-refs" \
      run_test_binary '^(TestRunner_StepRetryExecution|TestRunner_StepIDAccess|TestRunner_EventHandlerStepIDAccess|TestRunnerPartialSuccess)$'
    start_bg "runtime-runner-chat" \
      run_test_binary '^(TestRunner_ChatMessagesHandler|TestSetupPushBackConversation)$'
    start_bg "runtime-runner-waitstep" \
      run_test_binary '^TestWaitStep$'
    wait_bg
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
