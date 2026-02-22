package agentstep

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
)

func TestExecutor_SetContextAndGetMessages(t *testing.T) {
	t.Parallel()

	e := &Executor{}

	// Initially empty.
	assert.Empty(t, e.GetMessages())

	// SetContext stores messages.
	ctx := []exec.LLMMessage{
		{Role: exec.RoleSystem, Content: "be helpful"},
		{Role: exec.RoleUser, Content: "hello"},
	}
	e.SetContext(ctx)
	assert.Equal(t, ctx, e.contextMessages)

	// GetMessages returns savedMessages, not contextMessages.
	assert.Empty(t, e.GetMessages())

	// Simulate saved messages after execution.
	e.savedMessages = []exec.LLMMessage{
		{Role: exec.RoleUser, Content: "test"},
		{Role: exec.RoleAssistant, Content: "response"},
	}
	assert.Len(t, e.GetMessages(), 2)
	assert.Equal(t, "response", e.GetMessages()[1].Content)
}
