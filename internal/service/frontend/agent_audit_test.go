package frontend

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/service/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgentAuditHook(t *testing.T) {
	t.Parallel()

	store := &mockAuditStore{}
	svc := audit.New(store)
	hook := newAgentAuditHook(svc)

	info := agent.ToolExecInfo{
		ToolName:       "bash",
		Input:          json.RawMessage(`{"command":"echo hello"}`),
		ConversationID: "conv-123",
		UserID:         "user-1",
		Username:       "alice",
		IPAddress:      "192.168.1.1",
		Audit: &agent.AuditInfo{
			Action:          "bash_exec",
			DetailExtractor: agent.ExtractFields("command"),
		},
	}
	result := agent.ToolOut{Content: "hello\n", IsError: false}

	hook(context.Background(), info, result)

	require.Len(t, store.entries, 1)
	entry := store.entries[0]
	assert.Equal(t, audit.CategoryAgent, entry.Category)
	assert.Equal(t, "bash_exec", entry.Action)
	assert.Equal(t, "user-1", entry.UserID)
	assert.Equal(t, "alice", entry.Username)
	assert.Equal(t, "192.168.1.1", entry.IPAddress)

	var details map[string]any
	require.NoError(t, json.Unmarshal([]byte(entry.Details), &details))
	assert.Equal(t, "echo hello", details["command"])
	assert.Equal(t, "conv-123", details["conversation_id"])
	// command output should NOT be in audit details
	assert.NotContains(t, entry.Details, "hello\n")
}

func TestNewAgentAuditHook_FailedAction(t *testing.T) {
	t.Parallel()

	store := &mockAuditStore{}
	svc := audit.New(store)
	hook := newAgentAuditHook(svc)

	info := agent.ToolExecInfo{
		ToolName:       "bash",
		Input:          json.RawMessage(`{"command":"exit 1"}`),
		ConversationID: "conv-456",
		UserID:         "user-2",
		Username:       "bob",
		Audit: &agent.AuditInfo{
			Action:          "bash_exec",
			DetailExtractor: agent.ExtractFields("command"),
		},
	}
	result := agent.ToolOut{Content: "command failed", IsError: true}

	hook(context.Background(), info, result)

	require.Len(t, store.entries, 1)
	var details map[string]any
	require.NoError(t, json.Unmarshal([]byte(store.entries[0].Details), &details))
	assert.Equal(t, true, details["failed"])
}

func TestNewAgentAuditHook_SkipsNonAudited(t *testing.T) {
	t.Parallel()

	store := &mockAuditStore{}
	svc := audit.New(store)
	hook := newAgentAuditHook(svc)

	info := agent.ToolExecInfo{
		ToolName: "think",
		Input:    json.RawMessage(`{"thought":"hmm"}`),
		Audit:    nil, // not audited
	}

	hook(context.Background(), info, agent.ToolOut{})

	assert.Empty(t, store.entries)
}

func TestNewAgentAuditHook_NilDetailExtractor(t *testing.T) {
	t.Parallel()

	store := &mockAuditStore{}
	svc := audit.New(store)
	hook := newAgentAuditHook(svc)

	info := agent.ToolExecInfo{
		ToolName:       "custom_tool",
		Input:          json.RawMessage(`{"key":"value"}`),
		ConversationID: "conv-789",
		UserID:         "user-3",
		Username:       "charlie",
		Audit: &agent.AuditInfo{
			Action:          "custom_action",
			DetailExtractor: nil,
		},
	}

	hook(context.Background(), info, agent.ToolOut{Content: "ok"})

	require.Len(t, store.entries, 1)
	entry := store.entries[0]
	assert.Equal(t, "custom_action", entry.Action)

	var details map[string]any
	require.NoError(t, json.Unmarshal([]byte(entry.Details), &details))
	assert.Equal(t, "conv-789", details["conversation_id"])
	// Only conversation_id should be present (no extracted details)
	assert.Len(t, details, 1)
}

// mockAuditStore is a simple in-memory audit store for testing.
type mockAuditStore struct {
	entries []*audit.Entry
}

func (m *mockAuditStore) Append(_ context.Context, entry *audit.Entry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockAuditStore) Query(_ context.Context, _ audit.QueryFilter) (*audit.QueryResult, error) {
	return &audit.QueryResult{Entries: m.entries, Total: len(m.entries)}, nil
}
