// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	_ "github.com/dagucloud/dagu/internal/llm/allproviders"
	"github.com/dagucloud/dagu/internal/runtime"
	"github.com/dagucloud/dagu/internal/service/eventstore"
	"github.com/stretchr/testify/require"
)

type testCoordinatorCanceler struct{}

func (*testCoordinatorCanceler) RequestCancel(context.Context, string, string, *exec.DAGRunRef) error {
	return nil
}

type testAgentConfigStore struct {
	cfg *agent.Config
}

func (s *testAgentConfigStore) Load(context.Context) (*agent.Config, error) {
	return s.cfg, nil
}

func (s *testAgentConfigStore) Save(_ context.Context, cfg *agent.Config) error {
	s.cfg = cfg
	return nil
}

func (s *testAgentConfigStore) IsEnabled(context.Context) bool {
	return s.cfg != nil && s.cfg.Enabled
}

type testAgentModelStore struct {
	models map[string]*agent.ModelConfig
}

func (s *testAgentModelStore) Create(_ context.Context, model *agent.ModelConfig) error {
	if s.models == nil {
		s.models = map[string]*agent.ModelConfig{}
	}
	s.models[model.ID] = model
	return nil
}

func (s *testAgentModelStore) GetByID(_ context.Context, id string) (*agent.ModelConfig, error) {
	model, ok := s.models[id]
	if !ok {
		return nil, agent.ErrModelNotFound
	}
	return model, nil
}

func (s *testAgentModelStore) List(context.Context) ([]*agent.ModelConfig, error) {
	out := make([]*agent.ModelConfig, 0, len(s.models))
	for _, model := range s.models {
		out = append(out, model)
	}
	return out, nil
}

func (s *testAgentModelStore) Update(_ context.Context, model *agent.ModelConfig) error {
	if s.models == nil {
		s.models = map[string]*agent.ModelConfig{}
	}
	s.models[model.ID] = model
	return nil
}

func (s *testAgentModelStore) Delete(_ context.Context, id string) error {
	delete(s.models, id)
	return nil
}

func TestServiceListInitializesStateAndTaskSummary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	require.Equal(t, "software_dev", item.Name)
	require.Equal(t, StateIdle, item.State)
	require.Equal(t, 0, item.OpenTaskCount)
	require.Equal(t, 0, item.DoneTaskCount)
	require.Empty(t, item.NextTaskDescription)
	require.Equal(t, fixedTime, item.LastUpdatedAt)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateIdle, detail.State.State)
	require.Empty(t, detail.State.Tasks)
	require.Equal(t, []string{"build-app"}, workflowNames(detail.Workflows))
	require.Equal(t, filepath.Join(svc.stateDir, "artifacts", "software_dev"), detail.ArtifactDir)
	require.False(t, detail.ArtifactsAvailable)
	require.DirExists(t, detail.ArtifactDir)
}

func TestServiceDetailUsesTopLevelWorkflows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpecMultiDAGs()))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, []string{"build-app", "run-tests"}, workflowNames(detail.Workflows))
}

func TestServicePutSpecAcceptsLegacyPurposeAsGoalAlias(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Legacy controller
trigger:
  type: manual
purpose: Complete the assigned software work
workflows:
  names:
    - build-app
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, "Complete the assigned software work", detail.Definition.Goal)
}

func TestServicePutSpecAcceptsMissingGoal(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Goal-free controller
trigger:
  type: manual
workflows:
  names:
    - build-app
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Empty(t, detail.Definition.Goal)
	require.Equal(t, "Goal-free controller", detail.Definition.Description)
}

func TestServicePutSpecAcceptsMissingWorkflows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Workflow-flexible controller
trigger:
  type: manual
goal: Complete the assigned software work
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Empty(t, detail.Definition.Workflows.Names)
	require.Empty(t, detail.Definition.Workflows.Labels)
}

func TestServicePutSpecRejectsMissingTrigger(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Goal-free controller
workflows:
  names:
    - build-app
`
	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, `trigger`)
}

func TestServicePutSpecRejectsCronTriggerWithoutSchedules(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: cron
workflows:
  names:
    - build-app
`
	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, `schedules`)
}

func TestServicePutSpecRejectsCronTriggerWithoutPrompt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: cron
  schedules:
    - "* * * * *"
workflows:
  names:
    - build-app
`
	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, `prompt`)
}

func TestServicePutSpecRejectsManualTriggerWithPrompt(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: manual
  prompt: "Handle the current task."
workflows:
  names:
    - build-app
`
	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, `trigger.prompt`)
}

func TestServicePutSpecRejectsStagesField(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: manual
goal: Complete the assigned software work
stages:
  - research
workflows:
  names:
    - build-app
`

	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, `unknown field "stages"`)
}

func TestServicePutSpecRejectsDotsAndHyphensInControllerName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	err := svc.PutSpec(ctx, "software.dev", controllerSpec("build-app"))
	require.ErrorContains(t, err, `invalid controller name "software.dev"`)

	err = svc.PutSpec(ctx, "software-dev", controllerSpec("build-app"))
	require.ErrorContains(t, err, `invalid controller name "software-dev"`)
}

func TestServicePutSpecNormalizesLabelsAndExposesThemInSummary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: manual
goal: Complete the assigned software work
labels:
  - workspace=Engineering
  - Owner=Team-AI
workflows:
  names:
    - build-app
`

	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, []string{"workspace=engineering", "owner=team-ai"}, detail.Definition.Labels)

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []string{"workspace=engineering", "owner=team-ai"}, items[0].Labels)
}

