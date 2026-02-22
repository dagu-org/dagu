package agentstep

import (
	"context"
	"io"
	"os"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

// MockExecutorType is a test executor type that simulates a successful agent step.
const MockExecutorType = "mock-agent"

// MockExecutor is a mock implementation for testing agent step message flow.
type MockExecutor struct {
	stdout          io.Writer
	stderr          io.Writer
	contextMessages []exec.LLMMessage
	messages        []exec.LLMMessage
}

var _ executor.Executor = (*MockExecutor)(nil)
var _ executor.ChatMessageHandler = (*MockExecutor)(nil)

// NewMockExecutor creates a new mock agent executor with sample messages.
func NewMockExecutor(_ context.Context, _ core.Step) (executor.Executor, error) {
	return &MockExecutor{
		stdout: os.Stdout,
		stderr: os.Stderr,
		messages: []exec.LLMMessage{
			{Role: exec.RoleUser, Content: "agent input"},
			{
				Role:    exec.RoleAssistant,
				Content: "agent response",
				Metadata: &exec.LLMMessageMetadata{
					Provider:         "openai",
					Model:            "gpt-4",
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
					Cost:             0.001,
				},
			},
		},
	}, nil
}

func (m *MockExecutor) SetStdout(out io.Writer) { m.stdout = out }
func (m *MockExecutor) SetStderr(out io.Writer) { m.stderr = out }
func (m *MockExecutor) Kill(_ os.Signal) error  { return nil }
func (m *MockExecutor) Run(_ context.Context) error {
	_, _ = m.stdout.Write([]byte("mock agent response\n"))
	return nil
}
func (m *MockExecutor) SetContext(msgs []exec.LLMMessage) {
	m.contextMessages = msgs
	m.messages = append(msgs, m.messages...)
}
func (m *MockExecutor) GetMessages() []exec.LLMMessage { return m.messages }

// RegisterMockExecutors registers mock agent executors for testing.
func RegisterMockExecutors() {
	executor.RegisterExecutor(MockExecutorType, NewMockExecutor, nil, core.ExecutorCapabilities{Agent: true})
}
