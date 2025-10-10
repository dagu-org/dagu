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

func TestChildDAGExecutor_Kill_MixedProcesses(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := digraph.Env{
		DAGContext: digraph.DAGContext{
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = digraph.WithEnv(context.Background(), env)

	// Create a child DAG
	childDAG := &digraph.DAG{
		Name: "child-dag",
	}

	// Create child executor with both local and distributed processes
	executor := &ChildDAGExecutor{
		DAG: childDAG,
		env: env,
		cmds: map[string]*exec.Cmd{
			"local-run-1": &exec.Cmd{Process: &os.Process{Pid: 1234}},
			"local-run-2": &exec.Cmd{Process: &os.Process{Pid: 5678}},
		},
		distributedRuns: map[string]bool{
			"distributed-run-1": true,
			"distributed-run-2": true,
		},
	}

	// Set up expectations for RequestChildCancel
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-1", env.RootDAGRun).Return(nil)
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-2", env.RootDAGRun).Return(nil)

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error (killProcessGroup will fail for fake PIDs but that's expected)
	// We're mainly testing that both distributed and local processes are handled
	assert.Error(t, err) // Expected error from trying to kill fake processes

	// Verify RequestChildCancel was called for both distributed runs
	mockDB.AssertExpectations(t)
}

func TestChildDAGExecutor_Kill_OnlyDistributed(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := digraph.Env{
		DAGContext: digraph.DAGContext{
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = digraph.WithEnv(context.Background(), env)

	// Create a child DAG
	childDAG := &digraph.DAG{
		Name: "child-dag",
	}

	// Create child executor with only distributed processes
	executor := &ChildDAGExecutor{
		DAG:  childDAG,
		env:  env,
		cmds: make(map[string]*exec.Cmd),
		distributedRuns: map[string]bool{
			"distributed-run-1": true,
			"distributed-run-2": true,
		},
	}

	// Set up expectations for RequestChildCancel
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-1", env.RootDAGRun).Return(nil)
	mockDB.On("RequestChildCancel", mock.Anything, "distributed-run-2", env.RootDAGRun).Return(nil)

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was called for both distributed runs
	mockDB.AssertExpectations(t)
}

func TestChildDAGExecutor_Kill_OnlyLocal(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := digraph.Env{
		DAGContext: digraph.DAGContext{
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = digraph.WithEnv(context.Background(), env)

	// Create a child DAG
	childDAG := &digraph.DAG{
		Name: "child-dag",
	}

	// Create child executor with only local processes
	executor := &ChildDAGExecutor{
		DAG: childDAG,
		env: env,
		cmds: map[string]*exec.Cmd{
			"local-run-1": &exec.Cmd{Process: &os.Process{Pid: 1234}},
		},
		distributedRuns: make(map[string]bool),
	}

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify error (from trying to kill fake process)
	assert.Error(t, err)

	// Verify RequestChildCancel was NOT called
	mockDB.AssertNotCalled(t, "RequestChildCancel")
}

func TestChildDAGExecutor_Kill_Empty(t *testing.T) {
	// Create a mock database
	mockDB := new(mockDatabase)

	// Create a context with environment
	env := digraph.Env{
		DAGContext: digraph.DAGContext{
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = digraph.WithEnv(context.Background(), env)

	// Create a child DAG
	childDAG := &digraph.DAG{
		Name: "child-dag",
	}

	// Create child executor with no processes
	executor := &ChildDAGExecutor{
		DAG:             childDAG,
		env:             env,
		cmds:            make(map[string]*exec.Cmd),
		distributedRuns: make(map[string]bool),
	}

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was NOT called
	mockDB.AssertNotCalled(t, "RequestChildCancel")
}
