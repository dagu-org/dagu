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

	// onWorkingChange is called when the working state changes.
	onWorkingChange func(id string, working bool)
}

// ConversationManagerConfig contains configuration for creating a ConversationManager.
type ConversationManagerConfig struct {
	ID              string
	UserID          string
	Logger          *slog.Logger
	WorkingDir      string
	OnWorkingChange func(id string, working bool)
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
	logger = logger.With("conversation_id", id)

	return &ConversationManager{
		id:              id,
		userID:          cfg.UserID,
		lastActivity:    time.Now(),
		logger:          logger,
		subpub:          NewSubPub[StreamResponse](),
		messages:        make([]Message, 0),
		workingDir:      cfg.WorkingDir,
		onWorkingChange: cfg.OnWorkingChange,
	}
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
	onWorkingChange := cm.onWorkingChange
	cm.mu.Unlock()

	cm.logger.Debug("agent working state changed", "working", working)

	// Broadcast state change to subscribers
	cm.subpub.Broadcast(StreamResponse{
		ConversationState: &ConversationState{
			ConversationID: id,
			Working:        working,
			Model:          model,
		},
	})

	// Notify external listener
	if onWorkingChange != nil {
		onWorkingChange(id, working)
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

	// Create user message
	userLLMMessage := llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	}

	cm.mu.Lock()
	cm.lastActivity = time.Now()
	cm.sequenceID++
	seqID := cm.sequenceID
	loopInstance := cm.loop
	cm.mu.Unlock()

	if loopInstance == nil {
		return fmt.Errorf("conversation loop not initialized")
	}

	// Record and broadcast user message
	userMessage := Message{
		ID:             uuid.New().String(),
		ConversationID: cm.id,
		Type:           MessageTypeUser,
		SequenceID:     seqID,
		Content:        content,
		CreatedAt:      time.Now(),
		LLMData:        &userLLMMessage,
	}

	cm.mu.Lock()
	cm.messages = append(cm.messages, userMessage)
	cm.mu.Unlock()

	// Broadcast to subscribers
	cm.subpub.Publish(seqID, StreamResponse{
		Messages: []Message{userMessage},
	})

	// Queue the message for the loop to process
	loopInstance.QueueUserMessage(userLLMMessage)

	// Mark agent as working
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
	logger := cm.logger
	workingDir := cm.workingDir
	conversationID := cm.id
	cm.mu.Unlock()

	// Create tools
	tools := CreateTools()

	// Create loop context
	loopCtx, cancel := context.WithCancel(context.Background())

	// Create record message function
	recordMessage := func(ctx context.Context, msg Message) error {
		msg.ConversationID = conversationID
		if msg.ID == "" {
			msg.ID = uuid.New().String()
		}

		cm.mu.Lock()
		cm.messages = append(cm.messages, msg)
		cm.sequenceID++
		seqID := cm.sequenceID
		cm.mu.Unlock()

		msg.SequenceID = seqID

		// Broadcast to subscribers
		cm.subpub.Publish(seqID, StreamResponse{
			Messages: []Message{msg},
		})

		return nil
	}

	// emitUIAction broadcasts a UI action message to subscribers
	emitUIAction := func(action UIAction) {
		cm.mu.Lock()
		cm.sequenceID++
		seqID := cm.sequenceID
		cm.mu.Unlock()

		msg := Message{
			ID:             fmt.Sprintf("ui-%d", seqID),
			ConversationID: conversationID,
			Type:           MessageTypeUIAction,
			SequenceID:     seqID,
			UIAction:       &action,
			CreatedAt:      time.Now(),
		}

		cm.subpub.Publish(seqID, StreamResponse{
			Messages: []Message{msg},
		})
	}

	loopInstance := NewLoop(LoopConfig{
		Provider:       provider,
		Model:          model,
		History:        nil,
		Tools:          tools,
		RecordMessage:  recordMessage,
		Logger:         logger,
		SystemPrompt:   defaultSystemPrompt(),
		WorkingDir:     workingDir,
		ConversationID: conversationID,
		OnWorking: func(working bool) {
			cm.SetWorking(working)
		},
		EmitUIAction: emitUIAction,
	})

	cm.mu.Lock()
	if cm.loop != nil {
		// Another goroutine created the loop
		cm.mu.Unlock()
		cancel()
		return nil
	}
	cm.loop = loopInstance
	cm.loopCancel = cancel
	cm.loopCtx = loopCtx
	cm.mu.Unlock()

	// Start the loop in a goroutine
	go func() {
		if err := loopInstance.Go(loopCtx); err != nil && err != context.Canceled {
			logger.Error("conversation loop stopped", "error", err)
		}
	}()

	return nil
}

