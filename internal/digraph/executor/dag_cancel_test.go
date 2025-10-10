package executor

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDAGExecutor_Kill_Distributed(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := Env{
		DAGContext: digraph.DAGContext{
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = WithEnv(context.Background(), env)

	// Create a child DAG with worker selector for distributed execution
	childDAG := &digraph.DAG{
		Name: "child-dag",
		WorkerSelector: map[string]string{
			"type": "test-worker",
		},
	}

	// Create child executor with distributed run tracked
	child := &ChildDAGExecutor{
		DAG:             childDAG,
		distributedRuns: map[string]bool{"child-run-id": true},
		env:             env,
		cmds:            make(map[string]*exec.Cmd),
	}

	// Create a DAG executor
	executor := &dagExecutor{
		child: child,
	}

	// Set up expectation for RequestChildCancel
	mockDB.On("RequestChildCancel", mock.Anything, "child-run-id", env.RootDAGRun).Return(nil)

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was called
	mockDB.AssertExpectations(t)
}

func TestDAGExecutor_Kill_NotDistributed(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := Env{
		DAGContext: digraph.DAGContext{
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = WithEnv(context.Background(), env)

	// Create a child DAG without worker selector (local execution)
	childDAG := &digraph.DAG{
		Name: "child-dag",
	}

	// Create child executor without any distributed runs
	child := &ChildDAGExecutor{
		DAG:             childDAG,
		distributedRuns: make(map[string]bool),
		env:             env,
		cmds:            make(map[string]*exec.Cmd),
	}

	// Create a DAG executor
	executor := &dagExecutor{
		child: child,
	}

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was NOT called
	mockDB.AssertNotCalled(t, "RequestChildCancel")
}
