// Package fileconversation provides a file-based implementation of the ConversationStore interface.
package fileconversation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
)

const (
	conversationFileExtension   = ".json"
	conversationDirPermissions  = 0750
	conversationFilePermissions = 0600
	maxTitleLength              = 50
)

// ConversationForStorage is the JSON-serializable format for conversations.
type ConversationForStorage struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Title     string          `json:"title,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	Messages  []agent.Message `json:"messages"`
}

// ToConversation converts a ConversationForStorage to an agent.Conversation.
func (c *ConversationForStorage) ToConversation() *agent.Conversation {
	return &agent.Conversation{
		ID:        c.ID,
		UserID:    c.UserID,
		Title:     c.Title,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// FromConversation creates a ConversationForStorage from an agent.Conversation.
func FromConversation(conv *agent.Conversation, messages []agent.Message) *ConversationForStorage {
	return &ConversationForStorage{
		ID:        conv.ID,
		UserID:    conv.UserID,
		Title:     conv.Title,
		CreatedAt: conv.CreatedAt,
		UpdatedAt: conv.UpdatedAt,
		Messages:  messages,
	}
}

// Store implements agent.ConversationStore using the local filesystem.
// Conversations are stored as individual JSON files in user-specific directories.
// Thread-safe through internal locking.
type Store struct {
	baseDir   string
	mu        sync.RWMutex
	byID      map[string]string    // conversation ID -> file path
	byUser    map[string][]string  // user ID -> conversation IDs (sorted by UpdatedAt descending)
	updatedAt map[string]time.Time // conversation ID -> last update time
}

var _ agent.ConversationStore = (*Store)(nil)

// Option is a functional option for configuring the Store.
type Option func(*Store)

// New creates a new file-based conversation store.
// The baseDir must be non-empty; provided Option functions are applied to the store.
// If baseDir does not exist it is created with directory permissions 0750, and an initial
// in-memory index is built from existing conversation files. Returns an error on invalid input,
// failure to create the directory, or failure to build the initial index.
func New(baseDir string, opts ...Option) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("fileconversation: baseDir cannot be empty")
	}

	store := &Store{
		baseDir:   baseDir,
		byID:      make(map[string]string),
		byUser:    make(map[string][]string),
		updatedAt: make(map[string]time.Time),
	}

	for _, opt := range opts {
		opt(store)
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, conversationDirPermissions); err != nil {
		return nil, fmt.Errorf("fileconversation: failed to create directory %s: %w", baseDir, err)
	}

	// Build initial index
	if err := store.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("fileconversation: failed to build index: %w", err)
	}

	return store, nil
}

// rebuildIndex scans the directory and rebuilds the in-memory index.
func (s *Store) rebuildIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.byID = make(map[string]string)
	s.byUser = make(map[string][]string)
	s.updatedAt = make(map[string]time.Time)

	userDirs, err := os.ReadDir(s.baseDir)
	if err != nil {
		return fmt.Errorf("fileconversation: failed to read directory %s: %w", s.baseDir, err)
	}

	for _, userDir := range userDirs {
		if !userDir.IsDir() {
			continue
		}
		s.indexUserDirectory(userDir.Name())
	}

	for userID := range s.byUser {
		s.sortUserConversations(userID)
	}

	return nil
}

// indexUserDirectory indexes all conversations in a user's directory.
// Must be called with mu held.
func (s *Store) indexUserDirectory(userID string) {
	userPath := filepath.Join(s.baseDir, userID)
	entries, err := os.ReadDir(userPath)
	if err != nil {
		slog.Warn("Failed to read user directory during index rebuild",
			slog.String("path", userPath),
			slog.String("error", err.Error()))
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != conversationFileExtension {
			continue
		}

		filePath := filepath.Join(userPath, entry.Name())
		conv, err := s.loadConversationFromFile(filePath)
		if err != nil {
			slog.Warn("Failed to load conversation file during index rebuild",
				slog.String("file", filePath),
				slog.String("error", err.Error()))
			continue
		}

		s.byID[conv.ID] = filePath
		s.byUser[conv.UserID] = append(s.byUser[conv.UserID], conv.ID)
		s.updatedAt[conv.ID] = conv.UpdatedAt
	}
}

// sortUserConversations sorts a user's conversation list by UpdatedAt descending.
// Must be called with mu held.
func (s *Store) sortUserConversations(userID string) {
	convIDs := s.byUser[userID]
	sort.Slice(convIDs, func(i, j int) bool {
		return s.updatedAt[convIDs[i]].After(s.updatedAt[convIDs[j]])
	})
}

// loadConversationFromFile reads and parses a conversation from a JSON file.
func (s *Store) loadConversationFromFile(filePath string) (*ConversationForStorage, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is constructed internally
	if err != nil {
		return nil, fmt.Errorf("fileconversation: failed to read file %s: %w", filePath, err)
	}

	var stored ConversationForStorage
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("fileconversation: failed to parse conversation file %s: %w", filePath, err)
	}

	return &stored, nil
}

// conversationFilePath returns the file path for a conversation.
func (s *Store) conversationFilePath(userID, conversationID string) string {
	return filepath.Join(s.baseDir, userID, conversationID+conversationFileExtension)
}

// userDirPath returns the directory path for a user.
func (s *Store) userDirPath(userID string) string {
	return filepath.Join(s.baseDir, userID)
}

// CreateConversation creates a new conversation.
func (s *Store) CreateConversation(_ context.Context, conv *agent.Conversation) error {
	if err := validateConversation(conv, true); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.byID[conv.ID]; exists {
		return fmt.Errorf("fileconversation: conversation %s already exists", conv.ID)
	}

	userDir := s.userDirPath(conv.UserID)
	if err := os.MkdirAll(userDir, conversationDirPermissions); err != nil {
		return fmt.Errorf("fileconversation: failed to create user directory %s: %w", userDir, err)
	}

	filePath := s.conversationFilePath(conv.UserID, conv.ID)
	if err := s.writeConversationToFile(filePath, FromConversation(conv, nil)); err != nil {
		return err
	}

	s.byID[conv.ID] = filePath
	s.byUser[conv.UserID] = append(s.byUser[conv.UserID], conv.ID)
	s.updatedAt[conv.ID] = conv.UpdatedAt
	s.sortUserConversations(conv.UserID)

	return nil
}

// validateConversation checks that a conversation is valid for storage operations.
func validateConversation(conv *agent.Conversation, requireUserID bool) error {
	if conv == nil {
		return errors.New("fileconversation: conversation cannot be nil")
	}
	if conv.ID == "" {
		return fmt.Errorf("fileconversation: %w", agent.ErrInvalidConversationID)
	}
	if requireUserID && conv.UserID == "" {
		return fmt.Errorf("fileconversation: %w", agent.ErrInvalidUserID)
	}
	return nil
}

// writeConversationToFile writes a conversation to a JSON file atomically.
func (s *Store) writeConversationToFile(filePath string, conv *ConversationForStorage) error {
	if err := fileutil.WriteJSONAtomic(filePath, conv, conversationFilePermissions); err != nil {
		return fmt.Errorf("fileconversation: %w", err)
	}
	return nil
}

// GetConversation retrieves a conversation by ID.
func (s *Store) GetConversation(_ context.Context, id string) (*agent.Conversation, error) {
	if id == "" {
		return nil, agent.ErrInvalidConversationID
	}

	stored, err := s.loadConversationByID(id)
	if err != nil {
		return nil, err
	}

	return stored.ToConversation(), nil
}

// ListConversations returns all conversations for a user, sorted by UpdatedAt descending.
func (s *Store) ListConversations(ctx context.Context, userID string) ([]*agent.Conversation, error) {
	if userID == "" {
		return nil, agent.ErrInvalidUserID
	}

	s.mu.RLock()
	convIDs := make([]string, len(s.byUser[userID]))
	copy(convIDs, s.byUser[userID])
	s.mu.RUnlock()

	conversations := make([]*agent.Conversation, 0, len(convIDs))
	for _, id := range convIDs {
		conv, err := s.GetConversation(ctx, id)
		if err != nil {
			// Skip conversations that can't be loaded
			if errors.Is(err, agent.ErrConversationNotFound) {
				continue
			}
			return nil, err
		}
		conversations = append(conversations, conv)
	}

	return conversations, nil
}

// UpdateConversation updates conversation metadata.
func (s *Store) UpdateConversation(_ context.Context, conv *agent.Conversation) error {
	if err := validateConversation(conv, false); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[conv.ID]
	if !exists {
		return agent.ErrConversationNotFound
	}

	stored, err := s.loadConversationFromFile(filePath)
	if err != nil {
		return fmt.Errorf("fileconversation: failed to load existing conversation: %w", err)
	}

	stored.Title = conv.Title
	stored.UpdatedAt = conv.UpdatedAt

	if err := s.writeConversationToFile(filePath, stored); err != nil {
		return err
	}

	s.updatedAt[conv.ID] = conv.UpdatedAt
	s.sortUserConversations(stored.UserID)

	return nil
}

// DeleteConversation removes a conversation and all its messages.
func (s *Store) DeleteConversation(_ context.Context, id string) error {
	if id == "" {
		return agent.ErrInvalidConversationID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return agent.ErrConversationNotFound
	}

	stored, err := s.loadConversationFromFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileconversation: failed to load conversation for deletion: %w", err)
	}

	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileconversation: failed to delete conversation file: %w", err)
	}

	delete(s.byID, id)
	delete(s.updatedAt, id)

	userID := ""
	if stored != nil {
		userID = stored.UserID
	}
	s.removeConversationFromUserIndex(id, userID)

	return nil
}

// removeConversationFromUserIndex removes a conversation ID from the user index.
// Must be called with mu held.
func (s *Store) removeConversationFromUserIndex(id string, userID string) {
	if userID != "" {
		s.byUser[userID] = removeFromSlice(s.byUser[userID], id)
		return
	}

	for uid, convIDs := range s.byUser {
		if updated := removeFromSlice(convIDs, id); len(updated) != len(convIDs) {
			s.byUser[uid] = updated
			return
		}
	}
}

// removeFromSlice removes the first occurrence of target from slice.
func removeFromSlice(slice []string, target string) []string {
	for i, v := range slice {
		if v == target {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// AddMessage appends a message to a conversation.
func (s *Store) AddMessage(_ context.Context, conversationID string, msg *agent.Message) error {
	if err := validateMessageInput(conversationID, msg); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[conversationID]
	if !exists {
		return agent.ErrConversationNotFound
	}

	stored, err := s.loadConversationFromFile(filePath)
	if err != nil {
		return fmt.Errorf("fileconversation: failed to load conversation: %w", err)
	}

	stored.Messages = append(stored.Messages, *msg)
	stored.UpdatedAt = time.Now()
	stored.setTitleFromMessage(msg)

	if err := s.writeConversationToFile(filePath, stored); err != nil {
		return err
	}

	s.updatedAt[conversationID] = stored.UpdatedAt
	s.sortUserConversations(stored.UserID)

	return nil
}

// validateMessageInput validates the input parameters for AddMessage.
func validateMessageInput(conversationID string, msg *agent.Message) error {
	if conversationID == "" {
		return agent.ErrInvalidConversationID
	}
	if msg == nil {
		return errors.New("fileconversation: message cannot be nil")
	}
	return nil
}

// setTitleFromMessage sets the conversation title from the first user message if not already set.
func (c *ConversationForStorage) setTitleFromMessage(msg *agent.Message) {
	if c.Title == "" && msg.Type == agent.MessageTypeUser && msg.Content != "" {
		c.Title = truncateTitle(msg.Content)
	}
}

// truncateTitle truncates a title to maxTitleLength runes with ellipsis.
func truncateTitle(title string) string {
	runes := []rune(title)
	if len(runes) <= maxTitleLength {
		return title
	}
	if maxTitleLength < 3 {
		return string(runes[:maxTitleLength])
	}
	return string(runes[:maxTitleLength-3]) + "..."
}

// GetMessages retrieves all messages for a conversation.
func (s *Store) GetMessages(_ context.Context, conversationID string) ([]agent.Message, error) {
	if conversationID == "" {
		return nil, agent.ErrInvalidConversationID
	}

	stored, err := s.loadConversationByID(conversationID)
	if err != nil {
		return nil, err
	}

	messages := make([]agent.Message, len(stored.Messages))
	copy(messages, stored.Messages)

	return messages, nil
}

// loadConversationByID looks up and loads a conversation by its ID.
// Note: Uses explicit RUnlock instead of defer since file I/O follows the lock.
// The file could be deleted between index lookup and read, but loadConversationFromFile
// handles os.ErrNotExist gracefully.
func (s *Store) loadConversationByID(id string) (*ConversationForStorage, error) {
	s.mu.RLock()
	filePath, exists := s.byID[id]
	s.mu.RUnlock()

	if !exists {
		return nil, agent.ErrConversationNotFound
	}

	stored, err := s.loadConversationFromFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, agent.ErrConversationNotFound
		}
		return nil, fmt.Errorf("fileconversation: failed to load conversation: %w", err)
	}

	return stored, nil
}

// GetLatestSequenceID returns the highest sequence ID for a conversation.
func (s *Store) GetLatestSequenceID(_ context.Context, conversationID string) (int64, error) {
	if conversationID == "" {
		return 0, agent.ErrInvalidConversationID
	}

	stored, err := s.loadConversationByID(conversationID)
	if err != nil {
		return 0, err
	}

	var maxSeq int64
	for _, msg := range stored.Messages {
		if msg.SequenceID > maxSeq {
			maxSeq = msg.SequenceID
		}
	}

	return maxSeq, nil
}