func TestServicePutSpecRejectsInvalidLabels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: manual
goal: Complete the assigned software work
labels:
  - "bad tag"
workflows:
  names:
    - build-app
`

	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid labels")
}

func TestServicePutSpecExposesNicknameAndIconURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: manual
nickname: Build Captain
icon_url: https://cdn.example.com/controller/build-captain.png
goal: Complete the assigned software work
workflows:
  names:
    - build-app
`

	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, "Build Captain", detail.Definition.Nickname)
	require.Equal(t, "https://cdn.example.com/controller/build-captain.png", detail.Definition.IconURL)

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Build Captain", items[0].Nickname)
	require.Equal(t, "https://cdn.example.com/controller/build-captain.png", items[0].IconURL)
}

func TestServicePutSpecResetsRuntimeWhenAgentConfigChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpecWithModel("build-app", "model-a")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StateRunning
	state.Instruction = "Handle the current assigned task."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	state.SessionID = "sess-1"
	state.LastError = "old error"
	queueTurnMessage(state, "operator_message", "Operator update from tester:\nUse the new model.", fixedTime)
	require.NoError(t, svc.sessionStore.CreateSession(ctx, &agent.Session{
		ID:        state.SessionID,
		UserID:    svc.systemUser(def.Name).UserID,
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
	}))
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpecWithModel("build-app", "model-b")))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Empty(t, detail.State.SessionID)
	require.Empty(t, detail.State.LastError)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Contains(t, detail.State.PendingTurnMessages[0].Message, "Use the new model.")
	_, err = svc.sessionStore.GetSession(ctx, "sess-1")
	require.ErrorIs(t, err, agent.ErrSessionNotFound)
}

func TestServicePutSpecRejectsInvalidIconURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: manual
goal: Complete the assigned software work
icon_url: javascript:alert(1)
workflows:
  names:
    - build-app
`

	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, "icon_url must use http or https")
}

func TestServicePutSpecRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	err := svc.PutSpec(ctx, "software_dev", `trigger:
  type: manual
goal: Complete the assigned software work
bogus: true
workflows:
  names:
    - build-app
`)
	require.ErrorContains(t, err, `unknown field "bogus"`)

	err = svc.PutSpec(ctx, "software_dev", `trigger:
  type: manual
goal: Complete the assigned software work
workflows:
  names:
    - build-app
  bogus:
    - nope
`)
	require.ErrorContains(t, err, `unknown field "bogus"`)

	err = svc.PutSpec(ctx, "software_dev", `trigger:
  type: manual
goal: Complete the assigned software work
workflows:
  names:
    - build-app
agent:
  safeMode: true
  bogus: true
