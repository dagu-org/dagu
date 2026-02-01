package agent

import (
	"context"
	"errors"
	"sync"

	"github.com/dagu-org/dagu/internal/llm"
)

var errProviderNotConfigured = errors.New("provider not configured")

// mockLLMProvider implements llm.Provider for testing.
type mockLLMProvider struct {
	name           string
	chatFunc       func(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error)
	chatStreamFunc func(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error)
}

func (m *mockLLMProvider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.chatFunc != nil {
		return m.chatFunc(ctx, req)
	}
	return &llm.ChatResponse{Content: "mock response", FinishReason: "stop"}, nil
}

func (m *mockLLMProvider) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	if m.chatStreamFunc != nil {
		return m.chatStreamFunc(ctx, req)
	}
	ch := make(chan llm.StreamEvent, 1)
	ch <- llm.StreamEvent{Done: true}
	close(ch)
	return ch, nil
}

func (m *mockLLMProvider) Name() string {
	if m.name == "" {
		return "mock"
	}
	return m.name
}

var _ llm.Provider = (*mockLLMProvider)(nil)

// mockConfigStore implements ConfigStore for testing.
type mockConfigStore struct {
	config   *Config
	enabled  bool
	provider llm.Provider
	model    string
	err      error
}

func newMockConfigStore(enabled bool) *mockConfigStore {
	return &mockConfigStore{
		config:  DefaultConfig(),
		enabled: enabled,
		model:   "test-model",
	}
}

func (m *mockConfigStore) Load(_ context.Context) (*Config, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.config, nil
}

func (m *mockConfigStore) Save(_ context.Context, cfg *Config) error {
	if m.err != nil {
		return m.err
	}
	m.config = cfg
	return nil
}

func (m *mockConfigStore) IsEnabled(_ context.Context) bool {
	return m.enabled
}

func (m *mockConfigStore) GetProvider(_ context.Context) (llm.Provider, string, error) {
	if m.err != nil {
		return nil, "", m.err
	}
	if m.provider == nil {
		return nil, "", errProviderNotConfigured
	}
	return m.provider, m.model, nil
}

var _ ConfigStore = (*mockConfigStore)(nil)

// mockConversationStore implements ConversationStore for testing.
type mockConversationStore struct {
	conversations map[string]*Conversation
	messages      map[string][]Message
	mu            sync.Mutex

	createErr      error
	getErr         error
	listErr        error
	updateErr      error
	deleteErr      error
	addMessageErr  error
	getMessagesErr error
	getSeqIDErr    error
}

func newMockConversationStore() *mockConversationStore {
	return &mockConversationStore{
		conversations: make(map[string]*Conversation),
		messages:      make(map[string][]Message),
	}
}

func (m *mockConversationStore) requireConversation(id string) error {
	if _, exists := m.conversations[id]; !exists {
		return ErrConversationNotFound
	}
	return nil
}

func (m *mockConversationStore) CreateConversation(_ context.Context, conv *Conversation) error {
	if m.createErr != nil {
		return m.createErr
	}
	if conv.ID == "" {
		return ErrInvalidConversationID
	}
	if conv.UserID == "" {
		return ErrInvalidUserID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.conversations[conv.ID] = conv
	m.messages[conv.ID] = []Message{}
	return nil
}

func (m *mockConversationStore) GetConversation(_ context.Context, id string) (*Conversation, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if id == "" {
		return nil, ErrInvalidConversationID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireConversation(id); err != nil {
		return nil, err
	}
	return m.conversations[id], nil
}

func (m *mockConversationStore) ListConversations(_ context.Context, userID string) ([]*Conversation, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*Conversation
	for _, conv := range m.conversations {
		if conv.UserID == userID {
			result = append(result, conv)
		}
	}
	return result, nil
}

func (m *mockConversationStore) UpdateConversation(_ context.Context, conv *Conversation) error {
	if m.updateErr != nil {
		return m.updateErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireConversation(conv.ID); err != nil {
		return err
	}
	m.conversations[conv.ID] = conv
	return nil
}

func (m *mockConversationStore) DeleteConversation(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if id == "" {
		return ErrInvalidConversationID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireConversation(id); err != nil {
		return err
	}
	delete(m.conversations, id)
	delete(m.messages, id)
	return nil
}

func (m *mockConversationStore) AddMessage(_ context.Context, conversationID string, msg *Message) error {
	if m.addMessageErr != nil {
		return m.addMessageErr
	}
	if conversationID == "" {
		return ErrInvalidConversationID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireConversation(conversationID); err != nil {
		return err
	}
	m.messages[conversationID] = append(m.messages[conversationID], *msg)
	return nil
}

func (m *mockConversationStore) GetMessages(_ context.Context, conversationID string) ([]Message, error) {
	if m.getMessagesErr != nil {
		return nil, m.getMessagesErr
	}
	if conversationID == "" {
		return nil, ErrInvalidConversationID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireConversation(conversationID); err != nil {
		return nil, err
	}
	return m.messages[conversationID], nil
}

func (m *mockConversationStore) GetLatestSequenceID(_ context.Context, conversationID string) (int64, error) {
	if m.getSeqIDErr != nil {
		return 0, m.getSeqIDErr
	}
	if conversationID == "" {
		return 0, ErrInvalidConversationID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireConversation(conversationID); err != nil {
		return 0, err
	}

	var maxSeq int64
	for _, msg := range m.messages[conversationID] {
		if msg.SequenceID > maxSeq {
			maxSeq = msg.SequenceID
		}
	}
	return maxSeq, nil
}

// HasConversation checks if a conversation exists without returning an error.
func (m *mockConversationStore) HasConversation(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.conversations[id]
	return exists
}

var _ ConversationStore = (*mockConversationStore)(nil)

// newStopProvider creates a mock provider that returns a simple stop response.
func newStopProvider(content string) *mockLLMProvider {
	return &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			return &llm.ChatResponse{Content: content, FinishReason: "stop"}, nil
		},
	}
}

// newCapturingProvider creates a mock provider that captures requests to a channel.
func newCapturingProvider(requestCh chan<- *llm.ChatRequest, response *llm.ChatResponse) *mockLLMProvider {
	return &mockLLMProvider{
		chatFunc: func(_ context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
			select {
			case requestCh <- req:
			default:
			}
			return response, nil
		},
	}
}

// simpleStopResponse creates a simple stop response for testing.
func simpleStopResponse(content string) *llm.ChatResponse {
	return &llm.ChatResponse{Content: content, FinishReason: "stop"}
}
