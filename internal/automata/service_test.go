// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/core/exec"
	_ "github.com/dagucloud/dagu/internal/llm/allproviders"
	"github.com/dagucloud/dagu/internal/service/eventstore"
	"github.com/stretchr/testify/require"
)

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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))

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
	require.Equal(t, []string{"build-app"}, allowedDAGNames(detail.AllowedDAGs))
}

func TestServiceDetailUsesTopLevelAllowedDAGs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpecMultiDAGs()))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, []string{"build-app", "run-tests"}, allowedDAGNames(detail.AllowedDAGs))
}

func TestServicePutSpecAcceptsLegacyPurposeAsGoalAlias(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `description: Legacy automata
purpose: Complete the assigned software work
allowed_dags:
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

	spec := `description: Goal-free automata
allowed_dags:
  names:
    - build-app
`
	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Empty(t, detail.Definition.Goal)
	require.Equal(t, "Goal-free automata", detail.Definition.Description)
}

func TestServicePutSpecRejectsStagesField(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `goal: Complete the assigned software work
stages:
  - research
allowed_dags:
  names:
    - build-app
`

	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, `unknown field "stages"`)
}

func TestServicePutSpecRejectsDotsAndHyphensInAutomataName(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	err := svc.PutSpec(ctx, "software.dev", automataSpec("build-app"))
	require.ErrorContains(t, err, `invalid automata name "software.dev"`)

	err = svc.PutSpec(ctx, "software-dev", automataSpec("build-app"))
	require.ErrorContains(t, err, `invalid automata name "software-dev"`)
}

func TestServicePutSpecNormalizesTagsAndExposesThemInSummary(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `goal: Complete the assigned software work
tags:
  - workspace=Engineering
  - Owner=Team-AI
allowed_dags:
  names:
    - build-app
`

	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, []string{"workspace=engineering", "owner=team-ai"}, detail.Definition.Tags)

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, []string{"workspace=engineering", "owner=team-ai"}, items[0].Tags)
}

func TestServicePutSpecRejectsInvalidTags(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `goal: Complete the assigned software work
tags:
  - "bad tag"
allowed_dags:
  names:
    - build-app
`

	err := svc.PutSpec(ctx, "software_dev", spec)
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid tags")
}

func TestServicePutSpecExposesNicknameAndIconURL(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	spec := `nickname: Build Captain
icon_url: https://cdn.example.com/automata/build-captain.png
goal: Complete the assigned software work
allowed_dags:
  names:
    - build-app
`

	require.NoError(t, svc.PutSpec(ctx, "software_dev", spec))

	detail, err := svc.Detail(ctx, "software_dev")
	require.NoError(t, err)
	require.Equal(t, "Build Captain", detail.Definition.Nickname)
	require.Equal(t, "https://cdn.example.com/automata/build-captain.png", detail.Definition.IconURL)

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Build Captain", items[0].Nickname)
	require.Equal(t, "https://cdn.example.com/automata/build-captain.png", items[0].IconURL)
}

func TestServicePutSpecResetsRuntimeWhenAgentConfigChanges(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpecWithModel("build-app", "model-a")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpecWithModel("build-app", "model-b")))

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

	spec := `goal: Complete the assigned software work
icon_url: javascript:alert(1)
allowed_dags:
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

	err := svc.PutSpec(ctx, "software_dev", `goal: Complete the assigned software work
bogus: true
allowed_dags:
  names:
    - build-app
`)
	require.ErrorContains(t, err, `unknown field "bogus"`)

	err = svc.PutSpec(ctx, "software_dev", `goal: Complete the assigned software work
allowed_dags:
  names:
    - build-app
  bogus:
    - nope
`)
	require.ErrorContains(t, err, `unknown field "bogus"`)

	err = svc.PutSpec(ctx, "software_dev", `goal: Complete the assigned software work
allowed_dags:
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))

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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))

	err := svc.RequestStart(ctx, "software_dev", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle the current assigned task.",
	})
	require.ErrorContains(t, err, "at least one task template is required")

	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	err = svc.RequestStart(ctx, "software_dev", StartRequest{RequestedBy: "tester"})
	require.ErrorContains(t, err, "instruction is required before starting automata")
}

func TestServiceRequestStartStoresInstruction(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

func TestServiceRequestStartActivatesServiceWithoutOpenTasks(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceAutomataSpec("build-app")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")

	err := svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle inbound work continuously.",
	})
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, AutomataKindService, detail.Definition.Kind)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, "Handle inbound work continuously.", detail.State.Instruction)
	require.Equal(t, fixedTime, detail.State.ActivatedAt)
	require.Equal(t, "tester", detail.State.ActivatedBy)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Len(t, detail.State.Tasks, 1)
	require.Equal(t, TaskStateOpen, detail.State.Tasks[0].State)

	items, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, AutomataKindService, items[0].Kind)
	require.Equal(t, DisplayStatusRunning, items[0].DisplayStatus)
	require.True(t, items[0].Busy)
	require.False(t, items[0].NeedsInput)
}

func TestServiceRequestStartRejectsSecondServiceActivation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceAutomataSpec("build-app")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")
	require.NoError(t, svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Handle inbound work continuously.",
	}))

	err := svc.RequestStart(ctx, "queue_worker", StartRequest{
		RequestedBy: "tester",
		Instruction: "Different instruction",
	})
	require.ErrorContains(t, err, "service automata is already active")
}

func TestServiceSubmitOperatorMessageRequiresActiveTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
	createTask(t, svc, ctx, "software_dev", "Investigate the failing test", "tester")

	err := svc.SubmitOperatorMessage(ctx, "software_dev", OperatorMessageRequest{
		RequestedBy: "tester",
		Message:     "Please prioritize the flaky test first.",
	})
	require.ErrorContains(t, err, "not running an active task")
}

func TestServiceSubmitOperatorMessageAllowsIdleActiveService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceAutomataSpec("build-app")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
	_, err := svc.SaveMemory(ctx, "software_dev", "# Memory\n\nRemember the standing approach.")
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
}

func TestServiceRenameMovesMemory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestServiceWithSessionStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
	_, err := svc.SaveMemory(ctx, "software_dev", "# Memory\n\nRemember the standing approach.")
	require.NoError(t, err)

	err = svc.Rename(ctx, "software_dev", RenameRequest{NewName: "software_ops"})
	require.NoError(t, err)

	_, err = svc.GetMemory(ctx, "software_dev")
	require.ErrorIs(t, err, exec.ErrDAGNotFound)

	memory, err := svc.GetMemory(ctx, "software_ops")
	require.NoError(t, err)
	require.Contains(t, memory.Content, "Remember the standing approach.")
	require.Contains(t, memory.Path, "/automata/software_ops/MEMORY.md")
}

func TestServiceDuplicateCopiesMemory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
	_, err := svc.SaveMemory(ctx, "software_dev", "# Memory\n\nRemember the standing approach.")
	require.NoError(t, err)

	err = svc.Duplicate(ctx, "software_dev", DuplicateRequest{NewName: "software_ops"})
	require.NoError(t, err)

	original, err := svc.GetMemory(ctx, "software_dev")
	require.NoError(t, err)
	duplicate, err := svc.GetMemory(ctx, "software_ops")
	require.NoError(t, err)
	require.Equal(t, original.Content, duplicate.Content)
}

func TestServiceDeleteRemovesMemory(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
	_, err := svc.SaveMemory(ctx, "software_dev", "# Memory\n\nRemember the standing approach.")
	require.NoError(t, err)

	err = svc.Delete(ctx, "software_dev")
	require.NoError(t, err)

	if svc.memoryStore != nil {
		content, loadErr := svc.memoryStore.LoadAutomataMemory(ctx, "software_dev")
		require.NoError(t, loadErr)
		require.Empty(t, content)
	}
}

func TestServicePauseAndResumeTask(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

func TestServicePauseAndResumeStandbyService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceAutomataSpec("build-app")))
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
	require.NoError(t, err)

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, StatePaused, detail.State.State)
	require.Equal(t, fixedTime, detail.State.PausedAt)
	require.Equal(t, "tester", detail.State.PausedBy)

	err = svc.Resume(ctx, "queue_worker", "tester")
	require.NoError(t, err)

	detail, err = svc.Detail(ctx, "queue_worker")
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

func TestServiceReconcileOnceDoesNotWakeIdleScheduledAutomata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpecWithSchedule("build-app", "* * * * *")))
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

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceAutomataSpecWithSchedule("build-app", "* * * * *")))
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

	require.NoError(t, svc.HandleScheduleTick(ctx, fixedTime))

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, fixedTime, detail.State.LastScheduleMinute)
	require.Len(t, detail.State.PendingTurnMessages, 1)
	require.Equal(t, "scheduled_tick", detail.State.PendingTurnMessages[0].Kind)
}

func TestServiceHandleScheduleTickIgnoresInactiveOrTasklessService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceAutomataSpecWithSchedule("build-app", "* * * * *")))
	createTask(t, svc, ctx, "queue_worker", "Process the next queued request", "tester")

	require.NoError(t, svc.HandleScheduleTick(ctx, fixedTime))

	detail, err := svc.Detail(ctx, "queue_worker")
	require.NoError(t, err)
	require.Equal(t, StateRunning, detail.State.State)
	require.Equal(t, fixedTime, detail.State.LastScheduleMinute)
	require.Len(t, detail.State.PendingTurnMessages, 1)

	require.NoError(t, svc.PutSpec(ctx, "taskless_worker", `kind: service
description: Software development automata
goal: Complete the assigned software work
standing_instruction: Handle inbound work continuously.
schedule: "* * * * *"
allowed_dags:
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

func TestServiceReconcileOnceReturnsInactiveRunningAutomataToIdle(t *testing.T) {
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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
	require.Equal(t, StateIdle, detail.State.State)
	require.Equal(t, WaitingReasonNone, detail.State.WaitingReason)
	require.Equal(t, "Handle the current assigned task.", detail.State.Instruction)
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

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

	spec := `goal: Complete the assigned software work
allowed_dags:
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
}

func TestControllerRuntimeSetTaskDonePersists(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

func TestControllerRuntimeRunAllowedDAGRejectsConcurrentRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	ref := exec.NewDAGRunRef("build-app", "run-1")
	state.CurrentRunRef = &ref

	rt := &controllerRuntime{service: svc, def: def, state: state}
	_, err = rt.RunAllowedDAG(ctx, agent.AutomataRunDAGInput{DAGName: "build-app"})
	require.ErrorContains(t, err, "already active")
}

func TestControllerRuntimeRequestHumanInputEmitsEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime, store := newTestServiceWithEventStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.RequestHumanInput(ctx, agent.AutomataHumanPrompt{
		Question: "Approve the deployment?",
	}))

	require.Len(t, store.events, 1)
	event := store.events[0]
	require.Equal(t, eventstore.KindAutomata, event.Kind)
	require.Equal(t, eventstore.TypeAutomataNeedsInput, event.Type)
	require.Equal(t, "software_dev", event.AutomataName)
	require.Equal(t, fixedTime, event.OccurredAt)
	snapshot, err := eventstore.NotificationAutomataFromEvent(event)
	require.NoError(t, err)
	require.Equal(t, "Approve the deployment?", snapshot.PromptQuestion)
	require.Equal(t, "workflow", snapshot.Kind)
}

func TestControllerRuntimeFinishRejectsActiveChildRun(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
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

func TestControllerRuntimeFinishRejectsServiceAutomata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceAutomataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "queue_worker")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	err = rt.Finish(ctx, "done")
	require.ErrorContains(t, err, "cannot finish a service automata")
}

func TestControllerRuntimeFinishEmitsEventAndAssignsCycleID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, fixedTime, store := newTestServiceWithEventStore(t)

	require.NoError(t, svc.PutSpec(ctx, "software_dev", automataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	rt := &controllerRuntime{service: svc, def: def, state: state}
	require.NoError(t, rt.Finish(ctx, "All checklist tasks are complete."))

	require.NotEmpty(t, state.CurrentCycleID)
	require.Len(t, store.events, 1)
	event := store.events[0]
	require.Equal(t, eventstore.TypeAutomataFinished, event.Type)
	require.Equal(t, state.CurrentCycleID, event.AutomataCycleID)
	require.Equal(t, fixedTime, event.OccurredAt)
	snapshot, err := eventstore.NotificationAutomataFromEvent(event)
	require.NoError(t, err)
	require.Equal(t, "All checklist tasks are complete.", snapshot.Summary)
}

func TestServiceRuntimeOptionsExcludeFinishToolForService(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _ := newTestService(t)

	require.NoError(t, svc.PutSpec(ctx, "queue_worker", serviceAutomataSpec("build-app")))
	def, err := svc.GetDefinition(ctx, "queue_worker")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)

	opts, err := svc.runtimeOptions(ctx, def, state)
	require.NoError(t, err)
	require.NotContains(t, opts.AllowedTools, "finish_automata")
	require.Contains(t, opts.AllowedTools, "request_human_input")
}
