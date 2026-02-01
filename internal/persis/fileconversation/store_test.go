package fileconversation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (*Store, context.Context) {
	t.Helper()
	store, err := New(t.TempDir())
	require.NoError(t, err)
	return store, context.Background()
}

func createTestConversation(id, userID string) *agent.Conversation {
	now := time.Now()
	return &agent.Conversation{
		ID:        id,
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func createTestMessage(id, content string, seqID int64, msgType agent.MessageType) *agent.Message {
	return &agent.Message{
		ID:         id,
		Content:    content,
		SequenceID: seqID,
		Type:       msgType,
		CreatedAt:  time.Now(),
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name        string
		baseDir     string
		wantErr     bool
		errContains string
	}{
		{
			name:        "empty baseDir returns error",
			baseDir:     "",
			wantErr:     true,
			errContains: "cannot be empty",
		},
		{
			name:    "valid baseDir creates store",
			baseDir: t.TempDir(),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, err := New(tt.baseDir)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, store)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, store)
			}
		})
	}
}

func TestNew_DirectoryCreationFailure(t *testing.T) {
	// Create a file where directory should be created
	tmpDir := t.TempDir()
	blockingFile := filepath.Join(tmpDir, "blocked")
	err := os.WriteFile(blockingFile, []byte("block"), 0600)
	require.NoError(t, err)

	// Try to create store with path that can't be a directory
	store, err := New(filepath.Join(blockingFile, "subdir"))
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "failed to create directory")
}

func TestNew_WithExistingConversations(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create first store and add data
	store1, err := New(tmpDir)
	require.NoError(t, err)

	conv := createTestConversation("conv1", "user1")
	require.NoError(t, store1.CreateConversation(ctx, conv))

	// Create second store - should rebuild index
	store2, err := New(tmpDir)
	require.NoError(t, err)

	// Verify data is accessible
	retrieved, err := store2.GetConversation(ctx, "conv1")
	require.NoError(t, err)
	assert.Equal(t, "conv1", retrieved.ID)
}

func TestValidateConversation(t *testing.T) {
	tests := []struct {
		name          string
		conv          *agent.Conversation
		requireUserID bool
		wantErr       error
	}{
		{
			name:          "nil conversation",
			conv:          nil,
			requireUserID: false,
			wantErr:       nil, // returns generic error, not sentinel
		},
		{
			name:          "empty ID",
			conv:          &agent.Conversation{},
			requireUserID: false,
			wantErr:       agent.ErrInvalidConversationID,
		},
		{
			name:          "empty userID when required",
			conv:          &agent.Conversation{ID: "123"},
			requireUserID: true,
			wantErr:       agent.ErrInvalidUserID,
		},
		{
			name:          "valid without userID check",
			conv:          &agent.Conversation{ID: "123"},
			requireUserID: false,
			wantErr:       nil,
		},
		{
			name:          "valid with userID",
			conv:          &agent.Conversation{ID: "123", UserID: "user1"},
			requireUserID: true,
			wantErr:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConversation(tt.conv, tt.requireUserID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else if tt.conv == nil {
				assert.Error(t, err) // nil conv returns error but not a sentinel
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStore_CreateConversation(t *testing.T) {
	tests := []struct {
		name    string
		conv    *agent.Conversation
		wantErr error
	}{
		{
			name:    "valid conversation",
			conv:    createTestConversation("conv1", "user1"),
			wantErr: nil,
		},
		{
			name:    "empty ID",
			conv:    &agent.Conversation{UserID: "user1"},
			wantErr: agent.ErrInvalidConversationID,
		},
		{
			name:    "empty userID",
			conv:    &agent.Conversation{ID: "conv1"},
			wantErr: agent.ErrInvalidUserID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)
			err := store.CreateConversation(ctx, tt.conv)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStore_CreateConversation_Duplicate(t *testing.T) {
	store, ctx := setupTestStore(t)

	conv := createTestConversation("conv1", "user1")
	require.NoError(t, store.CreateConversation(ctx, conv))

	// Try to create duplicate
	err := store.CreateConversation(ctx, conv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestStore_GetConversation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Store, context.Context)
		id      string
		wantErr error
	}{
		{
			name: "exists",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
			},
			id:      "conv1",
			wantErr: nil,
		},
		{
			name:    "not found",
			setup:   nil,
			id:      "nonexistent",
			wantErr: agent.ErrConversationNotFound,
		},
		{
			name:    "empty ID",
			setup:   nil,
			id:      "",
			wantErr: agent.ErrInvalidConversationID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)
			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			conv, err := store.GetConversation(ctx, tt.id)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, conv)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, conv)
				assert.Equal(t, tt.id, conv.ID)
			}
		})
	}
}

