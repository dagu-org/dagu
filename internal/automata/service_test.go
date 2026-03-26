// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedag"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/stretchr/testify/require"
)

func init() {
	core.RegisterExecutorCapabilities("command", core.ExecutorCapabilities{
		Command:          true,
		MultipleCommands: true,
		Script:           true,
		Shell:            true,
	})
}

func TestServiceListInitializesStateAndStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	require.Equal(t, "software-dev", item.Name)
	require.Equal(t, StateIdle, item.State)
	require.Equal(t, "research", item.Stage)
	require.Equal(t, fixedTime, item.LastUpdatedAt)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.NotNil(t, detail.State)
	require.Equal(t, StateIdle, detail.State.State)
	require.Equal(t, "research", detail.State.CurrentStage)
	require.Equal(t, "system", detail.State.StageChangedBy)
	require.Len(t, detail.AllowedDAGs, 1)
	require.Equal(t, "build-app", detail.AllowedDAGs[0].Name)
}

func TestServiceDetailUsesCurrentStageAllowedDAGs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpecPerStage()))

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, "research", detail.State.CurrentStage)
	require.Equal(t, []string{"build-app"}, allowedDAGNames(detail.AllowedDAGs))

	require.NoError(t, svc.OverrideStage(ctx, "software-dev", StageOverrideRequest{
		Stage:       "implement",
		RequestedBy: "tester",
	}))

	detail, err = svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, "implement", detail.State.CurrentStage)
	require.Equal(t, []string{"run-tests"}, allowedDAGNames(detail.AllowedDAGs))
}

func TestServiceOverrideStagePersistsDeclaredStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	err := svc.OverrideStage(ctx, "software-dev", StageOverrideRequest{
		Stage:       "implement",
		RequestedBy: "tester",
		Note:        "planning complete",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.NotNil(t, detail.State)
	require.Equal(t, "implement", detail.State.CurrentStage)
	require.Equal(t, "tester", detail.State.StageChangedBy)
	require.Equal(t, "planning complete", detail.State.StageNote)
	require.Equal(t, fixedTime, detail.State.StageChangedAt)
}

func TestServiceOverrideStageRejectsUnknownStage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	err := svc.OverrideStage(ctx, "software-dev", StageOverrideRequest{Stage: "deploy"})
	require.Error(t, err)
	require.ErrorContains(t, err, `unknown stage "deploy"`)
}

func TestControllerRuntimeSetStageRequestsApproval(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpecPerStage()))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	err = rt.SetStage(ctx, "implement", "planning is complete")
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, "research", detail.State.CurrentStage)
	require.Equal(t, StateWaiting, detail.State.State)
	require.Equal(t, WaitingReasonHuman, detail.State.WaitingReason)
	require.NotNil(t, detail.State.PendingPrompt)
	require.NotNil(t, detail.State.PendingStageTransition)
	require.Equal(t, "implement", detail.State.PendingStageTransition.RequestedStage)
	require.Equal(t, "planning is complete", detail.State.PendingStageTransition.Note)
	require.Equal(t, "agent", detail.State.PendingStageTransition.RequestedBy)
	require.Equal(t, fixedTime, detail.State.PendingStageTransition.CreatedAt)
	require.Len(t, detail.State.PendingPrompt.Options, 2)
}

func TestServiceSubmitHumanResponseApprovesStageTransition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpecPerStage()))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.SetStage(ctx, "implement", "ready to code"))

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.NotNil(t, detail.State.PendingPrompt)

	err = svc.SubmitHumanResponse(ctx, "software-dev", HumanResponseRequest{
		PromptID:          detail.State.PendingPrompt.ID,
		SelectedOptionIDs: []string{stageApprovalOptionApprove},
	})
	require.NoError(t, err)

	detail, err = svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, "implement", detail.State.CurrentStage)
	require.Equal(t, "agent (approved)", detail.State.StageChangedBy)
	require.Equal(t, "ready to code", detail.State.StageNote)
	require.Equal(t, fixedTime, detail.State.StageChangedAt)
	require.Equal(t, StateRunning, detail.State.State)
	require.Nil(t, detail.State.PendingPrompt)
	require.Nil(t, detail.State.PendingStageTransition)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "stage_transition_approved", detail.State.PendingTurnMessages[0].Kind)
	require.Equal(t, []string{"run-tests"}, allowedDAGNames(detail.AllowedDAGs))
}

func TestServiceSubmitHumanResponseRejectsStageTransition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpecPerStage()))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.SetStage(ctx, "implement", "ready to code"))

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.NotNil(t, detail.State.PendingPrompt)

	err = svc.SubmitHumanResponse(ctx, "software-dev", HumanResponseRequest{
		PromptID:          detail.State.PendingPrompt.ID,
		SelectedOptionIDs: []string{stageApprovalOptionReject},
	})
	require.NoError(t, err)

	detail, err = svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, "research", detail.State.CurrentStage)
	require.Equal(t, StateRunning, detail.State.State)
	require.Nil(t, detail.State.PendingPrompt)
	require.Nil(t, detail.State.PendingStageTransition)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "stage_transition_rejected", detail.State.PendingTurnMessages[0].Kind)
	require.Equal(t, []string{"build-app"}, allowedDAGNames(detail.AllowedDAGs))
}