`)
	require.ErrorContains(t, err, `unknown field "bogus"`)
}

func TestServiceTaskCRUDAndReorder(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))

	first := createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")
	second := createTask(t, svc, ctx, "software_dev", "Ship the fix", "tester")

	updated, err := svc.UpdateTask(ctx, "software_dev", first.ID, UpdateTaskRequest{
		Description: new("Investigate and reproduce the failing test"),
		RequestedBy: "tester",
	})
	require.NoError(t, err)
	require.Equal(t, TaskStateOpen, updated.State)
	require.Equal(t, fixedTime, updated.UpdatedAt)

	require.NoError(t, svc.ReorderTasks(ctx, "software_dev", ReorderTasksRequest{
		TaskIDs:     []string{second.ID, first.ID},
		RequestedBy: "tester",
	}))
	require.NoError(t, svc.DeleteTask(ctx, "software_dev", second.ID, "tester"))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Empty(t, detail.State.Tasks)
	require.Len(t, detail.TaskTemplates, 1)
	require.Equal(t, first.ID, detail.TaskTemplates[0].ID)
	require.Equal(t, "Investigate and reproduce the failing test", detail.TaskTemplates[0].Description)
}

func TestServiceTaskOperationsValidateInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	task := createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	_, err := svc.UpdateTask(ctx, "software_dev", task.ID, UpdateTaskRequest{RequestedBy: "tester"})
	require.ErrorContains(t, err, "no task changes requested")

	err = svc.ReorderTasks(ctx, "software_dev", ReorderTasksRequest{
		TaskIDs:     []string{task.ID, task.ID},
		RequestedBy: "tester",
	})
	require.ErrorContains(t, err, "every task exactly once")

	err = svc.DeleteTask(ctx, "software_dev", "missing", "tester")
	require.ErrorContains(t, err, `unknown task "missing"`)
}

func TestServiceRequestStartRequiresInstructionAndOpenTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))

	err := svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	})
	require.ErrorContains(t, err, "at least one task template is required")

	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.Instruction = "Reuse the previous instruction."
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.RequestStart(ctx, "software_dev", StartRequest{RequestedBy: "tester"})
	require.ErrorContains(t, err, "instruction is required before starting controller")
}

func TestServiceRequestStartStoresInstruction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	err := svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Fix the failing integration test and open a review-ready change.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, "Fix the failing integration test and open a review-ready change.", detail.State.Instruction)
	require.Equal(t, "tester", detail.State.InstructionUpdatedBy)
	require.Equal(t, fixedTime, detail.State.InstructionUpdatedAt)
	require.False(t, detail.State.StartRequestedAt.IsZero())
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "kickoff", detail.State.PendingTurnMessages[0].Kind)
}

func TestServiceRequestStartRejectsWhenAllTasksDone(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	task := createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.Tasks = cloneTasksFromTemplates(state.TaskTemplates, state.LastUpdatedAt)
	state.CurrentCycleID = nextCycleID()
	require.NoError(t, svc.saveState(ctx, def.Name, state))
	_, err = svc.SetTaskDone(ctx, "software_dev", task.ID, true, "tester")
	require.NoError(t, err)

	err = svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Len(t, detail.State.Tasks, 1)
	require.Equal(t, TaskStateOpen, detail.State.Tasks[0].State)
}

func TestServiceRequestStartAcceptsLegacyServiceSpec(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpec("build-app")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")

	err := svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle inbound work continuously.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, ControllerKindWorkflow, detail.Definition.Kind)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, "Handle inbound work continuously.", detail.State.Instruction)
	require.True(t, detail.State.ActivatedAt.IsZero())
	require.Empty(t, detail.State.ActivatedBy)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Len(t, detail.State.Tasks, 1)
	require.Equal(t, TaskStateOpen, detail.State.Tasks[0].State)

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, ControllerKindWorkflow, items[0].Kind)
	require.Equal(t, DisplayStatusRunning, items[0].DisplayStatus)
	require.True(t, items[0].Busy)
	require.False(t, items[0].NeedsInput)
}

func TestServiceRequestStartRejectsSecondActiveCycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpec("build-app")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")
	require.NoError(t, svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle inbound work continuously.",
	}))

	err := svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Different instruction",
	})
	require.ErrorContains(t, err, "controller already has an active task")
}

func TestServiceSubmitOperatorMessageRequiresActiveTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	err := svc.SubmitOperatorMessage(ctx, "software_dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "Please prioritize the flaky test first.",
	})
	require.ErrorContains(t, err, "not running an active task")
}

func TestServiceSubmitOperatorMessageReopensFinishedController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StateFinished
	state.Instruction = "Handle the current assigned task."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	state.SessionID = "sess-finished"
	state.FinishedAt = fixedTime
	state.LastSummary = "Initial run completed, but the result needs review."
	state.Tasks = cloneTasksFromTemplates(state.TaskTemplates, fixedTime)
	for i := range state.Tasks {
		state.Tasks[i].State = TaskStateDone
		state.Tasks[i].DoneAt = fixedTime
		state.Tasks[i].DoneBy = "tester"
	}
	require.NoError(t, svc.sessionStore.CreateSession(ctx, &agent.Session{
		ID:        state.SessionID,
		UserID:    svc.systemUser(def.Name).UserID,
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
	}))
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.SubmitOperatorMessage(ctx, "software_dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "The previous result was weak. Rework it with stronger sources.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.True(t, detail.State.FinishedAt.IsZero())
	require.Empty(t, detail.State.LastSummary)
	require.Len(t, detail.State.Tasks, 1)
	require.Equal(t, TaskStateOpen, detail.State.Tasks[0].State)
	require.Len(t, detail.Messages, 1)
	require.Equal(t, agent.MessageTypeUser, detail.Messages[0].Type)
	require.Contains(t, detail.Messages[0].Content, "Rework it with stronger sources.")
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "operator_message", detail.State.PendingTurnMessages[0].Kind)
	require.Contains(t, detail.State.PendingTurnMessages[0].Message, "latest user message")
}

func TestServiceSubmitOperatorMessageReopensResetFinishedController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	spec := controllerSpec("build-app") + "reset_on_finish: true\n"
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StateIdle
	state.Instruction = "Handle the current assigned task."
	state.InstructionUpdatedBy = "tester"
	state.FinishedAt = fixedTime
	state.LastSummary = "Ready for the next cycle."
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.SubmitOperatorMessage(ctx, "software_dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "The output was not good enough. Reopen and improve it.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.True(t, detail.State.FinishedAt.IsZero())
	require.Len(t, detail.State.Tasks, 1)
	require.Equal(t, TaskStateOpen, detail.State.Tasks[0].State)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "operator_message", detail.State.PendingTurnMessages[0].Kind)
	require.Contains(t, detail.State.PendingTurnMessages[0].Message, "Reopen and improve it.")
}

func TestServiceSubmitOperatorMessageAllowsIdleActiveService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpec("build-app")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")
	require.NoError(t, svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle inbound work continuously.",
	}))

	err := svc.SubmitOperatorMessage(ctx, "queue_worker", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "Watch for failed builds first.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Len(t, detail.State.PendingTurnMessages, 2)
	require.Equal(t, "operator_message", detail.State.PendingTurnMessages[1].Kind)
	require.Equal(t, fixedTime, detail.State.PendingTurnMessages[1].CreatedAt)
	require.Contains(t, detail.State.PendingTurnMessages[1].Message, "Watch for failed builds first.")
}

func TestServiceSubmitOperatorMessageQueuesMessage(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")
	require.NoError(t, svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))

	err := svc.SubmitOperatorMessage(ctx, "software_dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "Focus on the regression first.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Len(t, detail.State.PendingTurnMessages, 2)
	require.Equal(t, "operator_message", detail.State.PendingTurnMessages[1].Kind)
	require.Equal(t, fixedTime, detail.State.PendingTurnMessages[1].CreatedAt)
	require.Contains(t, detail.State.PendingTurnMessages[1].Message, "Focus on the regression first.")
}

func TestServiceSubmitOperatorMessageAppendsToSessionWhenSessionExists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StateRunning
	state.Instruction = "Handle the current assigned task."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	state.SessionID = "sess-1"
	require.NoError(t, svc.sessionStore.CreateSession(ctx, &agent.Session{
		ID:        state.SessionID,
		UserID:    svc.systemUser(def.Name).UserID,
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
	}))
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.SubmitOperatorMessage(ctx, "software_dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "Focus on the regression first.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Len(t, detail.Messages, 1)
	require.Equal(t, agent.MessageTypeUser, detail.Messages[0].Type)
	require.Contains(t, detail.Messages[0].Content, "Focus on the regression first.")
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "operator_message", detail.State.PendingTurnMessages[0].Kind)
	require.Contains(t, detail.State.PendingTurnMessages[0].Message, "latest user message")
}

func TestServiceTaskUpdateWhileBlockedAppendsToSession(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	task := createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	ref := exec.NewDAGRunRef("build-app", "run-1")
	state.State = StateRunning
	state.Instruction = "Handle the current assigned task."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	state.SessionID = "sess-1"
	state.CurrentRunRef = &ref
	state.Tasks = cloneTasksFromTemplates(state.TaskTemplates, fixedTime)

	require.NoError(t, svc.sessionStore.CreateSession(ctx, &agent.Session{
		ID:        state.SessionID,
		UserID:    svc.systemUser(def.Name).UserID,
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
	}))
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	updated, err := svc.UpdateTask(ctx, "software_dev", task.ID, UpdateTaskRequest{
		Done:        new(true),
		RequestedBy: "tester",
	})
	require.NoError(t, err)
	require.Equal(t, TaskStateDone, updated.State)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Len(t, detail.Messages, 1)
	require.Equal(t, agent.MessageTypeUser, detail.Messages[0].Type)
	require.Contains(t, detail.Messages[0].Content, "updated the task list")
	require.Contains(t, detail.Messages[0].Content, "[x] Investigate the failing test")
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "task_list_updated", detail.State.PendingTurnMessages[0].Kind)
	require.Contains(t, detail.State.PendingTurnMessages[0].Message, "latest user message")
}

func TestServiceResetStatePreservesTasksAndInstruction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	task := createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StatePaused
	state.Instruction = "Handle the current assigned task."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	state.SessionID = "sess-reset"
	state.CurrentRunRef = new(exec.NewDAGRunRef("build-app", "run-1"))
	require.NoError(t, svc.sessionStore.CreateSession(ctx, &agent.Session{
		ID:        state.SessionID,
		UserID:    svc.systemUser(def.Name).UserID,
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
	}))
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.ResetState(ctx, "software_dev")
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateIdle, detail.State.State)
	require.Equal(t, "Handle the current assigned task.", detail.State.Instruction)
	require.Empty(t, detail.State.SessionID)
	require.Nil(t, detail.State.CurrentRunRef)
	require.Empty(t, detail.State.Tasks)
	require.Len(t, detail.TaskTemplates, 1)
	require.Equal(t, task.ID, detail.TaskTemplates[0].ID)

	_, err = svc.sessionStore.GetSession(ctx, "sess-reset")
	require.ErrorIs(t, err, agent.ErrSessionNotFound)
}

func TestServiceResetStatePreservesMemory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	_, err := svc.SaveMemory(ctx, "software_dev", "# Memory\n\nRemember the standing approach.")
	require.NoError(t, err)
	_, err = svc.SaveDocument(ctx, "software_dev", DocumentSoul, "# Soul\n\nBe precise.")
	require.NoError(t, err)
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")
	require.NoError(t, svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))

	require.NoError(t, svc.ResetState(ctx, "software_dev"))

	memory, err := svc.GetMemory(ctx, "software_dev")
	require.NoError(t, err)
	require.Contains(t, memory.Content, "Remember the standing approach.")
	soul, err := svc.GetDocument(ctx, "software_dev", DocumentSoul)
	require.NoError(t, err)
	require.Contains(t, soul.Content, "Be precise.")
}

func TestServiceRenameMovesMemory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	_, err := svc.SaveMemory(ctx, "software_dev", "# Memory\n\nRemember the standing approach.")
	require.NoError(t, err)
	_, err = svc.SaveDocument(ctx, "software_dev", DocumentSoul, "# Soul\n\nBe precise.")
	require.NoError(t, err)
	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(detail.ArtifactDir, "report.md"), []byte("# report"), 0o600))

	err = svc.Rename(ctx, "software_dev", RenameRequest{NewName: "software_ops"})
	require.NoError(t, err)

	_, err = svc.GetMemory(ctx, "software_dev")
	require.ErrorIs(t, err, exec.ErrDAGNotFound)

	memory, err := svc.GetMemory(ctx, "software_ops")
	require.NoError(t, err)
	require.Contains(t, memory.Content, "Remember the standing approach.")
	require.Contains(t, memory.Path, "/controller/software_ops/MEMORY.md")
	soul, err := svc.GetDocument(ctx, "software_ops", DocumentSoul)
	require.NoError(t, err)
	require.Contains(t, soul.Content, "Be precise.")
	require.Contains(t, soul.Path, "/controller/software_ops/SOUL.md")
	require.NoFileExists(t, filepath.Join(filepath.Dir(detail.ArtifactDir), "software_dev", "report.md"))
	require.FileExists(t, filepath.Join(filepath.Dir(detail.ArtifactDir), "software_ops", "report.md"))
	renamedSpec, err := svc.GetSpec(ctx, "software_ops")
	require.NoError(t, err)
	require.NotContains(t, renamedSpec, "cloned_from:")
}

func TestServiceDuplicateCopiesMemory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	_, err := svc.SaveMemory(ctx, "software_dev", "# Memory\n\nRemember the standing approach.")
	require.NoError(t, err)
	_, err = svc.SaveDocument(ctx, "software_dev", DocumentSoul, "# Soul\n\nBe precise.")
	require.NoError(t, err)

	err = svc.Duplicate(ctx, "software_dev", DuplicateRequest{NewName: "software_ops"})
	require.NoError(t, err)

	duplicateDef, err := svc.GetDefinition(ctx, "software_ops")
	require.NoError(t, err)
	require.Equal(t, "software_dev", duplicateDef.ClonedFrom)
	duplicateSpec, err := svc.GetSpec(ctx, "software_ops")
	require.NoError(t, err)
	require.Contains(t, duplicateSpec, "cloned_from: software_dev")

	original, err := svc.GetMemory(ctx, "software_dev")
	require.NoError(t, err)
	duplicate, err := svc.GetMemory(ctx, "software_ops")
	require.NoError(t, err)
	require.Equal(t, original.Content, duplicate.Content)
	originalSoul, err := svc.GetDocument(ctx, "software_dev", DocumentSoul)
	require.NoError(t, err)
	duplicateSoul, err := svc.GetDocument(ctx, "software_ops", DocumentSoul)
	require.NoError(t, err)
	require.Equal(t, originalSoul.Content, duplicateSoul.Content)

	err = svc.Duplicate(ctx, "software_ops", DuplicateRequest{NewName: "software_ops_next"})
	require.NoError(t, err)
	secondDuplicateDef, err := svc.GetDefinition(ctx, "software_ops_next")
	require.NoError(t, err)
	require.Equal(t, "software_ops", secondDuplicateDef.ClonedFrom)
}

func TestServiceDeleteRemovesMemory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	_, err := svc.SaveMemory(ctx, "software_dev", "# Memory\n\nRemember the standing approach.")
	require.NoError(t, err)
	_, err = svc.SaveDocument(ctx, "software_dev", DocumentSoul, "# Soul\n\nBe precise.")
	require.NoError(t, err)
	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(detail.ArtifactDir, "report.md"), []byte("# report"), 0o600))

	err = svc.Delete(ctx, "software_dev")
	require.NoError(t, err)

	if svc.memoryStore != nil {
		content, loadErr := svc.memoryStore.LoadControllerMemory(ctx, "software_dev")
		require.NoError(t, loadErr)
		require.Empty(t, content)
		store, ok := svc.memoryStore.(agent.ControllerDocumentStore)
		require.True(t, ok)
		soul, loadErr := store.LoadControllerDocument(ctx, "software_dev", DocumentSoul)
		require.NoError(t, loadErr)
		require.Empty(t, soul)
	}
	require.NoDirExists(t, detail.ArtifactDir)
}

func TestServicePauseAndResumeTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")
	require.NoError(t, svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))

	err := svc.Pause(ctx, "software_dev", "tester")
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StatePaused, detail.State.State)
	require.Equal(t, "tester", detail.State.PausedBy)
	require.Equal(t, fixedTime, detail.State.PausedAt)

	err = svc.Resume(ctx, "software_dev", "tester")
	require.NoError(t, err)

	detail, err = svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Empty(t, detail.State.PausedBy)
	require.True(t, detail.State.PausedAt.IsZero())
}

func TestServiceResumePausedPromptReturnsWaiting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
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

	err = svc.Resume(ctx, "software_dev", "tester")
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateWaiting, detail.State.State)
	require.Equal(t, WaitingReasonHuman, detail.State.WaitingReason)
}

func TestServicePauseRejectsIdleController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpec("build-app")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")
	require.NoError(t, svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle inbound work continuously.",
	}))
	def, err := svc.GetDefinition(ctx, "queue_worker")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.State = StateIdle
	state.PendingTurnMessages = nil
	state.Tasks = nil
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	err = svc.Pause(ctx, "queue_worker", "tester")
	require.ErrorContains(t, err, "only active controller can be paused")

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, StateIdle, detail.State.State)
	require.True(t, detail.State.PausedAt.IsZero())
	require.Empty(t, detail.State.PausedBy)
	require.Empty(t, detail.State.PendingTurnMessages)
}

func TestServiceSubmitHumanResponseWhilePausedQueuesWithoutResuming(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
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

	err = svc.SubmitHumanResponse(ctx, "software_dev", HumanResponseRequest{
		PromptID:         "prompt-1",
		FreeTextResponse: "approved",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StatePaused, detail.State.State)
	require.Nil(t, detail.State.PendingPrompt)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "human_response", detail.State.PendingTurnMessages[0].Kind)
}

func TestServiceReconcileOnceDoesNotWakeIdleScheduledController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpecWithCronTrigger("build-app", "* * * * *")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.Instruction = "Keep shipping queued work."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	require.NoError(t, svc.ReconcileOnce(ctx))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateIdle, detail.State.State)
	require.Equal(t, "Keep shipping queued work.", detail.State.Instruction)
	require.Empty(t, detail.State.PendingTurnMessages)
}

func TestServiceHandleScheduleTickQueuesTurnForActiveService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpecWithCronTrigger("build-app", "* * * * *")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")
	def, err := svc.GetDefinition(ctx, "queue_worker")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	require.NoError(t, svc.startCycle(ctx, def, state, "Handle inbound work continuously.", "tester"))
	state.State = StateIdle
	state.PendingTurnMessages = nil
	state.Tasks = nil
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	require.NoError(t, svc.HandleScheduleTick(ctx, fixedTime))

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, fixedTime, detail.State.LastScheduleMinute)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "scheduled_tick", detail.State.PendingTurnMessages[0].Kind)
}

func TestServiceRequestStartRejectsCronTriggeredController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpecWithCronTrigger("build-app", "* * * * *")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")

	err := svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle inbound work continuously.",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "manual")
}

func TestServiceHandleScheduleTickIgnoresInactiveOrTasklessService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpecWithCronTrigger("build-app", "* * * * *")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")

	require.NoError(t, svc.HandleScheduleTick(ctx, fixedTime))

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, fixedTime, detail.State.LastScheduleMinute)
	require.Len(t, detail.State.PendingTurnMessages, 1)

	require.NoError(t, svc.PutSpec(ctx, "taskless_worker", `kind: service