func TestStore_ListConversations(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*Store, context.Context)
		userID    string
		wantCount int
		wantErr   error
	}{
		{
			name:      "empty userID",
			setup:     nil,
			userID:    "",
			wantCount: 0,
			wantErr:   agent.ErrInvalidUserID,
		},
		{
			name:      "no conversations",
			setup:     nil,
			userID:    "user1",
			wantCount: 0,
			wantErr:   nil,
		},
		{
			name: "multiple conversations",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
				_ = s.CreateConversation(ctx, createTestConversation("conv2", "user1"))
			},
			userID:    "user1",
			wantCount: 2,
			wantErr:   nil,
		},
		{
			name: "user isolation",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
				_ = s.CreateConversation(ctx, createTestConversation("conv2", "user2"))
			},
			userID:    "user1",
			wantCount: 1,
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)
			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			convs, err := store.ListConversations(ctx, tt.userID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Len(t, convs, tt.wantCount)
			}
		})
	}
}

func TestStore_UpdateConversation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Store, context.Context) *agent.Conversation
		update  func(*agent.Conversation)
		wantErr error
	}{
		{
			name: "update title",
			setup: func(s *Store, ctx context.Context) *agent.Conversation {
				conv := createTestConversation("conv1", "user1")
				_ = s.CreateConversation(ctx, conv)
				return conv
			},
			update: func(conv *agent.Conversation) {
				conv.Title = "Updated Title"
				conv.UpdatedAt = time.Now()
			},
			wantErr: nil,
		},
		{
			name:  "not found",
			setup: nil,
			update: func(conv *agent.Conversation) {
				conv.ID = "nonexistent"
			},
			wantErr: agent.ErrConversationNotFound,
		},
		{
			name:  "empty ID",
			setup: nil,
			update: func(conv *agent.Conversation) {
				conv.ID = ""
			},
			wantErr: agent.ErrInvalidConversationID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)

			var conv *agent.Conversation
			if tt.setup != nil {
				conv = tt.setup(store, ctx)
			} else {
				conv = &agent.Conversation{}
			}

			tt.update(conv)
			err := store.UpdateConversation(ctx, conv)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)

				// Verify update persisted
				retrieved, err := store.GetConversation(ctx, conv.ID)
				require.NoError(t, err)
				assert.Equal(t, conv.Title, retrieved.Title)
			}
		})
	}
}

func TestStore_DeleteConversation(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Store, context.Context)
		id      string
		wantErr error
	}{
		{
			name: "exists",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
			},
			id:      "conv1",
			wantErr: nil,
		},
		{
			name:    "not found",
			setup:   nil,
			id:      "nonexistent",
			wantErr: agent.ErrConversationNotFound,
		},
		{
			name:    "empty ID",
			setup:   nil,
			id:      "",
			wantErr: agent.ErrInvalidConversationID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)
			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			err := store.DeleteConversation(ctx, tt.id)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)

				// Verify deletion
				_, err := store.GetConversation(ctx, tt.id)
				assert.ErrorIs(t, err, agent.ErrConversationNotFound)
			}
		})
	}
}

