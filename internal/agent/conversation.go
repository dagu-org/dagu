package agent

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/google/uuid"
)

// ConversationManager manages a single active conversation.
// It links the Loop with SSE streaming and handles state management.
type ConversationManager struct {
	id         string
	userID     string
	loop       *Loop
	loopCancel context.CancelFunc
	mu         sync.Mutex
	lastActivity time.Time
	model        string
	messages     []Message
	subpub       *SubPub[StreamResponse]
	working      bool
	logger       *slog.Logger
	workingDir   string
	sequenceID   int64
	environment  EnvironmentInfo

	// onWorkingChange is called when the working state changes.
	onWorkingChange func(id string, working bool)
	// onMessage is called when a message is recorded, for persistence.
	onMessage func(ctx context.Context, msg Message) error
}

// ConversationManagerConfig contains configuration for creating a ConversationManager.
type ConversationManagerConfig struct {
	ID              string
	UserID          string
	Logger          *slog.Logger
	WorkingDir      string
	OnWorkingChange func(id string, working bool)
	// OnMessage is called when a message is recorded, for persistence.
	OnMessage func(ctx context.Context, msg Message) error
	// History contains restored messages from persistence (for reactivation).
	History []Message
	// SequenceID is the latest sequence ID from persistence (for reactivation).
	SequenceID int64
	// Environment contains Dagu paths for the system prompt.
	Environment EnvironmentInfo
}

// NewConversationManager creates a new ConversationManager.
func NewConversationManager(cfg ConversationManagerConfig) *ConversationManager {
	id := cfg.ID
	if id == "" {
		id = uuid.New().String()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	messages := copyMessages(cfg.History)

	return &ConversationManager{
		id:              id,
		userID:          cfg.UserID,
		lastActivity:    time.Now(),
		logger:          logger.With("conversation_id", id),
		subpub:          NewSubPub[StreamResponse](),
		messages:        messages,
		workingDir:      cfg.WorkingDir,
		onWorkingChange: cfg.OnWorkingChange,
		onMessage:       cfg.OnMessage,
		sequenceID:      cfg.SequenceID,
		environment:     cfg.Environment,
	}
}

// copyMessages returns a shallow copy of the messages slice.
func copyMessages(src []Message) []Message {
	if len(src) == 0 {
		return nil
	}
	return append([]Message(nil), src...)
}

// ID returns the conversation ID.
func (cm *ConversationManager) ID() string {
	return cm.id
}

// UserID returns the user ID that owns this conversation.
func (cm *ConversationManager) UserID() string {
	return cm.userID
}

// SetWorking updates the agent working state and notifies subscribers.
func (cm *ConversationManager) SetWorking(working bool) {
	id, model, callback, changed := cm.updateWorkingState(working)
	if !changed {
		return
	}

	cm.logger.Debug("agent working state changed", "working", working)

	cm.subpub.Broadcast(StreamResponse{
		ConversationState: &ConversationState{
			ConversationID: id,
			Working:        working,
			Model:          model,
		},
	})

	if callback != nil {
		callback(id, working)
	}
}

// updateWorkingState atomically updates the working state and returns relevant data.
// Returns (id, model, callback, changed) where changed indicates if the state actually changed.
func (cm *ConversationManager) updateWorkingState(working bool) (string, string, func(string, bool), bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.working == working {
		return "", "", nil, false
	}

	cm.working = working
	return cm.id, cm.model, cm.onWorkingChange, true
}

// IsWorking returns the current agent working state.
func (cm *ConversationManager) IsWorking() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.working
}

// GetModel returns the model ID used by this conversation.
func (cm *ConversationManager) GetModel() string {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.model
}

// GetMessages returns a copy of all messages in this conversation.
func (cm *ConversationManager) GetMessages() []Message {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	msgs := make([]Message, len(cm.messages))
	copy(msgs, cm.messages)
	return msgs
}