trigger:
  type: cron
  schedules:
    - "* * * * *"
  prompt: "Handle inbound work continuously."
description: Software development controller
goal: Complete the assigned software work
workflows:
  names:
    - build-app
`))
	require.NoError(t, svc.HandleScheduleTick(ctx, fixedTime))

	detail, err = svc.Detail(ctx, "taskless_worker")
	require.NoError(t, err)
	require.Equal(t, StateIdle, detail.State.State)
	require.Equal(t, fixedTime, detail.State.LastScheduleMinute)
	require.Contains(t, detail.State.LastError, "requires at least one task template")
}

func TestServiceReconcileOnceReturnsInactiveRunningControllerToFinished(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)
	configStore := &testAgentConfigStore{
		cfg: &agent.Config{
			Enabled:        true,
			DefaultModelID: "local-test",
		},
	}
	modelStore := &testAgentModelStore{
		models: map[string]*agent.ModelConfig{
			"local-test": {
				ID:       "local-test",
				Name:     "Local Test",
				Provider: "local",
				Model:    "local-test",
				BaseURL:  "http://127.0.0.1:11434/v1",
			},
		},
	}
	svc.agentAPI = agent.NewAPI(agent.APIConfig{
		ConfigStore:  configStore,
		ModelStore:   modelStore,
		SessionStore: svc.sessionStore,
	})

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	state.State = StateRunning
	state.Instruction = "Handle the current assigned task."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	state.SessionID = "sess-inactive"

	require.NoError(t, svc.sessionStore.CreateSession(ctx, &agent.Session{
		ID:        state.SessionID,
		UserID:    svc.systemUser(def.Name).UserID,
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
	}))
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	require.NoError(t, svc.ReconcileOnce(ctx))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateFinished, detail.State.State)
	require.Equal(t, WaitingReasonNone, detail.State.WaitingReason)
	require.Equal(t, "Handle the current assigned task.", detail.State.Instruction)
	require.Equal(t, fixedTime, detail.State.FinishedAt)
}

func TestServiceReconcileOnceResetsInactiveRunningControllerWhenConfigured(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)
	configStore := &testAgentConfigStore{
		cfg: &agent.Config{
			Enabled:        true,
			DefaultModelID: "local-test",
		},
	}
	modelStore := &testAgentModelStore{
		models: map[string]*agent.ModelConfig{
			"local-test": {
				ID:       "local-test",
				Name:     "Local Test",
				Provider: "local",
				Model:    "local-test",
				BaseURL:  "http://127.0.0.1:11434/v1",
			},
		},
	}
	svc.agentAPI = agent.NewAPI(agent.APIConfig{
		ConfigStore:  configStore,
		ModelStore:   modelStore,
		SessionStore: svc.sessionStore,
	})

	spec := controllerSpec("build-app") + "reset_on_finish: true\n"
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	state.State = StateRunning
	state.Instruction = "Handle the current assigned task."
	state.InstructionUpdatedAt = fixedTime
	state.InstructionUpdatedBy = "tester"
	state.SessionID = "sess-inactive-reset"

	require.NoError(t, svc.sessionStore.CreateSession(ctx, &agent.Session{
		ID:        state.SessionID,
		UserID:    svc.systemUser(def.Name).UserID,
		CreatedAt: fixedTime,
		UpdatedAt: fixedTime,
	}))
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	require.NoError(t, svc.ReconcileOnce(ctx))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateIdle, detail.State.State)
	require.Equal(t, WaitingReasonNone, detail.State.WaitingReason)
	require.Equal(t, "Handle the current assigned task.", detail.State.Instruction)
	require.Equal(t, fixedTime, detail.State.FinishedAt)
}

func TestServiceReconcileOnceRecordsActionableDefaultModelError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)
	configStore := &testAgentConfigStore{
		cfg: &agent.Config{
			Enabled:        true,
			DefaultModelID: "missing-model",
		},
	}
	svc.agentAPI = agent.NewAPI(agent.APIConfig{
		ConfigStore:  configStore,
		ModelStore:   &testAgentModelStore{models: map[string]*agent.ModelConfig{}},
		SessionStore: svc.sessionStore,
	})

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")
	require.NoError(t, svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))

	require.NoError(t, svc.ReconcileOnce(ctx))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, fixedTime, detail.State.InstructionUpdatedAt)
	require.Contains(t, detail.State.LastError, `failed to resolve default model "missing-model"`)
	require.Contains(t, detail.State.LastError, "model not found")
}

func TestServiceRuntimeOptionsIncludeExplicitModel(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `trigger:
  type: manual