func TestStore_AddMessage(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*Store, context.Context)
		conversationID string
		msg            *agent.Message
		wantErr        bool
		errIs          error
	}{
		{
			name: "valid message",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
			},
			conversationID: "conv1",
			msg:            createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser),
			wantErr:        false,
		},
		{
			name:           "empty convID",
			setup:          nil,
			conversationID: "",
			msg:            createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser),
			wantErr:        true,
			errIs:          agent.ErrInvalidConversationID,
		},
		{
			name:           "conv not found",
			setup:          nil,
			conversationID: "nonexistent",
			msg:            createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser),
			wantErr:        true,
			errIs:          agent.ErrConversationNotFound,
		},
		{
			name: "nil message",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
			},
			conversationID: "conv1",
			msg:            nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)
			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			err := store.AddMessage(ctx, tt.conversationID, tt.msg)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStore_GetMessages(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*Store, context.Context)
		conversationID string
		wantCount      int
		wantErr        error
	}{
		{
			name: "with messages",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
				_ = s.AddMessage(ctx, "conv1", createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser))
				_ = s.AddMessage(ctx, "conv1", createTestMessage("msg2", "Hi", 2, agent.MessageTypeAssistant))
			},
			conversationID: "conv1",
			wantCount:      2,
			wantErr:        nil,
		},
		{
			name: "no messages",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
			},
			conversationID: "conv1",
			wantCount:      0,
			wantErr:        nil,
		},
		{
			name:           "empty convID",
			setup:          nil,
			conversationID: "",
			wantCount:      0,
			wantErr:        agent.ErrInvalidConversationID,
		},
		{
			name:           "not found",
			setup:          nil,
			conversationID: "nonexistent",
			wantCount:      0,
			wantErr:        agent.ErrConversationNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)
			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			msgs, err := store.GetMessages(ctx, tt.conversationID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Len(t, msgs, tt.wantCount)
			}
		})
	}
}

