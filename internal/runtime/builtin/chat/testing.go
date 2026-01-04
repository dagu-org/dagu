package chat

import (
	"context"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

// MockExecutorType is a test executor type that simulates a successful chat step.
const MockExecutorType = "mock-chat"

// MockEmptyExecutorType is a test executor type that returns no messages.
const MockEmptyExecutorType = "mock-empty-chat"

// MockExecutor is a mock implementation for testing chat message flow.
type MockExecutor struct {
	stdout   io.Writer
	stderr   io.Writer
	messages []execution.LLMMessage
}

var _ executor.Executor = (*MockExecutor)(nil)
var _ executor.ChatMessageHandler = (*MockExecutor)(nil)

// NewMockExecutor creates a new mock chat executor.
func NewMockExecutor(_ context.Context, _ core.Step) (executor.Executor, error) {
	return &MockExecutor{
		stdout: os.Stdout,
		stderr: os.Stderr,
		messages: []execution.LLMMessage{
			{Role: execution.RoleUser, Content: "test message"},
			{Role: execution.RoleAssistant, Content: "test response"},
		},
	}, nil
}

func (m *MockExecutor) SetStdout(out io.Writer) { m.stdout = out }
func (m *MockExecutor) SetStderr(out io.Writer) { m.stderr = out }
func (m *MockExecutor) Kill(_ os.Signal) error  { return nil }
func (m *MockExecutor) Run(_ context.Context) error {
	_, _ = m.stdout.Write([]byte("mock chat response\n"))
	return nil
}
func (m *MockExecutor) SetContext(msgs []execution.LLMMessage) {
	m.messages = append(msgs, m.messages...)
}
func (m *MockExecutor) GetMessages() []execution.LLMMessage { return m.messages }

// MockEmptyExecutor is a mock implementation that returns no messages.
type MockEmptyExecutor struct{}

var _ executor.Executor = (*MockEmptyExecutor)(nil)
var _ executor.ChatMessageHandler = (*MockEmptyExecutor)(nil)

// NewMockEmptyExecutor creates a mock chat executor that returns no messages.
func NewMockEmptyExecutor(_ context.Context, _ core.Step) (executor.Executor, error) {
	return &MockEmptyExecutor{}, nil
}

func (m *MockEmptyExecutor) SetStdout(_ io.Writer)  {}
func (m *MockEmptyExecutor) SetStderr(_ io.Writer)  {}
func (m *MockEmptyExecutor) Kill(_ os.Signal) error { return nil }
func (m *MockEmptyExecutor) Run(_ context.Context) error {
	return nil
}
func (m *MockEmptyExecutor) SetContext(_ []execution.LLMMessage) {}
func (m *MockEmptyExecutor) GetMessages() []execution.LLMMessage { return nil }

// RegisterMockExecutors registers mock executors for testing.
func RegisterMockExecutors() {
	executor.RegisterExecutor(MockExecutorType, NewMockExecutor, nil, core.ExecutorCapabilities{LLM: true})
	executor.RegisterExecutor(MockEmptyExecutorType, NewMockEmptyExecutor, nil, core.ExecutorCapabilities{LLM: true})
}