func TestServiceOverrideStageClearsPendingStageTransition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpecPerStage()))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.SetStage(ctx, "implement", "ready to code"))

	err = svc.OverrideStage(ctx, "software-dev", StageOverrideRequest{
		Stage:       "plan",
		RequestedBy: "tester",
		Note:        "manual correction",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, "plan", detail.State.CurrentStage)
	require.Equal(t, "tester", detail.State.StageChangedBy)
	require.Equal(t, "manual correction", detail.State.StageNote)
	require.Equal(t, fixedTime, detail.State.StageChangedAt)
	require.Nil(t, detail.State.PendingPrompt)
	require.Nil(t, detail.State.PendingStageTransition)
	require.Equal(t, StateRunning, detail.State.State)
}

func TestServiceRequestStartRequiresInstruction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	err := svc.RequestStart(ctx, "software-dev", StartRequest{RequestedBy: "tester"})
	require.Error(t, err)
	require.ErrorContains(t, err, "instruction is required before starting automata")

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StateIdle, detail.State.State)
	require.Empty(t, detail.State.Instruction)
}

func TestServiceRequestStartStoresInstruction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	err := svc.RequestStart(ctx, "software-dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Fix the failing integration test and open a review-ready change.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, "Fix the failing integration test and open a review-ready change.", detail.State.Instruction)
	require.Equal(t, "tester", detail.State.InstructionUpdatedBy)
	require.Equal(t, fixedTime, detail.State.InstructionUpdatedAt)
	require.False(t, detail.State.StartRequestedAt.IsZero())
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "kickoff", detail.State.PendingTurnMessages[0].Kind)
}

func TestServiceRequestStartRejectsActiveTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	require.NoError(t, svc.RequestStart(ctx, "software-dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))

	err := svc.RequestStart(ctx, "software-dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Start a second task.",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "already has an active task")
}

func TestServiceSubmitOperatorMessageRequiresActiveTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))

	err := svc.SubmitOperatorMessage(ctx, "software-dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "Please prioritize the flaky test first.",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "not running an active task")
}

func TestServiceSubmitOperatorMessageQueuesMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	require.NoError(t, svc.RequestStart(ctx, "software-dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))

	err := svc.SubmitOperatorMessage(ctx, "software-dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "Focus on the regression first.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Len(t, detail.State.PendingTurnMessages, 2)
	require.Equal(t, "operator_message", detail.State.PendingTurnMessages[1].Kind)
	require.Equal(t, fixedTime, detail.State.PendingTurnMessages[1].CreatedAt)
	require.Contains(t, detail.State.PendingTurnMessages[1].Message, "Focus on the regression first.")
}

func TestServicePauseAndResumeTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	require.NoError(t, svc.RequestStart(ctx, "software-dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))

	err := svc.Pause(ctx, "software-dev", "tester")
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StatePaused, detail.State.State)
	require.Equal(t, "tester", detail.State.PausedBy)
	require.Equal(t, fixedTime, detail.State.PausedAt)
	require.Len(t, detail.State.PendingTurnMessages, 1)

	err = svc.Resume(ctx, "software-dev", "tester")
	require.NoError(t, err)

	detail, err = svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Empty(t, detail.State.PausedBy)
	require.True(t, detail.State.PausedAt.IsZero())
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "kickoff", detail.State.PendingTurnMessages[0].Kind)
}

func TestServiceResumePausedAutomataQueuesResumeMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	require.NoError(t, svc.RequestStart(ctx, "software-dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))

	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.PendingTurnMessages = nil
	state.State = StatePaused
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.Resume(ctx, "software-dev", "tester")
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "resume", detail.State.PendingTurnMessages[0].Kind)
}

func TestServiceResumePausedPromptReturnsWaiting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StatePaused
	state.PendingPrompt = &Prompt{
		ID:        "prompt-1",
		Question:  "Need approval?",
		CreatedAt: time.Now(),
	}
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.Resume(ctx, "software-dev", "tester")
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StateWaiting, detail.State.State)
	require.Equal(t, WaitingReasonHuman, detail.State.WaitingReason)
}

func TestServiceSubmitHumanResponseWhilePausedQueuesWithoutResuming(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StatePaused
	state.PendingPrompt = &Prompt{
		ID:        "prompt-1",
		Question:  "Need approval?",
		CreatedAt: time.Now(),
	}
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.SubmitHumanResponse(ctx, "software-dev", HumanResponseRequest{
		PromptID:         "prompt-1",
		FreeTextResponse: "approved",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StatePaused, detail.State.State)
	require.Nil(t, detail.State.PendingPrompt)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "human_response", detail.State.PendingTurnMessages[0].Kind)
}

