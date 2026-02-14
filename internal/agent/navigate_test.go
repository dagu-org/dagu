package agent

import (
	"encoding/json"
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestNavigateTool_Run(t *testing.T) {
	t.Parallel()

	t.Run("emits UI action", func(t *testing.T) {
		t.Parallel()

		var emitted UIAction
		ctx := ToolContext{
			EmitUIAction: func(action UIAction) {
				emitted = action
			},
		}

		tool := NewNavigateTool()
		input := json.RawMessage(`{"path": "/dags/test-dag"}`)

		result := tool.Run(ctx, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "/dags/test-dag")
		assert.Equal(t, "navigate", emitted.Type)
		assert.Equal(t, "/dags/test-dag", emitted.Path)
	})

	t.Run("empty path returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewNavigateTool()
		input := json.RawMessage(`{"path": ""}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "required")
	})

	t.Run("missing path returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewNavigateTool()
		input := json.RawMessage(`{}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewNavigateTool()
		input := json.RawMessage(`{invalid}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "parse")
	})

	t.Run("works without EmitUIAction callback", func(t *testing.T) {
		t.Parallel()

		tool := NewNavigateTool()
		input := json.RawMessage(`{"path": "/dags"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "/dags")
	})

	t.Run("rejects non-admin navigation to admin-only page", func(t *testing.T) {
		t.Parallel()

		tool := NewNavigateTool()
		input := json.RawMessage(`{"path": "/users"}`)

		result := tool.Run(ToolContext{Role: auth.RoleManager}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "requires admin role")
	})

	t.Run("allows non-admin navigation to non-admin page", func(t *testing.T) {
		t.Parallel()

		tool := NewNavigateTool()
		input := json.RawMessage(`{"path": "/webhooks"}`)

		result := tool.Run(ToolContext{Role: auth.RoleDeveloper}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "/webhooks")
	})
}