// GetConversation returns the conversation metadata.
func (cm *ConversationManager) GetConversation() Conversation {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return Conversation{
		ID:        cm.id,
		UserID:    cm.userID,
		CreatedAt: cm.lastActivity,
		UpdatedAt: cm.lastActivity,
	}
}

// AcceptUserMessage enqueues a user message, ensuring the loop is ready first.
func (cm *ConversationManager) AcceptUserMessage(ctx context.Context, provider llm.Provider, model string, content string) error {
	if provider == nil {
		return fmt.Errorf("LLM provider is required")
	}

	if err := cm.ensureLoop(provider, model); err != nil {
		return err
	}

	userLLMMessage := llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	}

	userMessage, loopInstance := cm.recordUserMessage(content, &userLLMMessage)

	if loopInstance == nil {
		return fmt.Errorf("conversation loop not initialized")
	}

	cm.subpub.Publish(userMessage.SequenceID, StreamResponse{
		Messages: []Message{userMessage},
	})

	loopInstance.QueueUserMessage(userLLMMessage)
	cm.SetWorking(true)

	return nil
}

// recordUserMessage adds a user message to the conversation and returns it with the loop instance.
func (cm *ConversationManager) recordUserMessage(content string, llmMsg *llm.Message) (Message, *Loop) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.lastActivity = time.Now()
	cm.sequenceID++

	msg := Message{
		ID:             uuid.New().String(),
		ConversationID: cm.id,
		Type:           MessageTypeUser,
		SequenceID:     cm.sequenceID,
		Content:        content,
		CreatedAt:      time.Now(),
		LLMData:        llmMsg,
	}

	cm.messages = append(cm.messages, msg)
	return msg, cm.loop
}

// Subscribe returns a function that blocks until the next message is available.
func (cm *ConversationManager) Subscribe(ctx context.Context) func() (StreamResponse, bool) {
	cm.mu.Lock()
	lastSeq := cm.sequenceID
	cm.mu.Unlock()

	return cm.subpub.Subscribe(ctx, lastSeq)
}

// SubscribeWithSnapshot atomically subscribes and returns the current state.
// This prevents race conditions where messages could be missed between
// getting the initial state and subscribing.
func (cm *ConversationManager) SubscribeWithSnapshot(ctx context.Context) (StreamResponse, func() (StreamResponse, bool)) {
	cm.mu.Lock()
	// Get snapshot while holding lock
	msgs := make([]Message, len(cm.messages))
	copy(msgs, cm.messages)
	lastSeq := cm.sequenceID
	working := cm.working
	model := cm.model
	conv := Conversation{
		ID:        cm.id,
		UserID:    cm.userID,
		CreatedAt: cm.lastActivity,
		UpdatedAt: cm.lastActivity,
	}
	cm.mu.Unlock()

	// Subscribe with the same sequence we captured
	next := cm.subpub.Subscribe(ctx, lastSeq)

	snapshot := StreamResponse{
		Messages:     msgs,
		Conversation: &conv,
		ConversationState: &ConversationState{
			ConversationID: cm.id,
			Working:        working,
			Model:          model,
		},
	}

	return snapshot, next
}

// Cancel stops the conversation loop.
func (cm *ConversationManager) Cancel(_ context.Context) error {
	cancel := cm.clearLoop()

	if cancel != nil {
		cancel()
	}

	cm.SetWorking(false)
	cm.logger.Info("conversation cancelled")
	return nil
}

// clearLoop resets the loop state and returns the cancel function.
func (cm *ConversationManager) clearLoop() context.CancelFunc {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cancel := cm.loopCancel
	cm.loopCancel = nil
	cm.loop = nil
	return cancel
}