goal: Complete the assigned software work
workflows:
  names:
    - build-app
agent:
  model: claude-sonnet-4-6
  safeMode: true
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	opts, err := svc.runtimeOptions(ctx, def, state)
	require.NoError(t, err)
	require.Equal(t, "claude-sonnet-4-6", opts.Model)
	require.Equal(t, filepath.Join(svc.stateDir, "software_dev", "workspace"), opts.WorkingDir)
	require.DirExists(t, opts.WorkingDir)
}

func TestServiceRuntimeOptionsPromptRestrictsWorkflowExecutionToConfiguredList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	opts, err := svc.runtimeOptions(ctx, def, state)
	require.NoError(t, err)
	require.Contains(t, opts.SystemPromptExtra, "Run only workflows listed under Managed workflows.")
	require.Contains(t, opts.SystemPromptExtra, "After every workflow run, inspect the latest result before deciding what to do next.")
	require.Contains(t, opts.SystemPromptExtra, "If the latest workflow result is incomplete, incorrect, brittle, or missing expected artifacts, update that workflow with patch before rerunning it or relying on its output.")
	require.Contains(t, opts.SystemPromptExtra, "If none of the configured workflows fits the task, create a new workflow YAML in the DAGs directory with patch, register it with register_workflow, then run it.")
	require.Contains(t, opts.SystemPromptExtra, "Use register_workflow to add a created DAG name to this controller's workflows.names list. Do not edit the controller spec manually.")
	require.Contains(t, opts.SystemPromptExtra, "Controller spec path:")
	require.Contains(t, opts.SystemPromptExtra, "Controller artifacts directory: "+filepath.Join(svc.stateDir, "artifacts", "software_dev"))
	require.Contains(t, opts.SystemPromptExtra, "prefer configuring their top-level artifacts.dir to the Controller artifacts directory")
}

