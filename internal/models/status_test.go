package models_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatusSerialization(t *testing.T) {
	startedAt, finishedAt := time.Now(), time.Now().Add(time.Second*1)
	dag := &digraph.DAG{
		HandlerOn: digraph.HandlerOn{},
		Steps: []digraph.Step{
			{
				Name: "1", Description: "",
				Dir: "dir", Command: "echo 1", Args: []string{},
				Depends: []string{}, ContinueOn: digraph.ContinueOn{},
				RetryPolicy: digraph.RetryPolicy{}, MailOnError: false,
				RepeatPolicy: digraph.RepeatPolicy{}, Preconditions: []*digraph.Condition{},
			},
		},
		MailOn:    &digraph.MailOn{},
		ErrorMail: &digraph.MailConfig{},
		InfoMail:  &digraph.MailConfig{},
		SMTP:      &digraph.SMTPConfig{},
	}
	dagRunID := uuid.Must(uuid.NewV7()).String()
	statusToPersist := models.NewStatusBuilder(dag).Create(dagRunID, scheduler.StatusSuccess, 0, startedAt, models.WithFinishedAt(finishedAt))

	rawJSON, err := json.Marshal(statusToPersist)
	require.NoError(t, err)

	statusObject, err := models.StatusFromJSON(string(rawJSON))
	require.NoError(t, err)

	require.Equal(t, statusToPersist.Name, statusObject.Name)
	require.Equal(t, 1, len(statusObject.Nodes))
	require.Equal(t, dag.Steps[0].Name, statusObject.Nodes[0].Step.Name)
}

func TestStatusBuilder(t *testing.T) {
	dag := &digraph.DAG{
		Name: "test-dag",
		HandlerOn: digraph.HandlerOn{
			Exit:    &digraph.Step{Name: "exit-handler"},
			Success: &digraph.Step{Name: "success-handler"},
			Failure: &digraph.Step{Name: "failure-handler"},
			Cancel:  &digraph.Step{Name: "cancel-handler"},
		},
		Steps: []digraph.Step{
			{Name: "step1", Command: "echo hello"},
			{Name: "step2", Command: "echo world"},
		},
		Params: []string{"param1", "param2"},
		Preconditions: []*digraph.Condition{
			{Condition: "test -f file.txt", Expected: "true"},
		},
	}

	builder := models.NewStatusBuilder(dag)
	dagRunID := "test-run-123"
	status := scheduler.StatusRunning
	pid := 12345
	startedAt := time.Now()

	// Test basic creation
	result := builder.Create(dagRunID, status, pid, startedAt)

	assert.Equal(t, dag.Name, result.Name)
	assert.Equal(t, dagRunID, result.DAGRunID)
	assert.Equal(t, status, result.Status)
	assert.Equal(t, models.PID(pid), result.PID)
	assert.NotEmpty(t, result.StartedAt)
	assert.Equal(t, 2, len(result.Nodes))
	assert.NotNil(t, result.OnExit)
	assert.NotNil(t, result.OnSuccess)
	assert.NotNil(t, result.OnFailure)
	assert.NotNil(t, result.OnCancel)
	assert.Equal(t, "param1 param2", result.Params)
	assert.Equal(t, dag.Params, result.ParamsList)
	assert.Equal(t, dag.Preconditions, result.Preconditions)
}

