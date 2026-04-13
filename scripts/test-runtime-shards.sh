#!/usr/bin/env bash

set -euo pipefail

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
    start_bg "runtime-rest" \
      ./scripts/test-shard-split.sh ./internal/runtime 4 \
      '' \
      '^(TestRunner$|TestRunner_ErrorHandling$|TestRunner_DAGPreconditions$|TestRunner_StatusDefersForcedStatusUntilTerminal$|TestRunner_SignalHandling$|TestRunner_ComplexDependencyChains$|TestRunner_EdgeCases$|TestRunner_ComplexRetryScenarios$|TestRunner_StepRetryExecution$|TestRunner_StepIDAccess$|TestRunner_EventHandlerStepIDAccess$|TestRunnerPartialSuccess$|TestRunner_ChatMessagesHandler$|TestSetupPushBackConversation$|TestWaitStep$)$'
    start_bg "runtime-runner-early" \
      go test -v -race ./internal/runtime \
      -run '^TestRunner$/(SequentialStepsSuccess|SequentialStepsWithFailure|ParallelSteps|ParallelStepsWithFailure|ComplexCommand|ContinueOnFailure|ContinueOnSkip|ContinueOnExitCode|ContinueOnOutputStdout|ContinueOnOutputStderr|ContinueOnOutputRegexp|ContinueOnMarkSuccess|Cancel|Timeout)$'
    wait_bg
    ;;
  base-a-retry-output)
    start_bg "runtime-runner-retry-repeat" \
      go test -v -race ./internal/runtime \
      -run '^TestRunner$/(RetryPolicyFail|RetryWithScript|RetryPolicySuccess|PreconditionMatch|PreconditionNotMatch|PreconditionWithCommandMet|PreconditionWithCommandNotMet|OnExitHandler|OnExitHandlerFail|OnAbortHandler|OnSuccessHandler|OnFailureHandler|CancelOnSignal|Repeat|RepeatFail|StopRepetitiveTaskGracefully|WorkingDirNoExist)$'
    start_bg "runtime-runner-output-specialvars" \
      go test -v -race ./internal/runtime \
      -run '^TestRunner$/(OutputVariables|OutputInheritance|OutputJSONReference|HandlingJSONWithSpecialChars|SpecialVarsDAGRUNLOGFILE|SpecialVarsDAGRUNSTEPSTDOUTFILE|SpecialVarsDAGRUNSTEPSTDERRFILE|SpecialVarsDAGRUNID|SpecialVarsDAGNAME|SpecialVarsDAGRUNSTEPNAME|StdoutPathExpandsStepNameBeforePrepare|StdoutPathExpandsStepEnvBeforePrepare|StdoutPathExpandsUpstreamStepRefBeforePrepare|DAGRunStatusNotAvailableToMainSteps)$'
    wait_bg
    ;;
  base-b-policies-advanced)
    start_bg "runtime-runner-repeat-policies" \
      go test -v -race ./internal/runtime \
      -run '^TestRunner$/(RepeatPolicyRepeatsUntilCommandConditionMatchesExpected|RepeatPolicyRepeatWhileConditionExits0|RepeatPolicyRepeatsWhileCommandExitCodeMatches|RepeatPolicyRepeatsUntilFileConditionMatchesExpected|RepeatPolicyRepeatsUntilOutputVarConditionMatchesExpected|RetryPolicyWithOutputCapture|FailedStepWithOutputCapture|RetryPolicySubDAGRunWithOutputCapture|SingleStepTimeoutFailsStep|TimeoutPreemptsRetriesAndMarksFailed|ParallelStepsTimeoutFailIndividually|StepLevelTimeoutOverridesLongDAGTimeoutAndFails|RejectedTakesPrecedenceOverWaiting)$'
    start_bg "runtime-runner-advanced-parents" \
      go test -v -race ./internal/runtime \
      -run '^(TestRunner_ErrorHandling|TestRunner_DAGPreconditions|TestRunner_StatusDefersForcedStatusUntilTerminal|TestRunner_SignalHandling|TestRunner_ComplexDependencyChains|TestRunner_EdgeCases)$'
    start_bg "runtime-runner-complex-retry" \
      go test -v -race ./internal/runtime \
      -run '^TestRunner_ComplexRetryScenarios$'
    wait_bg
    ;;
  base-b-refs-chatwait)
    start_bg "runtime-runner-refs" \
      go test -v -race ./internal/runtime \
      -run '^(TestRunner_StepRetryExecution|TestRunner_StepIDAccess|TestRunner_EventHandlerStepIDAccess|TestRunnerPartialSuccess)$'
    start_bg "runtime-runner-chat-wait" \
      go test -v -race ./internal/runtime \
      -run '^(TestRunner_ChatMessagesHandler|TestSetupPushBackConversation|TestWaitStep)$'
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
