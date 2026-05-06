// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sessionSearchInput(t *testing.T, query string, limit int) json.RawMessage {
	t.Helper()
	input := map[string]any{"query": query}
	if limit > 0 {
		input["limit"] = limit
	}
	b, err := json.Marshal(input)
	require.NoError(t, err)
	return b
}

func seedSearchSession(t *testing.T, store *mockSessionStore, sess *Session, messages ...Message) {
	t.Helper()
	require.NoError(t, store.CreateSession(context.Background(), sess))
	for i := range messages {
		require.NoError(t, store.AddMessage(context.Background(), sess.ID, &messages[i]))
	}
}

func TestSessionSearchTool_Run(t *testing.T) {
	t.Parallel()

	t.Run("searches persisted sessions for current user only", func(t *testing.T) {
		t.Parallel()
		store := newMockSessionStore()
		now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
		seedSearchSession(t, store, &Session{
			ID:        "past-session",
			UserID:    "admin",
			Title:     "Import cleanup",
			DAGName:   "daily-import",
			CreatedAt: now.Add(-2 * time.Hour),
			UpdatedAt: now.Add(-time.Hour),
		}, Message{
			Type:       MessageTypeUser,
			SequenceID: 1,
			Content:    "Can you debug the import failure?",
			CreatedAt:  now.Add(-2 * time.Hour),
		}, Message{
			Type:       MessageTypeAssistant,
			SequenceID: 2,
			Content:    "The fix was to write reports under DAGU_DOCS_DIR.",
			CreatedAt:  now.Add(-time.Hour),
		})
		seedSearchSession(t, store, &Session{
			ID:        "current-session",
			UserID:    "admin",
			CreatedAt: now,
			UpdatedAt: now,
		}, Message{
			Type:       MessageTypeUser,
			SequenceID: 1,
			Content:    "DAGU_DOCS_DIR in the current turn should not be returned.",
			CreatedAt:  now,
		})
		seedSearchSession(t, store, &Session{
			ID:        "other-user-session",
			UserID:    "other-user",
			CreatedAt: now,
			UpdatedAt: now,
		}, Message{
			Type:       MessageTypeUser,
			SequenceID: 1,
			Content:    "Other user also mentioned DAGU_DOCS_DIR.",
			CreatedAt:  now,
		})

		result := NewSessionSearchTool().Run(ToolContext{
			Context:      context.Background(),
			SessionID:    "current-session",
			User:         UserIdentity{UserID: "admin"},
			SessionStore: store,
		}, sessionSearchInput(t, "dagu_docs_dir", 5))

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "past-session")
		assert.Contains(t, result.Content, "daily-import")
		assert.Contains(t, result.Content, "DAGU_DOCS_DIR")
		assert.NotContains(t, result.Content, "current-session")
		assert.NotContains(t, result.Content, "other-user-session")
		assert.NotContains(t, result.Content, "Other user")
	})

	t.Run("requires query", func(t *testing.T) {
		t.Parallel()

		result := NewSessionSearchTool().Run(ToolContext{
			Context:      context.Background(),
			User:         UserIdentity{UserID: "admin"},
			SessionStore: newMockSessionStore(),
		}, sessionSearchInput(t, "   ", 0))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "Query is required")
	})

	t.Run("requires session store", func(t *testing.T) {
		t.Parallel()

		result := NewSessionSearchTool().Run(ToolContext{
			Context: context.Background(),
			User:    UserIdentity{UserID: "admin"},
		}, sessionSearchInput(t, "anything", 0))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "Session store is not available")
	})
}
