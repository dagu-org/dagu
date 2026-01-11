package audit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	entries []*Entry
}

func (m *mockStore) Append(_ context.Context, entry *Entry) error {
	m.entries = append(m.entries, entry)
	return nil
}

func (m *mockStore) Query(_ context.Context, _ QueryFilter) (*QueryResult, error) {
	return &QueryResult{Entries: m.entries, Total: len(m.entries)}, nil
}

func TestService_Log(t *testing.T) {
	store := &mockStore{}
	svc := New(store)

	entry := NewEntry(CategoryUser, "login", "user-123", "testuser")
	err := svc.Log(context.Background(), entry)

	require.NoError(t, err)
	assert.Len(t, store.entries, 1)
	assert.Equal(t, "login", store.entries[0].Action)
}

func TestService_Query(t *testing.T) {
	store := &mockStore{
		entries: []*Entry{
			NewEntry(CategoryUser, "login", "user-1", "user1"),
			NewEntry(CategoryUser, "logout", "user-2", "user2"),
		},
	}
	svc := New(store)

	result, err := svc.Query(context.Background(), QueryFilter{})

	require.NoError(t, err)
	assert.Len(t, result.Entries, 2)
	assert.Equal(t, 2, result.Total)
}

func TestService_LogTerminalSessionStart(t *testing.T) {
	store := &mockStore{}
	svc := New(store)

	err := svc.LogTerminalSessionStart(context.Background(), "user-123", "testuser", "session-456", "192.168.1.1")

	require.NoError(t, err)
	require.Len(t, store.entries, 1)
	assert.Equal(t, CategoryTerminal, store.entries[0].Category)
	assert.Equal(t, "session_start", store.entries[0].Action)
	assert.Contains(t, store.entries[0].Details, "session-456")
}

func TestService_LogTerminalCommand(t *testing.T) {
	store := &mockStore{}
	svc := New(store)

	err := svc.LogTerminalCommand(context.Background(), "user-123", "testuser", "session-456", "ls -la", "192.168.1.1")

	require.NoError(t, err)
	require.Len(t, store.entries, 1)
	assert.Equal(t, "command", store.entries[0].Action)
	assert.Contains(t, store.entries[0].Details, "ls -la")
}

func TestService_LogTerminalSessionEnd(t *testing.T) {
	store := &mockStore{}
	svc := New(store)

	err := svc.LogTerminalSessionEnd(context.Background(), "user-123", "testuser", "session-456", "closed", "192.168.1.1")

	require.NoError(t, err)
	require.Len(t, store.entries, 1)
	assert.Equal(t, "session_end", store.entries[0].Action)
	assert.Contains(t, store.entries[0].Details, "closed")
}

func TestEscapeJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"simple", "hello", "hello"},
		{"quote", `say "hello"`, `say \"hello\"`},
		{"backslash", `path\to\file`, `path\\to\\file`},
		{"newline", "line1\nline2", `line1\nline2`},
		{"carriage return", "line1\rline2", `line1\rline2`},
		{"tab", "col1\tcol2", `col1\tcol2`},
		{"control char", "hello\x00world", "helloworld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeJSON(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