func TestStatusBuilderWithOptions(t *testing.T) {
	dag := &digraph.DAG{
		Name: "test-dag",
		Steps: []digraph.Step{
			{Name: "step1"},
		},
	}

	builder := models.NewStatusBuilder(dag)
	dagRunID := "test-run-456"
	status := scheduler.StatusSuccess
	pid := 54321
	startedAt := time.Now()
	finishedAt := startedAt.Add(5 * time.Minute)

	// Create nodes for options
	nodes := []scheduler.NodeData{
		{
			Step: digraph.Step{Name: "step1"},
			State: scheduler.NodeState{
				Status:     scheduler.NodeStatusSuccess,
				StartedAt:  startedAt,
				FinishedAt: finishedAt,
			},
		},
	}

	exitNode := scheduler.NewNode(digraph.Step{Name: "exit-step"}, scheduler.NodeState{})
	successNode := scheduler.NewNode(digraph.Step{Name: "success-step"}, scheduler.NodeState{})
	failureNode := scheduler.NewNode(digraph.Step{Name: "failure-step"}, scheduler.NodeState{})
	cancelNode := scheduler.NewNode(digraph.Step{Name: "cancel-step"}, scheduler.NodeState{})

	rootRef := digraph.NewDAGRunRef("root-dag", "root-run-123")
	parentRef := digraph.NewDAGRunRef("parent-dag", "parent-run-456")

	// Test with all options
	result := builder.Create(
		dagRunID,
		status,
		pid,
		startedAt,
		models.WithFinishedAt(finishedAt),
		models.WithNodes(nodes),
		models.WithOnExitNode(exitNode),
		models.WithOnSuccessNode(successNode),
		models.WithOnFailureNode(failureNode),
		models.WithOnCancelNode(cancelNode),
		models.WithLogFilePath("/tmp/log.txt"),
		models.WithPreconditions([]*digraph.Condition{{Condition: "test", Expected: "true"}}),
		models.WithHierarchyRefs(rootRef, parentRef),
		models.WithAttemptID("attempt-789"),
		models.WithQueuedAt("2024-01-01 12:00:00"),
		models.WithCreatedAt(1234567890),
	)

	assert.Equal(t, stringutil.FormatTime(finishedAt), result.FinishedAt)
	assert.Equal(t, 1, len(result.Nodes))
	assert.Equal(t, "exit-step", result.OnExit.Step.Name)
	assert.Equal(t, "success-step", result.OnSuccess.Step.Name)
	assert.Equal(t, "failure-step", result.OnFailure.Step.Name)
	assert.Equal(t, "cancel-step", result.OnCancel.Step.Name)
	assert.Equal(t, "/tmp/log.txt", result.Log)
	assert.Equal(t, 1, len(result.Preconditions))
	assert.Equal(t, rootRef, result.Root)
	assert.Equal(t, parentRef, result.Parent)
	assert.Equal(t, "attempt-789", result.AttemptID)
	assert.Equal(t, "2024-01-01 12:00:00", result.QueuedAt)
	assert.Equal(t, int64(1234567890), result.CreatedAt)
}

func TestInitialStatus(t *testing.T) {
	dag := &digraph.DAG{
		Name: "initial-test",
		HandlerOn: digraph.HandlerOn{
			Exit:    &digraph.Step{Name: "exit"},
			Success: &digraph.Step{Name: "success"},
			Failure: &digraph.Step{Name: "failure"},
			Cancel:  &digraph.Step{Name: "cancel"},
		},
		Steps: []digraph.Step{
			{Name: "step1"},
			{Name: "step2"},
		},
		Params: []string{"arg1", "arg2"},
		Preconditions: []*digraph.Condition{
			{Condition: "test condition"},
		},
	}

	status := models.InitialStatus(dag)

	assert.Equal(t, dag.Name, status.Name)
	assert.Equal(t, scheduler.StatusNone, status.Status)
	assert.Equal(t, models.PID(0), status.PID)
	assert.Equal(t, 2, len(status.Nodes))
	assert.NotNil(t, status.OnExit)
	assert.NotNil(t, status.OnSuccess)
	assert.NotNil(t, status.OnFailure)
	assert.NotNil(t, status.OnCancel)
	assert.Equal(t, "arg1 arg2", status.Params)
	assert.Equal(t, dag.Params, status.ParamsList)
	assert.Equal(t, dag.Preconditions, status.Preconditions)
	assert.NotZero(t, status.CreatedAt)
	assert.Equal(t, "", status.StartedAt)
	assert.Equal(t, "", status.FinishedAt)
}

func TestStatusFromJSONError(t *testing.T) {
	// Test with invalid JSON
	_, err := models.StatusFromJSON("invalid json")
	assert.Error(t, err)

	// Test with empty string
	_, err = models.StatusFromJSON("")
	assert.Error(t, err)
}

func TestDAGRunStatus_DAGRun(t *testing.T) {
	status := &models.DAGRunStatus{
		Name:     "test-dag",
		DAGRunID: "run-123",
	}

	dagRun := status.DAGRun()
	assert.Equal(t, "test-dag", dagRun.Name)
	assert.Equal(t, "run-123", dagRun.ID)
}