func TestServiceBuildRunCompletionMessageDirectsControllerToInspectAndPatchBadWorkflowResults(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t)

	message := svc.buildRunCompletionMessage(&exec.DAGRunStatus{
		Name:     "build-app",
		DAGRunID: "run-1",
		Status:   core.Failed,
		Error:    "workflow failed",
	})

	require.Contains(t, message, "Inspect this latest result before deciding the next action.")
	require.Contains(t, message, "If the workflow itself caused a bad or incomplete result, patch the workflow before rerunning it or depending on its output.")
}

func TestControllerRuntimeDefaultWorkingDirForDAGRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	dag, err := svc.dagStore.GetDetails(ctx, "build-app")
	require.NoError(t, err)
	require.False(t, dag.WorkingDirExplicit)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	dir, err := rt.defaultWorkingDirForDAGRun(dag)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(svc.stateDir, "software_dev", "workspace"), dir)
	require.DirExists(t, dir)

	dag.WorkingDirExplicit = true
	dir, err = rt.defaultWorkingDirForDAGRun(dag)
	require.NoError(t, err)
	require.Empty(t, dir)
	dag.WorkingDirExplicit = false

	dag.WorkerSelector = map[string]string{"role": "worker"}
	svc.coordinatorCli = &testCoordinatorCanceler{}
	dir, err = rt.defaultWorkingDirForDAGRun(dag)
	require.NoError(t, err)
	require.Empty(t, dir)
}

