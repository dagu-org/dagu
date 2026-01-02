package test

import (
	"context"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/mock"
)

var _ execution.DAGRunAttempt = (*MockDAGRunAttempt)(nil)

// MockDAGRunAttempt is a shared mock implementation of execution.DAGRunAttempt for testing.
type MockDAGRunAttempt struct {
	mock.Mock
	// Status can be set for tests that need to return a specific status without mock setup
	Status *execution.DAGRunStatus
}

func (m *MockDAGRunAttempt) ID() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockDAGRunAttempt) Open(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) Write(ctx context.Context, status execution.DAGRunStatus) error {
	args := m.Called(ctx, status)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) ReadStatus(ctx context.Context) (*execution.DAGRunStatus, error) {
	// If Status is set, return it directly without mock expectations
	if m.Status != nil {
		return m.Status, nil
	}
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*execution.DAGRunStatus), args.Error(1)
}

func (m *MockDAGRunAttempt) ReadDAG(ctx context.Context) (*core.DAG, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *MockDAGRunAttempt) Abort(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) IsAborting(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockDAGRunAttempt) Hide(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) Hidden() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockDAGRunAttempt) WriteOutputs(ctx context.Context, outputs *execution.DAGRunOutputs) error {
	args := m.Called(ctx, outputs)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) ReadOutputs(ctx context.Context) (*execution.DAGRunOutputs, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*execution.DAGRunOutputs), args.Error(1)
}

func (m *MockDAGRunAttempt) WriteMessages(ctx context.Context, messages *execution.LLMMessages) error {
	args := m.Called(ctx, messages)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) ReadMessages(ctx context.Context) (*execution.LLMMessages, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*execution.LLMMessages), args.Error(1)
}