func TestStore_GetLatestSequenceID(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(*Store, context.Context)
		conversationID string
		want           int64
		wantErr        error
	}{
		{
			name: "with messages",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
				_ = s.AddMessage(ctx, "conv1", createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser))
				_ = s.AddMessage(ctx, "conv1", createTestMessage("msg2", "Hi", 5, agent.MessageTypeAssistant))
				_ = s.AddMessage(ctx, "conv1", createTestMessage("msg3", "Ok", 3, agent.MessageTypeUser))
			},
			conversationID: "conv1",
			want:           5,
			wantErr:        nil,
		},
		{
			name: "no messages returns 0",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
			},
			conversationID: "conv1",
			want:           0,
			wantErr:        nil,
		},
		{
			name:           "empty convID",
			setup:          nil,
			conversationID: "",
			want:           0,
			wantErr:        agent.ErrInvalidConversationID,
		},
		{
			name:           "not found",
			setup:          nil,
			conversationID: "nonexistent",
			want:           0,
			wantErr:        agent.ErrConversationNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)
			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			seqID, err := store.GetLatestSequenceID(ctx, tt.conversationID)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, seqID)
			}
		})
	}
}

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "short string",
			input: "Hello",
			want:  "Hello",
		},
		{
			name:  "exactly 50 chars",
			input: strings.Repeat("a", 50),
			want:  strings.Repeat("a", 50),
		},
		{
			name:  "51 chars gets truncated",
			input: strings.Repeat("a", 51),
			want:  strings.Repeat("a", 47) + "...",
		},
		{
			name:  "100 chars",
			input: strings.Repeat("b", 100),
			want:  strings.Repeat("b", 47) + "...",
		},
		{
			name:  "unicode characters",
			input: strings.Repeat("日", 51),
			want:  strings.Repeat("日", 47) + "...",
		},
		{
			name:  "emoji",
			input: strings.Repeat("😀", 51),
			want:  strings.Repeat("😀", 47) + "...",
		},
		{
			name:  "mixed content exceeds limit",
			input: "Hello " + strings.Repeat("世界", 30),
			want:  "", // computed dynamically below
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateTitle(tt.input)

			if tt.name == "mixed content exceeds limit" {
				// For mixed content, verify length constraint and ellipsis
				runes := []rune(got)
				assert.LessOrEqual(t, len(runes), maxTitleLength)
				assert.True(t, strings.HasSuffix(got, "..."))
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRemoveFromSlice(t *testing.T) {
	tests := []struct {
		name   string
		slice  []string
		target string
		want   []string
	}{
		{
			name:   "empty slice",
			slice:  []string{},
			target: "a",
			want:   []string{},
		},
		{
			name:   "not found",
			slice:  []string{"a", "b", "c"},
			target: "d",
			want:   []string{"a", "b", "c"},
		},
		{
			name:   "remove first",
			slice:  []string{"a", "b", "c"},
			target: "a",
			want:   []string{"b", "c"},
		},
		{
			name:   "remove middle",
			slice:  []string{"a", "b", "c"},
			target: "b",
			want:   []string{"a", "c"},
		},
		{
			name:   "remove last",
			slice:  []string{"a", "b", "c"},
			target: "c",
			want:   []string{"a", "b"},
		},
		{
			name:   "single element found",
			slice:  []string{"a"},
			target: "a",
			want:   []string{},
		},
		{
			name:   "single element not found",
			slice:  []string{"a"},
			target: "b",
			want:   []string{"a"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeFromSlice(tt.slice, tt.target)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConversationForStorage_SetTitleFromMessage(t *testing.T) {
	tests := []struct {
		name          string
		initialTitle  string
		msg           *agent.Message
		expectedTitle string
	}{
		{
			name:          "sets title from first user message",
			initialTitle:  "",
			msg:           createTestMessage("msg1", "Hello world", 1, agent.MessageTypeUser),
			expectedTitle: "Hello world",
		},
		{
			name:          "does not override existing title",
			initialTitle:  "Existing Title",
			msg:           createTestMessage("msg1", "New content", 1, agent.MessageTypeUser),
			expectedTitle: "Existing Title",
		},
		{
			name:          "ignores assistant messages",
			initialTitle:  "",
			msg:           createTestMessage("msg1", "Assistant response", 1, agent.MessageTypeAssistant),
			expectedTitle: "",
		},
		{
			name:          "ignores empty content",
			initialTitle:  "",
			msg:           createTestMessage("msg1", "", 1, agent.MessageTypeUser),
			expectedTitle: "",
		},
		{
			name:          "truncates long title",
			initialTitle:  "",
			msg:           createTestMessage("msg1", strings.Repeat("x", 100), 1, agent.MessageTypeUser),
			expectedTitle: strings.Repeat("x", 47) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv := &ConversationForStorage{
				ID:     "test",
				Title:  tt.initialTitle,
				UserID: "user1",
			}

			conv.setTitleFromMessage(tt.msg)

			assert.Equal(t, tt.expectedTitle, conv.Title)
		})
	}
}

func TestStore_RemoveConversationFromUserIndex(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(*Store, context.Context)
		id     string
		userID string
		verify func(*testing.T, *Store)
	}{
		{
			name: "remove with known userID",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
			},
			id:     "conv1",
			userID: "user1",
			verify: func(t *testing.T, s *Store) {
				s.mu.RLock()
				defer s.mu.RUnlock()
				assert.NotContains(t, s.byUser["user1"], "conv1")
			},
		},
		{
			name: "remove with unknown userID searches all users",
			setup: func(s *Store, ctx context.Context) {
				_ = s.CreateConversation(ctx, createTestConversation("conv1", "user1"))
			},
			id:     "conv1",
			userID: "", // unknown
			verify: func(t *testing.T, s *Store) {
				s.mu.RLock()
				defer s.mu.RUnlock()
				assert.NotContains(t, s.byUser["user1"], "conv1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)
			if tt.setup != nil {
				tt.setup(store, ctx)
			}

			store.mu.Lock()
			store.removeConversationFromUserIndex(tt.id, tt.userID)
			store.mu.Unlock()

			tt.verify(t, store)
		})
	}
}

func TestStore_SortUserConversations(t *testing.T) {
	store, ctx := setupTestStore(t)

	now := time.Now()
	convs := []*agent.Conversation{
		{ID: "conv1", UserID: "user1", CreatedAt: now, UpdatedAt: now},
		{ID: "conv2", UserID: "user1", CreatedAt: now, UpdatedAt: now.Add(time.Hour)},
		{ID: "conv3", UserID: "user1", CreatedAt: now, UpdatedAt: now.Add(-time.Hour)},
	}

	for _, conv := range convs {
		require.NoError(t, store.CreateConversation(ctx, conv))
	}

	// Force re-sort
	store.mu.Lock()
	store.sortUserConversations("user1")
	store.mu.Unlock()

	// Verify order (most recent first)
	store.mu.RLock()
	ids := store.byUser["user1"]
	store.mu.RUnlock()

	assert.Equal(t, "conv2", ids[0]) // newest
	assert.Equal(t, "conv1", ids[1]) // middle
	assert.Equal(t, "conv3", ids[2]) // oldest
}

func TestStore_LoadConversationFromFile_Errors(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(string) string // returns file path
		wantErr     bool
		errContains string
	}{
		{
			name: "valid JSON",
			setup: func(dir string) string {
				path := filepath.Join(dir, "valid.json")
				conv := &ConversationForStorage{ID: "test", UserID: "user1"}
				data, _ := json.Marshal(conv)
				_ = os.WriteFile(path, data, 0600)
				return path
			},
			wantErr: false,
		},
		{
			name: "invalid JSON",
			setup: func(dir string) string {
				path := filepath.Join(dir, "invalid.json")
				_ = os.WriteFile(path, []byte("{invalid}"), 0600)
				return path
			},
			wantErr:     true,
			errContains: "failed to parse",
		},
		{
			name: "file not found",
			setup: func(dir string) string {
				return filepath.Join(dir, "nonexistent.json")
			},
			wantErr:     true,
			errContains: "failed to read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			store, _ := New(tmpDir)
			filePath := tt.setup(tmpDir)

			conv, err := store.loadConversationFromFile(filePath)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				assert.Nil(t, conv)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, conv)
			}
		})
	}
}

