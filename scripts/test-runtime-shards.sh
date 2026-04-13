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

  pids=()
  names=()
  return "$status"
}

start_bg "runtime-rest" \
  ./scripts/test-shard.sh ./internal/runtime \
  '' \
  '^TestRunner$'
start_bg "runtime-runner-early" \
  go test -v -race ./internal/runtime \
  -run '^TestRunner/(SequentialStepsSuccess|SequentialStepsWithFailure|ParallelSteps|ParallelStepsWithFailure|ComplexCommand|ContinueOnFailure|ContinueOnSkip|ContinueOnExitCode|ContinueOnOutputStdout|ContinueOnOutputStderr|ContinueOnOutputRegexp|ContinueOnMarkSuccess|Cancel|Timeout)$'
start_bg "runtime-runner-retry-repeat" \
  go test -v -race ./internal/runtime \
  -run '^TestRunner/(RetryPolicyFail|RetryWithScript|RetryPolicySuccess|PreconditionMatch|PreconditionNotMatch|PreconditionWithCommandMet|PreconditionWithCommandNotMet|OnExitHandler|OnExitHandlerFail|OnAbortHandler|OnSuccessHandler|OnFailureHandler|CancelOnSignal|Repeat|RepeatFail|StopRepetitiveTaskGracefully|WorkingDirNoExist)$'
wait_bg

start_bg "runtime-runner-output-specialvars" \
  go test -v -race ./internal/runtime \
  -run '^TestRunner/(OutputVariables|OutputInheritance|OutputJSONReference|HandlingJSONWithSpecialChars|SpecialVarsDAGRUNLOGFILE|SpecialVarsDAGRUNSTEPSTDOUTFILE|SpecialVarsDAGRUNSTEPSTDERRFILE|SpecialVarsDAGRUNID|SpecialVarsDAGNAME|SpecialVarsDAGRUNSTEPNAME|StdoutPathExpandsStepNameBeforePrepare|StdoutPathExpandsStepEnvBeforePrepare|StdoutPathExpandsUpstreamStepRefBeforePrepare|DAGRunStatusNotAvailableToMainSteps)$'
start_bg "runtime-runner-repeat-policies" \
  go test -v -race ./internal/runtime \
  -run '^TestRunner/(RepeatPolicyRepeatsUntilCommandConditionMatchesExpected|RepeatPolicyRepeatWhileConditionExits0|RepeatPolicyRepeatsWhileCommandExitCodeMatches|RepeatPolicyRepeatsUntilFileConditionMatchesExpected|RepeatPolicyRepeatsUntilOutputVarConditionMatchesExpected|RetryPolicyWithOutputCapture|FailedStepWithOutputCapture|RetryPolicySubDAGRunWithOutputCapture|SingleStepTimeoutFailsStep|TimeoutPreemptsRetriesAndMarksFailed|ParallelStepsTimeoutFailIndividually|StepLevelTimeoutOverridesLongDAGTimeoutAndFails|RejectedTakesPrecedenceOverWaiting)$'
start_bg "runtime-runner-advanced" \
  go test -v -race ./internal/runtime \
  -run '^TestRunner/(SetupError|PanicRecovery|DAGPreconditionNotMet|RunningStatusWinsBeforeForcedTerminalStatus|SignalWithDoneChannel|SignalWithOverride|DiamondDependency|ComplexFailurePropagation|EmptyPlan|SingleNodePlan|AllNodesFail|RetryWithSignalTermination|RetryWithSpecificExitCodes|RepeatPolicyBooleanTrueRepeatsWhileStepSucceeds|RepeatPolicyBooleanTrueWithFailureStopsOnFailure|RepeatPolicyUntilModeWithoutConditionRepeatsOnFailure|RepeatPolicyWhileWithConditionRepeatsWhileConditionSucceeds|RepeatPolicyWhileWithConditionAndExpectedRepeatsWhileMatches|RepeatPolicyUntilWithConditionRepeatsUntilConditionSucceeds|RepeatPolicyUntilWithConditionAndExpectedRepeatsUntilMatches|RepeatPolicyUntilWithExitCodeRepeatsUntilExitCodeMatches|RepeatPolicyLimit|RepeatPolicyOutputVariablesReloadedBeforeConditionEval)$'
wait_bg

start_bg "runtime-runner-refs" \
  go test -v -race ./internal/runtime \
  -run '^TestRunner/(RetrySuccessfulStep|StepReferenceInCommand|StepWithoutID|StepExitCodeReference|OnSuccessHandlerWithStepReferences|OnFailureHandlerWithStepReferences|OnExitHandlerWithMultipleStepReferences|HandlerWithoutIDCannotBeReferenced|HandlersCanOnlyReferenceMainSteps|NodeStatusPartialSuccess|NodeStatusPartialSuccessWithMarkSuccess|MultipleFailuresWithContinueOn|NoSuccessfulStepsWithContinueOn|FailureWithoutContinueOn)$'
start_bg "runtime-runner-chat-wait" \
  go test -v -race ./internal/runtime \
  -run '^TestRunner/(HandlerNotCalledForNonChatSteps|HandlerConfiguredCorrectly|SetupChatMessagesNoDependencies|SetupChatMessagesWithDependencies|SetupChatMessagesReadError|SetupChatMessagesDeduplicatesSystem|SaveChatMessagesOnSuccess|SaveChatMessagesWriteError|SaveChatMessagesWithInheritedContext|SaveChatMessagesNoMessages|AgentStepSavesMessages|AgentStepInheritsFromDependency|HandlerNotCalledForAgentStepWithNoMessages|LoadsOwnMessagesForPushedBackAgentStep|NoOpForFirstExecution|NoOpForNonAgentStep|NoOpWithoutApproval|GracefulOnReadError|WaitStepResultsInWaitStatus|WaitStepBlocksDependentNodes|ParallelBranchWithWaitStep|WaitStepAtStart|WaitStepWithInputConfig|MultipleWaitSteps)$'
wait_bg