func TestControllerRuntimeSetTaskDonePersists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	task := createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	state.Tasks = cloneTasksFromTemplates(state.TaskTemplates, fixedTime)
	state.CurrentCycleID = nextCycleID()
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.SetTaskDone(ctx, task.ID, true))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Len(t, detail.State.Tasks, 1)
	require.Equal(t, TaskStateDone, detail.State.Tasks[0].State)
	require.Equal(t, "agent", detail.State.Tasks[0].DoneBy)
	require.Equal(t, fixedTime, detail.State.Tasks[0].DoneAt)
}

func TestControllerRuntimeRunWorkflowRejectsConcurrentRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	ref := exec.NewDAGRunRef("build-app", "run-1")
	state.CurrentRunRef = &ref

	rt := &controllerRuntime{service: svc, def: def, state: state}
	_, err = rt.RunWorkflow(ctx, agent.ControllerRunWorkflowInput{WorkflowName: "build-app"})
	require.ErrorContains(t, err, "already active")
}

func TestControllerRuntimeRunWorkflowRejectsWorkflowOutsideConfiguredList(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	_, err = rt.RunWorkflow(ctx, agent.ControllerRunWorkflowInput{WorkflowName: "run-tests"})
	require.ErrorContains(t, err, `is not included in this controller's workflows list`)
}

func TestControllerRuntimeRunWorkflowRejectsWorkflowMatchedOnlyByLabels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Label-scoped controller
trigger:
  type: manual
goal: Complete the assigned software work
workflows:
  labels:
    - dev
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	_, err = rt.RunWorkflow(ctx, agent.ControllerRunWorkflowInput{WorkflowName: "run-tests"})
	require.ErrorContains(t, err, `is not included in this controller's workflows list`)
}

func TestControllerRuntimeListWorkflowsReturnsEmptyWhenNothingIsConfigured(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Workflow-less controller
trigger:
  type: manual
goal: Complete the assigned software work
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	items, err := rt.ListWorkflows(ctx)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestControllerRuntimeRegisterWorkflowCreatesManagedEntry(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Workflow-less controller
trigger:
  type: manual
goal: Complete the assigned software work
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	result, err := rt.RegisterWorkflow(ctx, "run-tests")
	require.NoError(t, err)
	require.Equal(t, "run-tests", result.WorkflowName)
	require.False(t, result.AlreadyManaged)

	reloaded, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, []string{"run-tests"}, reloaded.Workflows.Names)

	items, err := rt.ListWorkflows(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"run-tests"}, workflowNamesFromAgent(items))
}

