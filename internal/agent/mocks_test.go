package agent

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/llm"
)

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
	config  *Config
	enabled bool
	err     error
}

func newMockConfigStore(enabled bool) *mockConfigStore {
	return &mockConfigStore{
		config:  DefaultConfig(),
		enabled: enabled,
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

var _ ConfigStore = (*mockConfigStore)(nil)

// mockModelStore implements ModelStore for testing.
type mockModelStore struct {
	models map[string]*ModelConfig
	err    error
}

func newMockModelStore() *mockModelStore {
	return &mockModelStore{
		models: make(map[string]*ModelConfig),
	}
}

func (m *mockModelStore) Create(_ context.Context, model *ModelConfig) error {
	if m.err != nil {
		return m.err
	}
	m.models[model.ID] = model
	return nil
}

func (m *mockModelStore) GetByID(_ context.Context, id string) (*ModelConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	model, ok := m.models[id]
	if !ok {
		return nil, ErrModelNotFound
	}
	return model, nil
}

func (m *mockModelStore) List(_ context.Context) ([]*ModelConfig, error) {
	if m.err != nil {
		return nil, m.err
	}
	var result []*ModelConfig
	for _, model := range m.models {
		result = append(result, model)
	}
	return result, nil
}

func (m *mockModelStore) Update(_ context.Context, model *ModelConfig) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.models[model.ID]; !ok {
		return ErrModelNotFound
	}
	m.models[model.ID] = model
	return nil
}

func (m *mockModelStore) Delete(_ context.Context, id string) error {
	if m.err != nil {
		return m.err
	}
	if _, ok := m.models[id]; !ok {
		return ErrModelNotFound
	}
	delete(m.models, id)
	return nil
}

// addModel is a convenience method that adds a model to the store and returns the store
// for chaining in test setup.
func (m *mockModelStore) addModel(model *ModelConfig) *mockModelStore {
	m.models[model.ID] = model
	return m
}

var _ ModelStore = (*mockModelStore)(nil)

// mockSessionStore implements SessionStore for testing.
type mockSessionStore struct {
	sessions map[string]*Session
	messages map[string][]Message
	mu       sync.Mutex

	createErr      error
	getErr         error
	listErr        error
	updateErr      error
	deleteErr      error
	addMessageErr  error
	getMessagesErr error
	getSeqIDErr    error
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]*Session),
		messages: make(map[string][]Message),
	}
}

func (m *mockSessionStore) requireSession(id string) error {
	if _, exists := m.sessions[id]; !exists {
		return ErrSessionNotFound
	}
	return nil
}