func TestServiceSubmitOperatorMessageWhilePausedKeepsPaused(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	require.NoError(t, svc.RequestStart(ctx, "software-dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))
	require.NoError(t, svc.Pause(ctx, "software-dev", "tester"))

	err := svc.SubmitOperatorMessage(ctx, "software-dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "Resume with the flaky test first.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StatePaused, detail.State.State)
	require.Len(t, detail.State.PendingTurnMessages, 2)
	require.Equal(t, "operator_message", detail.State.PendingTurnMessages[1].Kind)
}

func TestServiceReconcileOnceDoesNotWakeIdleScheduledAutomata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpecWithSchedule("build-app", "* * * * *")))

	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.Instruction = "Keep shipping queued work."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	require.NoError(t, svc.ReconcileOnce(ctx))

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StateIdle, detail.State.State)
	require.Equal(t, "Keep shipping queued work.", detail.State.Instruction)
	require.Empty(t, detail.State.PendingTurnMessages)
}

func TestServiceReconcileOnceDoesNotWakePausedAutomata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StatePaused
	state.Instruction = "Handle the current assigned task."
	queueTurnMessage(state, "resume", "resume now", time.Now())
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	require.NoError(t, svc.ReconcileOnce(ctx))

	detail, err := svc.Detail(ctx, "software-dev")
	require.NoError(t, err)
	require.Equal(t, StatePaused, detail.State.State)
	require.Len(t, detail.State.PendingTurnMessages, 1)
}

func TestControllerRuntimeRunAllowedDAGRejectsConcurrentRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	ref := exec.NewDAGRunRef("build-app", "run-1")
	state.CurrentRunRef = &ref

	rt := &controllerRuntime{service: svc, def: def, state: state}
	_, err = rt.RunAllowedDAG(ctx, agent.AutomataRunDAGInput{DAGName: "build-app"})
	require.Error(t, err)
	require.ErrorContains(t, err, "already active")
}

func TestControllerRuntimeFinishRejectsActiveChildRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software-dev", automataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software-dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	ref := exec.NewDAGRunRef("build-app", "run-1")
	state.CurrentRunRef = &ref

	rt := &controllerRuntime{service: svc, def: def, state: state}
	err = rt.Finish(ctx, "done")
	require.Error(t, err)
	require.ErrorContains(t, err, "while a child DAG run is active")
}

func newTestService(t *testing.T) (*Service, time.Time) {
	t.Helper()

	root := t.TempDir()
	dagsDir := filepath.Join(root, "dags")
	dataDir := filepath.Join(root, "data")
	runsDir := filepath.Join(root, "runs")

	require.NoError(t, os.MkdirAll(dagsDir, 0o750))
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	require.NoError(t, os.MkdirAll(runsDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(dagsDir, "build-app.yaml"),
		[]byte(testDAGYAML("build-app")),
		0o600,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(dagsDir, "run-tests.yaml"),
		[]byte(testDAGYAML("run-tests")),
		0o600,
	))

	cfg := &config.Config{
		Core: config.Core{
			Location: time.UTC,
		},
		Paths: config.PathsConfig{
			DAGsDir:    dagsDir,
			DataDir:    dataDir,
			DAGRunsDir: runsDir,
		},
	}
	fixedTime := time.Date(2026, time.March, 26, 10, 0, 0, 0, time.UTC)
	svc := New(
		cfg,
		filedag.New(dagsDir, filedag.WithSkipExamples(true)),
		filedagrun.New(runsDir),
		WithClock(func() time.Time { return fixedTime }),
	)
	return svc, fixedTime
}

func automataSpec(allowedDAG string) string {
	return `description: Software development automata
purpose: Ship one development task
goal: Complete the assigned software work
stages:
  - research
  - plan
  - implement
allowedDAGs:
  names:
    - ` + allowedDAG + `
`
}

func automataSpecWithSchedule(allowedDAG, schedule string) string {
	return `description: Software development automata
purpose: Ship one development task
goal: Complete the assigned software work
schedule: "` + schedule + `"
stages:
  - research
  - plan
  - implement
allowedDAGs:
  names:
    - ` + allowedDAG + `
`
}

func automataSpecPerStage() string {
	return `description: Software development automata
purpose: Ship one development task
goal: Complete the assigned software work
stages:
  - name: research
    allowedDAGs:
      names:
        - build-app
  - name: plan
  - name: implement
    allowedDAGs:
      names:
        - run-tests
`
}

func allowedDAGNames(items []AllowedDAGInfo) []string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

func testDAGYAML(name string) string {
	return `name: ` + name + `
description: Example DAG
tags:
  - dev
steps:
  - name: echo
    command: echo hello
`
}