// defaultSystemPrompt returns the default system prompt for the agent.
func defaultSystemPrompt() string {
	return `You are Dagu Agent, an AI assistant that helps users manage DAG workflows.

## Tools
- bash: Execute shell commands
- read: Read file contents
- patch: Create, edit, or delete files
- think: Plan and reason through complex tasks

## DAG File Location
DAGs are stored as YAML files in the DAGs directory. Use 'dagu home' to find the location.

## DAG YAML Structure

### Minimal DAG
steps:
  - command: echo hello

### Complete Structure (all fields optional except steps)
name: string                    # defaults to filename
schedule: "cron-expr"           # or array, or {start, stop, restart}
env:
  - KEY: value
params: "KEY=value KEY2=value2" # or array of maps
tags:
  - name: value
steps: [...]                    # required
handlerOn:                      # lifecycle hooks
  success: {command: ...}
  failure: {command: ...}
  exit: {command: ...}

### Step Fields
- name: step-name              # optional, auto-generated
- command: string|array        # shell command
- script: |                    # multi-line script (alternative to command)
    #!/bin/bash
    echo hello
- depends: [step1, step2]      # dependencies (for type: graph)
- output: VAR_NAME             # capture stdout to variable
- env: [{KEY: value}]          # step-specific env vars
- workingDir: /path            # working directory
- preconditions:               # skip if condition fails
    - condition: "${VAR}"
      expected: "value"
- continueOn:                  # continue on failure
    failure: true
- retryPolicy:                 # retry on failure
    limit: 3
    intervalSec: 10

### Execution Types
type: chain  # (default) sequential - each step auto-depends on previous, no need for 'depends:'
type: graph  # parallel - must use explicit 'depends:', steps without it run immediately

### Docker/Container Execution
Two modes: Image mode (create container) or Exec mode (use existing)

Image Mode:
container:
  image: python:3.11          # required
  volumes: ["./src:/app"]
  env: [KEY=value]
  workingDir: /app
  user: "1000:1000"
  platform: linux/amd64
  ports: ["8080:8080"]
  network: bridge
  shell: ["/bin/bash", "-c"]  # wrap commands with shell
  # DAG-level only:
  startup: keepalive          # keepalive | entrypoint | command
  waitFor: running            # running | healthy
  keepContainer: true

Exec Mode (use existing container):
container: my-container       # string form
container:
  exec: my-container          # object form
  user: root
  workingDir: /app

### Step Types (Executors)

http - API calls:
- type: http
  command: GET https://api.example.com
  config: {headers: {...}, timeout: 30, body: "..."}

ssh - Remote execution:
- type: ssh
  command: ls -la
  config: {user: ubuntu, host: server.com, key: ~/.ssh/id_rsa}

jq - JSON processing:
- type: jq
  command: '.items[] | .name'
  script: '{"items": [...]}'

mail - Send email:
- type: mail
  config: {from: a@b.com, to: [c@d.com], subject: "...", message: "..."}

s3 - S3 operations:
- type: s3
  command: upload  # or download, list, delete
  config: {bucket: my-bucket, key: path/to/obj, source: /local/file}

postgres/sqlite - SQL database:
- type: postgres
  command: SELECT * FROM users WHERE id = $1
  config: {dsn: postgres://..., params: [123]}

redis - Redis operations:
- type: redis
  config: {url: redis://localhost:6379, command: GET, key: mykey}

archive - Archive operations:
- type: archive
  command: extract  # or create, list
  config: {source: /path/to/file.zip, destination: /output}

hitl - Human-in-the-loop approval:
- type: hitl
  config: {prompt: "Approve?", input: [reason], required: [reason]}

chat - LLM conversation:
- type: chat
  config: {provider: openai, model: gpt-4}
  messages: [{role: user, content: "..."}]

gha - GitHub Actions (experimental):
- type: gha
  command: actions/checkout@v4
  params: {fetch-depth: 0}

### Sub-DAGs
- call: sub-workflow           # by name
  params: "INPUT=${value}"
- call: workflows/external.yaml  # by file path

### Embedded Sub-DAGs (same file)
Use '---' to define multiple DAGs. Root DAG: name optional. Sub-DAGs: name required.
steps:
  - call: processor
---
name: processor
steps:
  - command: echo "processing"

### Parallel Items
- call: worker
  parallel:
    items: [a, b, c]
    maxConcurrent: 2
  params: "ITEM=${ITEM}"

### Variables
${PARAM}                       # parameter reference
${ENV_VAR:-default}            # env var with default
` + "`command`" + `                      # command substitution
${OUTPUT.json.path}            # JSON path from output

### Built-in Variables
DAG_NAME, DAG_RUN_ID, DAG_RUN_LOG_FILE, DAG_RUN_STEP_NAME

## Workflow
1. Read existing file first (if editing)
2. Use patch tool to create/modify
3. Validate: dagu dry-run <dag.yaml>
4. Run: dagu start <dag.yaml>

## Important
- YAML indentation matters (2 spaces)
- Use 'dagu dry-run' before confirming changes
- Ask for confirmation before significant changes`
}