func TestControllerRuntimeRegisterWorkflowPreservesExistingWorkflowNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	_, err = rt.RegisterWorkflow(ctx, "run-tests")
	require.NoError(t, err)

	reloaded, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, []string{"build-app", "run-tests"}, reloaded.Workflows.Names)
}

func TestControllerRuntimeRegisterWorkflowReturnsAlreadyManaged(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	result, err := rt.RegisterWorkflow(ctx, "build-app")
	require.NoError(t, err)
	require.Equal(t, "build-app", result.WorkflowName)
	require.True(t, result.AlreadyManaged)

	reloaded, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, []string{"build-app"}, reloaded.Workflows.Names)
}

func TestControllerRuntimeListWorkflowsIgnoresWorkflowLabels(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Label-scoped controller
trigger:
  type: manual
goal: Complete the assigned software work
workflows:
  labels:
    - dev
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	items, err := rt.ListWorkflows(ctx)
	require.NoError(t, err)
	require.Empty(t, items)
}

func TestControllerRuntimeListWorkflowsReloadsUpdatedDefinition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpecMultiDAGs()))

	items, err := rt.ListWorkflows(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"build-app", "run-tests"}, workflowNamesFromAgent(items))
}

func TestControllerRuntimeRunWorkflowUsesUpdatedDefinition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)
	svc.cfg.Paths.Executable = "true"
	svc.subCmdBuilder = runtime.NewSubCmdBuilder(svc.cfg)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpecMultiDAGs()))

	result, err := rt.RunWorkflow(ctx, agent.ControllerRunWorkflowInput{WorkflowName: "run-tests"})
	require.NoError(t, err)
	require.Equal(t, "run-tests", result.WorkflowName)
	require.NotNil(t, state.CurrentRunRef)
	require.Equal(t, "run-tests", state.CurrentRunRef.Name)
}

func TestControllerRuntimeRequestHumanInputEmitsEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime, store := newTestServiceWithEventStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.RequestHumanInput(ctx, agent.ControllerHumanPrompt{
		Question: "Approve the deployment?",
	}))

	require.Len(t, store.events, 1)
	event := store.events[0]
	require.Equal(t, eventstore.KindController, event.Kind)
	require.Equal(t, eventstore.TypeControllerNeedsInput, event.Type)
	require.Equal(t, "software_dev", event.ControllerName)
	require.Equal(t, fixedTime, event.OccurredAt)
	snapshot, err := eventstore.NotificationControllerFromEvent(event)
	require.NoError(t, err)
	require.Equal(t, "Approve the deployment?", snapshot.PromptQuestion)
	require.Equal(t, "workflow", snapshot.Kind)
}

func TestControllerRuntimeFinishRejectsActiveChildRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	ref := exec.NewDAGRunRef("build-app", "run-1")
	state.CurrentRunRef = &ref

	rt := &controllerRuntime{service: svc, def: def, state: state}
	err = rt.Finish(ctx, "done")
	require.ErrorContains(t, err, "while a child DAG run is active")
}

func TestControllerRuntimeFinishAllowsLegacyServiceController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "queue_worker")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.Finish(ctx, "done"))

	require.Equal(t, StateFinished, state.State)
	require.Equal(t, "done", state.LastSummary)
}

func TestControllerRuntimeFinishEmitsEventAndAssignsCycleID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime, store := newTestServiceWithEventStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", controllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.Finish(ctx, "All checklist tasks are complete."))

	require.NotEmpty(t, state.CurrentCycleID)
	require.Len(t, store.events, 1)
	event := store.events[0]
	require.Equal(t, eventstore.TypeControllerFinished, event.Type)
	require.Equal(t, state.CurrentCycleID, event.ControllerCycleID)
	require.Equal(t, fixedTime, event.OccurredAt)
	snapshot, err := eventstore.NotificationControllerFromEvent(event)
	require.NoError(t, err)
	require.Equal(t, "All checklist tasks are complete.", snapshot.Summary)
}

func TestControllerRuntimeFinishResetsWhenConfigured(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime, store := newTestServiceWithEventStore(t)

	spec := controllerSpec("build-app") + "reset_on_finish: true\n"
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")
	require.NoError(t, svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	}))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	cycleID := state.CurrentCycleID

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.Finish(ctx, "Ready for the next cycle."))

	require.Equal(t, StateIdle, state.State)
	require.Empty(t, state.CurrentCycleID)
	require.Empty(t, state.Tasks)
	require.Equal(t, fixedTime, state.FinishedAt)
	require.Equal(t, "Ready for the next cycle.", state.LastSummary)
	require.Len(t, store.events, 1)
	event := store.events[0]
	require.Equal(t, eventstore.TypeControllerFinished, event.Type)
	require.Equal(t, cycleID, event.ControllerCycleID)
	snapshot, err := eventstore.NotificationControllerFromEvent(event)
	require.NoError(t, err)
	require.Equal(t, "finished", snapshot.Status)
	require.Equal(t, "Ready for the next cycle.", snapshot.Summary)
}

func TestServiceRuntimeOptionsIncludeFinishToolForLegacyService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceControllerSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "queue_worker")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	opts, err := svc.runtimeOptions(ctx, def, state)
	require.NoError(t, err)
	require.Contains(t, opts.AllowedTools, "finish_controller")
	require.Contains(t, opts.AllowedTools, "patch")
	require.Contains(t, opts.AllowedTools, "request_human_input")
	require.Contains(t, opts.AllowedTools, "list_workflows")
	require.Contains(t, opts.AllowedTools, "register_workflow")
	require.Contains(t, opts.AllowedTools, "run_workflow")
}