// ensureLoop creates the loop if it doesn't exist.
func (cm *ConversationManager) ensureLoop(provider llm.Provider, model string) error {
	history, needsInit := cm.prepareLoopInit(model)
	if !needsInit {
		return nil
	}

	tools := CreateTools()
	loopCtx, cancel := context.WithCancel(context.Background())

	loopInstance := NewLoop(LoopConfig{
		Provider:       provider,
		Model:          model,
		History:        history,
		Tools:          tools,
		RecordMessage:  cm.createRecordMessageFunc(),
		Logger:         cm.logger,
		SystemPrompt:   GenerateSystemPrompt(cm.environment, nil),
		WorkingDir:     cm.workingDir,
		ConversationID: cm.id,
		OnWorking:      cm.SetWorking,
		EmitUIAction:   cm.createEmitUIActionFunc(),
	})

	if !cm.trySetLoop(loopInstance, cancel) {
		cancel()
		return nil
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				cm.logger.Error("conversation loop panicked", "panic", r)
			}
			cm.logger.Info("conversation loop goroutine exiting")
			cm.clearLoop()
		}()
		if err := loopInstance.Go(loopCtx); err != nil {
			if err == context.Canceled {
				cm.logger.Info("conversation loop canceled normally")
			} else {
				cm.logger.Error("conversation loop stopped with error", "error", err)
			}
		}
	}()

	return nil
}

// prepareLoopInit checks if loop initialization is needed and extracts history.
// Returns (history, needsInit) where needsInit is false if loop already exists.
func (cm *ConversationManager) prepareLoopInit(model string) ([]llm.Message, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.loop != nil {
		return nil, false
	}

	cm.model = model
	return cm.extractLLMHistoryLocked(), true
}

// trySetLoop attempts to set the loop instance atomically.
// Returns true if successful, false if another goroutine already set it.
func (cm *ConversationManager) trySetLoop(loop *Loop, cancel context.CancelFunc) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.loop != nil {
		return false
	}

	cm.loop = loop
	cm.loopCancel = cancel
	return true
}

// extractLLMHistoryLocked converts stored messages to LLM format.
// Must be called with cm.mu held.
func (cm *ConversationManager) extractLLMHistoryLocked() []llm.Message {
	history := make([]llm.Message, 0, len(cm.messages))
	for _, msg := range cm.messages {
		if msg.LLMData != nil {
			history = append(history, *msg.LLMData)
		}
	}
	return history
}

// createRecordMessageFunc returns a function for recording messages to the conversation.
func (cm *ConversationManager) createRecordMessageFunc() MessageRecordFunc {
	return func(ctx context.Context, msg Message) error {
		msg.ConversationID = cm.id
		if msg.ID == "" {
			msg.ID = uuid.New().String()
		}

		seqID := cm.appendMessage(msg)
		msg.SequenceID = seqID

		cm.subpub.Publish(seqID, StreamResponse{
			Messages: []Message{msg},
		})

		if cm.onMessage != nil {
			if err := cm.onMessage(ctx, msg); err != nil {
				cm.logger.Warn("failed to persist message", "error", err)
			}
		}

		return nil
	}
}

// appendMessage adds a message to the conversation and returns the new sequence ID.
func (cm *ConversationManager) appendMessage(msg Message) int64 {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.messages = append(cm.messages, msg)
	cm.sequenceID++
	return cm.sequenceID
}

// createEmitUIActionFunc returns a function for emitting UI actions.
func (cm *ConversationManager) createEmitUIActionFunc() UIActionFunc {
	return func(action UIAction) {
		seqID := cm.nextSequenceID()

		cm.subpub.Publish(seqID, StreamResponse{
			Messages: []Message{{
				ID:             fmt.Sprintf("ui-%d", seqID),
				ConversationID: cm.id,
				Type:           MessageTypeUIAction,
				SequenceID:     seqID,
				UIAction:       &action,
				CreatedAt:      time.Now(),
			}},
		})
	}
}

// nextSequenceID increments and returns the next sequence ID.
func (cm *ConversationManager) nextSequenceID() int64 {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.sequenceID++
	return cm.sequenceID
}