func TestDAGRunStatus_Errors(t *testing.T) {
	status := &models.DAGRunStatus{
		Nodes: []*models.Node{
			{Step: digraph.Step{Name: "step1"}, Error: "error1"},
			{Step: digraph.Step{Name: "step2"}, Error: ""},
			{Step: digraph.Step{Name: "step3"}, Error: "error3"},
		},
		OnExit:    &models.Node{Step: digraph.Step{Name: "exit"}, Error: "exit error"},
		OnSuccess: &models.Node{Step: digraph.Step{Name: "success"}, Error: ""},
		OnFailure: &models.Node{Step: digraph.Step{Name: "failure"}, Error: "failure error"},
		OnCancel:  &models.Node{Step: digraph.Step{Name: "cancel"}, Error: "cancel error"},
	}

	errors := status.Errors()
	assert.Equal(t, 5, len(errors))
	assert.Contains(t, errors[0].Error(), "node step1: error1")
	assert.Contains(t, errors[1].Error(), "node step3: error3")
	assert.Contains(t, errors[2].Error(), "onExit: exit error")
	assert.Contains(t, errors[3].Error(), "onFailure: failure error")
	assert.Contains(t, errors[4].Error(), "onCancel: cancel error")
}

func TestDAGRunStatus_NodeByName(t *testing.T) {
	status := &models.DAGRunStatus{
		Nodes: []*models.Node{
			{Step: digraph.Step{Name: "step1"}},
			{Step: digraph.Step{Name: "step2"}},
		},
		OnExit:    &models.Node{Step: digraph.Step{Name: "exit"}},
		OnSuccess: &models.Node{Step: digraph.Step{Name: "success"}},
		OnFailure: &models.Node{Step: digraph.Step{Name: "failure"}},
		OnCancel:  &models.Node{Step: digraph.Step{Name: "cancel"}},
	}

	// Test finding regular nodes
	node, err := status.NodeByName("step1")
	assert.NoError(t, err)
	assert.Equal(t, "step1", node.Step.Name)

	// Test finding handler nodes
	node, err = status.NodeByName("exit")
	assert.NoError(t, err)
	assert.Equal(t, "exit", node.Step.Name)

	node, err = status.NodeByName("success")
	assert.NoError(t, err)
	assert.Equal(t, "success", node.Step.Name)

	// Test node not found
	_, err = status.NodeByName("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "node nonexistent not found")
}

func TestPID_String(t *testing.T) {
	tests := []struct {
		name     string
		pid      models.PID
		expected string
	}{
		{
			name:     "positive PID",
			pid:      models.PID(12345),
			expected: "12345",
		},
		{
			name:     "zero PID",
			pid:      models.PID(0),
			expected: "",
		},
		{
			name:     "negative PID",
			pid:      models.PID(-1),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.pid.String())
		})
	}
}

func TestNodesFromSteps(t *testing.T) {
	steps := []digraph.Step{
		{
			Name:        "step1",
			Command:     "echo hello",
			Description: "First step",
		},
		{
			Name:        "step2",
			Command:     "echo world",
			Description: "Second step",
		},
	}

	nodes := models.NodesFromSteps(steps)

	assert.Equal(t, 2, len(nodes))
	assert.Equal(t, "step1", nodes[0].Step.Name)
	assert.Equal(t, "step2", nodes[1].Step.Name)
	assert.Equal(t, scheduler.NodeStatusNone, nodes[0].Status)
	assert.Equal(t, scheduler.NodeStatusNone, nodes[1].Status)
}

func TestWithCreatedAtDefaultTime(t *testing.T) {
	dag := &digraph.DAG{Name: "test"}
	status := models.InitialStatus(dag)

	// Test WithCreatedAt with 0 - should use current time
	beforeTime := time.Now().UnixMilli()
	models.WithCreatedAt(0)(&status)
	afterTime := time.Now().UnixMilli()

	assert.GreaterOrEqual(t, status.CreatedAt, beforeTime)
	assert.LessOrEqual(t, status.CreatedAt, afterTime)
}