func (m *mockSessionStore) CreateSession(_ context.Context, sess *Session) error {
	if m.createErr != nil {
		return m.createErr
	}
	if sess.ID == "" {
		return ErrInvalidSessionID
	}
	if sess.UserID == "" {
		return ErrInvalidUserID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessions[sess.ID] = sess
	m.messages[sess.ID] = []Message{}
	return nil
}

func (m *mockSessionStore) GetSession(_ context.Context, id string) (*Session, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if id == "" {
		return nil, ErrInvalidSessionID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireSession(id); err != nil {
		return nil, err
	}
	return m.sessions[id], nil
}

func (m *mockSessionStore) ListSessions(_ context.Context, userID string) ([]*Session, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	if userID == "" {
		return nil, ErrInvalidUserID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var result []*Session
	for _, sess := range m.sessions {
		if sess.UserID == userID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (m *mockSessionStore) UpdateSession(_ context.Context, sess *Session) error {
	if m.updateErr != nil {
		return m.updateErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireSession(sess.ID); err != nil {
		return err
	}
	m.sessions[sess.ID] = sess
	return nil
}

func (m *mockSessionStore) DeleteSession(_ context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if id == "" {
		return ErrInvalidSessionID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireSession(id); err != nil {
		return err
	}
	delete(m.sessions, id)
	delete(m.messages, id)
	return nil
}

func (m *mockSessionStore) AddMessage(_ context.Context, sessionID string, msg *Message) error {
	if m.addMessageErr != nil {
		return m.addMessageErr
	}
	if sessionID == "" {
		return ErrInvalidSessionID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireSession(sessionID); err != nil {
		return err
	}
	m.messages[sessionID] = append(m.messages[sessionID], *msg)
	return nil
}

func (m *mockSessionStore) GetMessages(_ context.Context, sessionID string) ([]Message, error) {
	if m.getMessagesErr != nil {
		return nil, m.getMessagesErr
	}
	if sessionID == "" {
		return nil, ErrInvalidSessionID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireSession(sessionID); err != nil {
		return nil, err
	}
	return m.messages[sessionID], nil
}

func (m *mockSessionStore) GetLatestSequenceID(_ context.Context, sessionID string) (int64, error) {
	if m.getSeqIDErr != nil {
		return 0, m.getSeqIDErr
	}
	if sessionID == "" {
		return 0, ErrInvalidSessionID
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.requireSession(sessionID); err != nil {
		return 0, err
	}

	var maxSeq int64
	for _, msg := range m.messages[sessionID] {
		if msg.SequenceID > maxSeq {
			maxSeq = msg.SequenceID
		}
	}
	return maxSeq, nil
}

// HasSession checks if a session exists without returning an error.
func (m *mockSessionStore) HasSession(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.sessions[id]
	return exists
}

var _ SessionStore = (*mockSessionStore)(nil)

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

// testModelConfig creates a ModelConfig with sensible defaults for testing.
// The ID, Name, Provider, Model, and APIKey are all pre-filled.
// Override fields after calling if needed (e.g., pricing).
func testModelConfig(id string) *ModelConfig {
	return &ModelConfig{
		ID:       id,
		Name:     "Test " + id,
		Provider: "openai",
		Model:    "gpt-4.1",
		APIKey:   "test-key-" + id,
	}
}

// newDelegateProvider creates a mock provider that returns a delegate tool call
// on the first Chat call and a stop response on subsequent calls.
func newDelegateProvider(task string) *mockLLMProvider {
	var mu sync.Mutex
	callCount := 0
	return &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			mu.Lock()
			callCount++
			n := callCount
			mu.Unlock()
			if n == 1 {
				return &llm.ChatResponse{
					Content:      "",
					FinishReason: "tool_calls",
					ToolCalls: []llm.ToolCall{{
						ID:   "call-1",
						Type: "function",
						Function: llm.ToolCallFunction{
							Name:      "delegate",
							Arguments: fmt.Sprintf(`{"tasks": [{"task": %q}]}`, task),
						},
					}},
				}, nil
			}
			return &llm.ChatResponse{Content: "done", FinishReason: "stop"}, nil
		},
	}
}

// newSequenceProvider creates a mock provider that returns responses in order.
// After all responses are consumed, returns the last one repeatedly.
func newSequenceProvider(responses ...*llm.ChatResponse) *mockLLMProvider {
	var mu sync.Mutex
	idx := 0
	return &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			mu.Lock()
			defer mu.Unlock()
			resp := responses[idx]
			if idx < len(responses)-1 {
				idx++
			}
			return resp, nil
		},
	}
}

// newErrorOnceProvider creates a mock provider that fails on the first call
// and returns the given response on subsequent calls.
func newErrorOnceProvider(response *llm.ChatResponse) *mockLLMProvider {
	var mu sync.Mutex
	callCount := 0
	return &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			mu.Lock()
			callCount++
			n := callCount
			mu.Unlock()
			if n == 1 {
				return nil, fmt.Errorf("transient error")
			}
			return response, nil
		},
	}
}

// newToolCallProvider creates a mock provider that always returns a tool call.
// Useful for testing max depth limits.
func newToolCallProvider(toolName, toolID string) *mockLLMProvider {
	var mu sync.Mutex
	callNum := 0
	return &mockLLMProvider{
		chatFunc: func(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
			mu.Lock()
			callNum++
			id := fmt.Sprintf("%s-%d", toolID, callNum)
			mu.Unlock()
			return &llm.ChatResponse{
				FinishReason: "tool_calls",
				ToolCalls: []llm.ToolCall{{
					ID:   id,
					Type: "function",
					Function: llm.ToolCallFunction{
						Name:      toolName,
						Arguments: `{}`,
					},
				}},
			}, nil
		},
	}
}

// testAPIWithModels creates an API instance pre-configured with the given model configs
// and mock providers already cached. Returns the API and model store for further customization.
func testAPIWithModels(t *testing.T, models ...*ModelConfig) (*API, *mockModelStore) {
	t.Helper()

	configStore := newMockConfigStore(true)
	ms := newMockModelStore()

	for _, m := range models {
		ms.addModel(m)
	}

	if len(models) > 0 {
		configStore.config.DefaultModelID = models[0].ID
	}

	api := NewAPI(APIConfig{
		ConfigStore: configStore,
		ModelStore:  ms,
		WorkingDir:  t.TempDir(),
	})

	for _, m := range models {
		api.providers.Set(m.ToLLMConfig(), &mockLLMProvider{})
	}

	return api, ms
}
