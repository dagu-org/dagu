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
// Based on Shelley's server/convo.go pattern.
type ConversationManager struct {
	id           string
	userID       string
	loop         *Loop
	loopCancel   context.CancelFunc
	loopCtx      context.Context
	mu           sync.Mutex
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

// copyMessages returns a copy of the messages slice.
func copyMessages(src []Message) []Message {
	if len(src) == 0 {
		return nil
	}
	dst := make([]Message, len(src))
	copy(dst, src)
	return dst
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
	cm.mu.Lock()
	if cm.working == working {
		cm.mu.Unlock()
		return
	}
	cm.working = working
	id := cm.id
	model := cm.model
	callback := cm.onWorkingChange
	cm.mu.Unlock()

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

	cm.mu.Lock()
	cm.lastActivity = time.Now()
	cm.sequenceID++
	seqID := cm.sequenceID

	userMessage := Message{
		ID:             uuid.New().String(),
		ConversationID: cm.id,
		Type:           MessageTypeUser,
		SequenceID:     seqID,
		Content:        content,
		CreatedAt:      time.Now(),
		LLMData:        &userLLMMessage,
	}

	cm.messages = append(cm.messages, userMessage)
	loopInstance := cm.loop
	cm.mu.Unlock()

	if loopInstance == nil {
		return fmt.Errorf("conversation loop not initialized")
	}

	cm.subpub.Publish(seqID, StreamResponse{
		Messages: []Message{userMessage},
	})

	loopInstance.QueueUserMessage(userLLMMessage)
	cm.SetWorking(true)

	return nil
}

// Subscribe returns a function that blocks until the next message is available.
func (cm *ConversationManager) Subscribe(ctx context.Context) func() (StreamResponse, bool) {
	cm.mu.Lock()
	lastSeq := cm.sequenceID
	cm.mu.Unlock()

	return cm.subpub.Subscribe(ctx, lastSeq)
}

// Cancel stops the conversation loop.
func (cm *ConversationManager) Cancel(ctx context.Context) error {
	cm.mu.Lock()
	cancel := cm.loopCancel
	cm.loopCancel = nil
	cm.loopCtx = nil
	cm.loop = nil
	cm.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	cm.SetWorking(false)
	cm.logger.Info("conversation cancelled")
	return nil
}

// Touch updates the last activity timestamp.
func (cm *ConversationManager) Touch() {
	cm.mu.Lock()
	cm.lastActivity = time.Now()
	cm.mu.Unlock()
}

// ensureLoop creates the loop if it doesn't exist.
func (cm *ConversationManager) ensureLoop(provider llm.Provider, model string) error {
	cm.mu.Lock()
	if cm.loop != nil {
		cm.mu.Unlock()
		return nil
	}

	cm.model = model
	llmHistory := cm.extractLLMHistory()
	cm.mu.Unlock()

	tools := CreateTools()
	loopCtx, cancel := context.WithCancel(context.Background())

	loopInstance := NewLoop(LoopConfig{
		Provider:       provider,
		Model:          model,
		History:        llmHistory,
		Tools:          tools,
		RecordMessage:  cm.createRecordMessageFunc(),
		Logger:         cm.logger,
		SystemPrompt:   generateSystemPrompt(cm.environment),
		WorkingDir:     cm.workingDir,
		ConversationID: cm.id,
		OnWorking:      cm.SetWorking,
		EmitUIAction:   cm.createEmitUIActionFunc(),
	})

	cm.mu.Lock()
	if cm.loop != nil {
		cm.mu.Unlock()
		cancel()
		return nil
	}
	cm.loop = loopInstance
	cm.loopCancel = cancel
	cm.loopCtx = loopCtx
	cm.mu.Unlock()

	go func() {
		if err := loopInstance.Go(loopCtx); err != nil && err != context.Canceled {
			cm.logger.Error("conversation loop stopped", "error", err)
		}
	}()

	return nil
}

// extractLLMHistory converts stored messages to LLM format.
// Must be called with cm.mu held.
func (cm *ConversationManager) extractLLMHistory() []llm.Message {
	var history []llm.Message
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

		cm.mu.Lock()
		cm.messages = append(cm.messages, msg)
		cm.sequenceID++
		seqID := cm.sequenceID
		cm.mu.Unlock()

		msg.SequenceID = seqID

		cm.subpub.Publish(seqID, StreamResponse{
			Messages: []Message{msg},
		})

		if cm.onMessage != nil {
			if err := cm.onMessage(ctx, msg); err != nil {
				cm.logger.Warn("Failed to persist message", "error", err)
			}
		}

		return nil
	}
}

// createEmitUIActionFunc returns a function for emitting UI actions.
func (cm *ConversationManager) createEmitUIActionFunc() UIActionFunc {
	return func(action UIAction) {
		cm.mu.Lock()
		cm.sequenceID++
		seqID := cm.sequenceID
		cm.mu.Unlock()

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

// generateSystemPrompt creates the system prompt with environment information.
func generateSystemPrompt(env EnvironmentInfo) string {
	return fmt.Sprintf(`You are Dagu Agent, an AI assistant that helps users manage DAG workflows.

## Environment
- DAGs Directory: %s
- Logs Directory: %s
- Data Directory: %s
- Config File: %s

## Tools
- bash: Execute shell commands
- read: Read file contents
- patch: Create, edit, or delete files
- think: Plan and reason through complex tasks
- get_dag_reference: Get DAG YAML documentation (ALWAYS call before creating/editing DAGs)

## Command
Usage:
  dagu [command]

Available Commands:
  cleanup     Remove old DAG run history
  dequeue     Dequeue a DAG-run from the specified queue
  dry         Simulate a DAG-run without executing actual commands
  enqueue     Enqueue a DAG-run to the queue.
  help        Help about any command
  retry       Retry a previously executed DAG-run with the same run ID
  status      Display the current status of a DAG-run
  stop        Stop a running DAG-run gracefully
  validate    Validate a DAG specification
  version     Display the Dagu version information
  worker      Start a worker that polls the coordinator for tasks

## DAG Writing Guidelines
- No 'name:' field in root level
- Use '---' to define subDAGs
- Check schema for fields and types
- Default is 'type: chain' if not specified, making steps execute sequentially
- Use 'type: graph' for complex DAGs with parallel steps with 'depends:' field
- Use 'call: <subdag_name>' to invoke subDAGs or external DAGs

## DAG Files
DAGs are YAML files. Create new DAGs in the DAGs directory shown above.

## Workflow for DAG Creation/Editing
1. Call get_dag_reference("overview") to understand DAG structure
2. Call get_dag_reference with specific sections as needed (steps, executors, containers, subdags, examples)
3. Read existing file first (if editing)
4. Use patch tool to create/modify
5. Validate: dagu dry-run <dag.yaml>

## Important
- YAML indentation matters (2 spaces)
- Use 'dagu dry-run' before confirming changes
- Ask for confirmation before significant changes`, env.DAGsDir, env.LogDir, env.DataDir, env.ConfigFile)
}