func TestStore_WriteConversationToFile_Errors(t *testing.T) {
	t.Run("write to read-only directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := New(tmpDir)
		require.NoError(t, err)

		// Create a read-only subdirectory
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		require.NoError(t, os.MkdirAll(readOnlyDir, 0500))
		defer func() { _ = os.Chmod(readOnlyDir, 0750) }()

		conv := &ConversationForStorage{ID: "test", UserID: "user1"}
		filePath := filepath.Join(readOnlyDir, "test.json")

		err = store.writeConversationToFile(filePath, conv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create temp file")
	})

	t.Run("rename blocked by existing directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		store, err := New(tmpDir)
		require.NoError(t, err)

		// Create a directory with the target name
		targetPath := filepath.Join(tmpDir, "blocked.json")
		require.NoError(t, os.MkdirAll(targetPath, 0750))

		conv := &ConversationForStorage{ID: "test", UserID: "user1"}

		err = store.writeConversationToFile(targetPath, conv)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to rename")
	})
}

func TestStore_IndexUserDirectory_Errors(t *testing.T) {
	t.Run("invalid JSON files are skipped", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create user dir with invalid file
		userDir := filepath.Join(tmpDir, "user1")
		require.NoError(t, os.MkdirAll(userDir, 0750))
		invalidFile := filepath.Join(userDir, "invalid.json")
		require.NoError(t, os.WriteFile(invalidFile, []byte("{bad}"), 0600))

		// Create valid file
		validConv := &ConversationForStorage{ID: "valid", UserID: "user1"}
		validData, _ := json.Marshal(validConv)
		validFile := filepath.Join(userDir, "valid.json")
		require.NoError(t, os.WriteFile(validFile, validData, 0600))

		store, err := New(tmpDir)
		require.NoError(t, err)

		// Should have only indexed the valid file
		store.mu.RLock()
		_, exists := store.byID["valid"]
		store.mu.RUnlock()
		assert.True(t, exists)
	})

	t.Run("non-json files are skipped", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create user dir with non-json file
		userDir := filepath.Join(tmpDir, "user1")
		require.NoError(t, os.MkdirAll(userDir, 0750))
		require.NoError(t, os.WriteFile(filepath.Join(userDir, "readme.txt"), []byte("text"), 0600))

		store, err := New(tmpDir)
		require.NoError(t, err)

		store.mu.RLock()
		count := len(store.byID)
		store.mu.RUnlock()
		assert.Equal(t, 0, count)
	})

	t.Run("subdirectories in user dir are skipped", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create user dir with subdirectory
		userDir := filepath.Join(tmpDir, "user1")
		require.NoError(t, os.MkdirAll(filepath.Join(userDir, "subdir"), 0750))

		store, err := New(tmpDir)
		require.NoError(t, err)

		store.mu.RLock()
		count := len(store.byID)
		store.mu.RUnlock()
		assert.Equal(t, 0, count)
	})
}

