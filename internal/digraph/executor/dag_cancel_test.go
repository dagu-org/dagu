package executor

import (
	"context"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDAGExecutor_Kill_Distributed(t *testing.T) {
	// Create a mock database
	mockDB := new(MockDatabase)

	// Create a context with environment
	env := Env{
		Env: digraph.Env{
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = WithEnv(context.Background(), env)

	// Create a DAG executor
	executor := &dagExecutor{
		isDistributed: true,
		childDAGRunID: "child-run-id",
		env:           env,
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
	mockDB := new(MockDatabase)

	// Create a context with environment
	env := Env{
		Env: digraph.Env{
			DB:         mockDB,
			RootDAGRun: digraph.NewDAGRunRef("root-dag", "root-run-id"),
			DAGRunID:   "parent-run-id",
		},
	}
	_ = WithEnv(context.Background(), env)

	// Create a DAG executor that's not distributed
	executor := &dagExecutor{
		isDistributed: false,
		env:           env,
	}

	// Call Kill
	err := executor.Kill(os.Interrupt)

	// Verify no error
	assert.NoError(t, err)

	// Verify RequestChildCancel was NOT called
	mockDB.AssertNotCalled(t, "RequestChildCancel")
}