func TestConversationForStorage_ToConversation(t *testing.T) {
	now := time.Now()
	stored := &ConversationForStorage{
		ID:        "test-id",
		UserID:    "user-id",
		Title:     "Test Title",
		CreatedAt: now,
		UpdatedAt: now,
		Messages:  []agent.Message{{ID: "msg1"}},
	}

	conv := stored.ToConversation()

	assert.Equal(t, stored.ID, conv.ID)
	assert.Equal(t, stored.UserID, conv.UserID)
	assert.Equal(t, stored.Title, conv.Title)
	assert.Equal(t, stored.CreatedAt, conv.CreatedAt)
	assert.Equal(t, stored.UpdatedAt, conv.UpdatedAt)
}

func TestFromConversation(t *testing.T) {
	now := time.Now()
	conv := &agent.Conversation{
		ID:        "test-id",
		UserID:    "user-id",
		Title:     "Test Title",
		CreatedAt: now,
		UpdatedAt: now,
	}
	msgs := []agent.Message{{ID: "msg1"}, {ID: "msg2"}}

	stored := FromConversation(conv, msgs)

	assert.Equal(t, conv.ID, stored.ID)
	assert.Equal(t, conv.UserID, stored.UserID)
	assert.Equal(t, conv.Title, stored.Title)
	assert.Equal(t, conv.CreatedAt, stored.CreatedAt)
	assert.Equal(t, conv.UpdatedAt, stored.UpdatedAt)
	assert.Len(t, stored.Messages, 2)
}

func TestStore_RebuildIndex(t *testing.T) {
	tmpDir := t.TempDir()
	ctx := context.Background()

	// Create store and add data
	store1, err := New(tmpDir)
	require.NoError(t, err)

	conv := createTestConversation("conv1", "user1")
	require.NoError(t, store1.CreateConversation(ctx, conv))

	msg := createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser)
	require.NoError(t, store1.AddMessage(ctx, conv.ID, msg))

	// Simulate restart
	store2, err := New(tmpDir)
	require.NoError(t, err)

	// Verify data persisted
	retrieved, err := store2.GetConversation(ctx, "conv1")
	require.NoError(t, err)
	assert.Equal(t, "conv1", retrieved.ID)
	assert.Equal(t, "Hello", retrieved.Title) // auto-generated from message

	msgs, err := store2.GetMessages(ctx, "conv1")
	require.NoError(t, err)
	assert.Len(t, msgs, 1)

	convs, err := store2.ListConversations(ctx, "user1")
	require.NoError(t, err)
	assert.Len(t, convs, 1)
}

func TestStore_LoadConversationByID_FileDeleted(t *testing.T) {
	store, ctx := setupTestStore(t)

	// Create conversation
	conv := createTestConversation("conv1", "user1")
	require.NoError(t, store.CreateConversation(ctx, conv))

	// Delete the file directly (simulating external deletion)
	store.mu.RLock()
	filePath := store.byID["conv1"]
	store.mu.RUnlock()
	require.NoError(t, os.Remove(filePath))

	// Try to load - should return ErrConversationNotFound
	_, err := store.GetConversation(ctx, "conv1")
	assert.ErrorIs(t, err, agent.ErrConversationNotFound)
}

func TestStore_ListConversations_SkipsDeletedFiles(t *testing.T) {
	store, ctx := setupTestStore(t)

	// Create two conversations
	require.NoError(t, store.CreateConversation(ctx, createTestConversation("conv1", "user1")))
	require.NoError(t, store.CreateConversation(ctx, createTestConversation("conv2", "user1")))

	// Delete one file externally
	store.mu.RLock()
	filePath := store.byID["conv1"]
	store.mu.RUnlock()
	require.NoError(t, os.Remove(filePath))

	// List should skip the deleted conversation
	convs, err := store.ListConversations(ctx, "user1")
	require.NoError(t, err)
	assert.Len(t, convs, 1)
	assert.Equal(t, "conv2", convs[0].ID)
}

func TestStore_CreateConversation_UserDirCreationError(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := New(tmpDir)
	require.NoError(t, err)

	// Create a file where user directory should be
	userPath := filepath.Join(tmpDir, "user1")
	require.NoError(t, os.WriteFile(userPath, []byte("block"), 0600))

	conv := createTestConversation("conv1", "user1")
	err = store.CreateConversation(context.Background(), conv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create user directory")
}

func TestStore_DeleteConversation_FileAlreadyDeleted(t *testing.T) {
	store, ctx := setupTestStore(t)

	// Create conversation
	conv := createTestConversation("conv1", "user1")
	require.NoError(t, store.CreateConversation(ctx, conv))

	// Delete file externally first
	store.mu.RLock()
	filePath := store.byID["conv1"]
	store.mu.RUnlock()
	require.NoError(t, os.Remove(filePath))

	// DeleteConversation should still work (clean up index)
	err := store.DeleteConversation(ctx, "conv1")
	require.NoError(t, err)

	// Verify removed from index
	_, err = store.GetConversation(ctx, "conv1")
	assert.ErrorIs(t, err, agent.ErrConversationNotFound)
}

func TestStore_RebuildIndex_NonDirectorySkipped(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file in base dir (should be skipped, not a user directory)
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("text"), 0600))

	store, err := New(tmpDir)
	require.NoError(t, err)

	store.mu.RLock()
	count := len(store.byID)
	store.mu.RUnlock()
	assert.Equal(t, 0, count)
}

func TestStore_Options(t *testing.T) {
	optionCalled := false
	testOption := func(_ *Store) {
		optionCalled = true
	}

	_, err := New(t.TempDir(), testOption)
	require.NoError(t, err)
	assert.True(t, optionCalled)
}

func TestStore_CorruptedFileErrors(t *testing.T) {
	tests := []struct {
		name      string
		operation func(*Store, context.Context, *agent.Conversation) error
	}{
		{
			name: "UpdateConversation",
			operation: func(s *Store, ctx context.Context, conv *agent.Conversation) error {
				conv.Title = "New Title"
				return s.UpdateConversation(ctx, conv)
			},
		},
		{
			name: "AddMessage",
			operation: func(s *Store, ctx context.Context, _ *agent.Conversation) error {
				msg := createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser)
				return s.AddMessage(ctx, "conv1", msg)
			},
		},
		{
			name: "GetMessages",
			operation: func(s *Store, ctx context.Context, _ *agent.Conversation) error {
				_, err := s.GetMessages(ctx, "conv1")
				return err
			},
		},
		{
			name: "DeleteConversation",
			operation: func(s *Store, ctx context.Context, _ *agent.Conversation) error {
				return s.DeleteConversation(ctx, "conv1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, ctx := setupTestStore(t)

			conv := createTestConversation("conv1", "user1")
			require.NoError(t, store.CreateConversation(ctx, conv))

			// Corrupt the file
			store.mu.RLock()
			filePath := store.byID["conv1"]
			store.mu.RUnlock()
			require.NoError(t, os.WriteFile(filePath, []byte("{invalid}"), 0600))

			err := tt.operation(store, ctx, conv)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to load")
		})
	}
}

func TestValidateMessageInput(t *testing.T) {
	tests := []struct {
		name           string
		conversationID string
		msg            *agent.Message
		wantErr        bool
		errIs          error
	}{
		{
			name:           "valid",
			conversationID: "conv1",
			msg:            createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser),
			wantErr:        false,
		},
		{
			name:           "empty conversation ID",
			conversationID: "",
			msg:            createTestMessage("msg1", "Hello", 1, agent.MessageTypeUser),
			wantErr:        true,
			errIs:          agent.ErrInvalidConversationID,
		},
		{
			name:           "nil message",
			conversationID: "conv1",
			msg:            nil,
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateMessageInput(tt.conversationID, tt.msg)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
