package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/google/uuid"
)

// SessionManager manages a single active session.
// It links the Loop with SSE streaming and handles state management.
type SessionManager struct {
	id              string
	user            UserIdentity
	loop            *Loop
	loopCancel      context.CancelFunc
	mu              sync.Mutex
	createdAt       time.Time
	lastActivity    time.Time
	lastHeartbeat   time.Time
	model           string
	messages        []Message
	subpub          *SubPub[StreamResponse]
	working         bool
	logger          *slog.Logger
	workingDir      string
	sequenceID      int64
	environment     EnvironmentInfo
	safeMode        bool
	hooks           *Hooks
	onWorkingChange func(id string, working bool)
	onMessage       func(ctx context.Context, msg Message) error
	pendingPrompts  map[string]chan UserPromptResponse
	promptsMu       sync.Mutex
	inputCostPer1M  float64
	outputCostPer1M float64
	totalCost       float64
	memoryStore     MemoryStore
	skillStore      SkillStore
	enabledSkills   []string
	dagName         string
	sessionStore    SessionStore
	parentSessionID string
	delegateTask    string
	registry        SubSessionRegistry
	delegates       map[string]DelegateSnapshot // guarded by mu
}

// SessionManagerConfig contains configuration for creating a SessionManager.
type SessionManagerConfig struct {
	ID              string
	User            UserIdentity
	Logger          *slog.Logger
	WorkingDir      string
	OnWorkingChange func(id string, working bool)
	OnMessage       func(ctx context.Context, msg Message) error
	History         []Message
	SequenceID      int64
	Environment     EnvironmentInfo
	SafeMode        bool
	Hooks           *Hooks
	InputCostPer1M  float64
	OutputCostPer1M float64
	MemoryStore     MemoryStore
	SkillStore      SkillStore
	EnabledSkills   []string
	DAGName         string
	SessionStore    SessionStore
	// ParentSessionID links this session to its parent (non-empty = sub-session).
	ParentSessionID string
	// DelegateTask is the task description given to the sub-agent.
	DelegateTask string
	// Registry manages sub-session lifecycle for delegate tools.
	Registry SubSessionRegistry
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager(cfg SessionManagerConfig) *SessionManager {
	id := cfg.ID
	if id == "" {
		id = uuid.New().String()
	}

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	messages := copyMessages(cfg.History)

	now := time.Now()
	return &SessionManager{
		id:              id,
		user:            cfg.User,
		createdAt:       now,
		lastActivity:    now,
		logger:          logger.With("session_id", id),
		subpub:          NewSubPub[StreamResponse](),
		messages:        messages,
		workingDir:      cfg.WorkingDir,
		onWorkingChange: cfg.OnWorkingChange,
		onMessage:       cfg.OnMessage,
		sequenceID:      cfg.SequenceID,
		environment:     cfg.Environment,
		safeMode:        cfg.SafeMode,
		hooks:           cfg.Hooks,
		pendingPrompts:  make(map[string]chan UserPromptResponse),
		delegates:       make(map[string]DelegateSnapshot),
		inputCostPer1M:  cfg.InputCostPer1M,
		outputCostPer1M: cfg.OutputCostPer1M,
		memoryStore:     cfg.MemoryStore,
		skillStore:      cfg.SkillStore,
		enabledSkills:   cfg.EnabledSkills,
		dagName:         cfg.DAGName,
		sessionStore:    cfg.SessionStore,
		parentSessionID: cfg.ParentSessionID,
		delegateTask:    cfg.DelegateTask,
		registry:        cfg.Registry,
	}
}

// UpdateUserContext updates user metadata for an existing session and active loop.
func (sm *SessionManager) UpdateUserContext(u UserIdentity) {
	sm.mu.Lock()
	sm.user = u
	loop := sm.loop
	sm.mu.Unlock()

	if loop != nil {
		loop.SetUserContext(u)
	}
}

// SetSafeMode updates the safe mode setting for this session.
func (sm *SessionManager) SetSafeMode(enabled bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.safeMode = enabled
	// Update active loop if one exists
	if sm.loop != nil {
		sm.loop.SetSafeMode(enabled)
	}
}

// copyMessages returns a shallow copy of the messages slice.
func copyMessages(src []Message) []Message {
	if len(src) == 0 {
		return nil
	}
	return append([]Message(nil), src...)
}

// ID returns the session ID.
func (sm *SessionManager) ID() string {
	return sm.id
}

// UserID returns the user ID that owns this session.
func (sm *SessionManager) UserID() string {
	return sm.user.UserID
}

// SetWorking updates the agent working state and notifies subscribers.
func (sm *SessionManager) SetWorking(working bool) {
	id, model, totalCost, callback, changed := sm.updateWorkingState(working)
	if !changed {
		return
	}
	sm.logger.Debug("agent working state changed", "working", working)
	sm.subpub.Broadcast(StreamResponse{
		SessionState: &SessionState{
			SessionID: id,
			Working:   working,
			Model:     model,
			TotalCost: totalCost,
		},
	})
	if callback != nil {
		callback(id, working)
	}
}

// updateWorkingState atomically updates the working state and returns relevant data.
// Returns (id, model, totalCost, callback, changed) where changed indicates if the state actually changed.
func (sm *SessionManager) updateWorkingState(working bool) (string, string, float64, func(string, bool), bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.working == working {
		return "", "", 0, nil, false
	}

	sm.working = working
	return sm.id, sm.model, sm.totalCost, sm.onWorkingChange, true
}

// IsWorking returns the current agent working state.
func (sm *SessionManager) IsWorking() bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.working
}

// LastActivity returns the time of the most recent activity in this session.
func (sm *SessionManager) LastActivity() time.Time {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.lastActivity
}

// HasPendingPrompt returns true if the session has pending user prompts.
func (sm *SessionManager) HasPendingPrompt() bool {
	sm.promptsMu.Lock()
	defer sm.promptsMu.Unlock()
	return len(sm.pendingPrompts) > 0
}

// RecordHeartbeat updates the heartbeat and activity timestamps.
func (sm *SessionManager) RecordHeartbeat() {
	sm.mu.Lock()
	now := time.Now()
	sm.lastHeartbeat = now
	sm.lastActivity = now
	sm.mu.Unlock()
}

// LastHeartbeat returns the time of the most recent heartbeat.
func (sm *SessionManager) LastHeartbeat() time.Time {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.lastHeartbeat
}

// GetModel returns the model ID used by this session.
func (sm *SessionManager) GetModel() string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.model
}

// GetMessages returns a copy of all messages in this session.
func (sm *SessionManager) GetMessages() []Message {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	msgs := make([]Message, len(sm.messages))
	copy(msgs, sm.messages)
	return msgs
}

// GetSession returns the session metadata.
func (sm *SessionManager) GetSession() Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return Session{
		ID:              sm.id,
		UserID:          sm.user.UserID,
		DAGName:         sm.dagName,
		ParentSessionID: sm.parentSessionID,
		DelegateTask:    sm.delegateTask,
		CreatedAt:       sm.createdAt,
		UpdatedAt:       sm.lastActivity,
	}
}

// RecordExternalMessage records a message from an external source (e.g., a delegate child loop).
// It publishes the message to the SubPub for SSE streaming and persists it via onMessage.
func (sm *SessionManager) RecordExternalMessage(ctx context.Context, msg Message) error {
	msg.SessionID = sm.id
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	sm.mu.Lock()
	sm.lastActivity = time.Now()
	sm.mu.Unlock()

	msg.SequenceID = sm.appendMessage(msg)

	sm.subpub.Publish(msg.SequenceID, StreamResponse{
		Messages: []Message{msg},
	})

	if sm.onMessage != nil {
		if err := sm.onMessage(ctx, msg); err != nil {
			sm.logger.Warn("failed to persist message", "error", err)
			return err
		}
	}

	return nil
}

// AcceptUserMessage enqueues a user message, ensuring the loop is ready first.
func (sm *SessionManager) AcceptUserMessage(ctx context.Context, provider llm.Provider, modelID string, resolvedModel string, content string) error {
	if provider == nil {
		return errors.New("LLM provider is required")
	}

	if err := sm.ensureLoop(provider, modelID, resolvedModel); err != nil {
		return err
	}

	llmMsg := llm.Message{
		Role:    llm.RoleUser,
		Content: content,
	}

	userMessage, loopInstance := sm.recordUserMessage(content, &llmMsg)
	if loopInstance == nil {
		return errors.New("session loop not initialized")
	}

	sm.subpub.Publish(userMessage.SequenceID, StreamResponse{
		Messages: []Message{userMessage},
	})

	// Persist user message to store.
	if sm.onMessage != nil {
		if err := sm.onMessage(ctx, userMessage); err != nil {
			sm.logger.Warn("failed to persist user message", "error", err)
		}
	}

	loopInstance.QueueUserMessage(llmMsg)
	sm.SetWorking(true)

	return nil
}

// recordUserMessage adds a user message to the session and returns it with the loop instance.
func (sm *SessionManager) recordUserMessage(content string, llmMsg *llm.Message) (Message, *Loop) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	sm.lastActivity = now
	sm.sequenceID++

	msg := Message{
		ID:         uuid.New().String(),
		SessionID:  sm.id,
		Type:       MessageTypeUser,
		SequenceID: sm.sequenceID,
		Content:    content,
		CreatedAt:  now,
		LLMData:    llmMsg,
	}

	sm.messages = append(sm.messages, msg)
	return msg, sm.loop
}

// Subscribe returns a function that blocks until the next message is available.
func (sm *SessionManager) Subscribe(ctx context.Context) func() (StreamResponse, bool) {
	sm.mu.Lock()
	lastSeq := sm.sequenceID
	sm.mu.Unlock()

	return sm.subpub.Subscribe(ctx, lastSeq)
}

// SubscribeWithSnapshot atomically captures current state and subscribes.
func (sm *SessionManager) SubscribeWithSnapshot(ctx context.Context) (StreamResponse, func() (StreamResponse, bool)) {
	sm.mu.Lock()
	msgs := make([]Message, len(sm.messages))
	copy(msgs, sm.messages)
	lastSeq := sm.sequenceID
	working := sm.working
	model := sm.model
	totalCost := sm.totalCost
	id := sm.id

	sm.promptsMu.Lock()
	hasPendingPrompt := len(sm.pendingPrompts) > 0
	sm.promptsMu.Unlock()
	sess := Session{
		ID:              id,
		UserID:          sm.user.UserID,
		DAGName:         sm.dagName,
		ParentSessionID: sm.parentSessionID,
		DelegateTask:    sm.delegateTask,
		CreatedAt:       sm.createdAt,
		UpdatedAt:       sm.lastActivity,
	}
	var delegates []DelegateSnapshot
	if len(sm.delegates) > 0 {
		delegates = make([]DelegateSnapshot, 0, len(sm.delegates))
		for _, snap := range sm.delegates {
			delegates = append(delegates, snap)
		}
	}
	next := sm.subpub.Subscribe(ctx, lastSeq)
	sm.mu.Unlock()

	return StreamResponse{
		Messages: msgs,
		Session:  &sess,
		SessionState: &SessionState{
			SessionID:        id,
			Working:          working,
			HasPendingPrompt: hasPendingPrompt,
			Model:            model,
			TotalCost:        totalCost,
		},
		Delegates: delegates,
	}, next
}

// Cancel stops the session loop. The context parameter is unused
// because cancellation is performed synchronously via the internal cancel function.
func (sm *SessionManager) Cancel(_ context.Context) error {
	if cancel := sm.clearLoop(); cancel != nil {
		cancel()
	}
	sm.SetWorking(false)
	sm.logger.Info("session cancelled")
	return nil
}

// clearLoop resets the loop state and returns the cancel function.
func (sm *SessionManager) clearLoop() context.CancelFunc {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cancel := sm.loopCancel
	sm.loopCancel = nil
	sm.loop = nil
	return cancel
}

// ensureLoop creates the loop if it doesn't exist.
// The lock is released during createLoop (which may be slow) and re-acquired
// afterward with a double-check to handle concurrent callers.
func (sm *SessionManager) ensureLoop(provider llm.Provider, modelID string, resolvedModel string) error {
	sm.mu.Lock()
	if sm.loop != nil {
		sm.mu.Unlock()
		return nil
	}
	sm.model = modelID
	history := sm.extractLLMHistoryLocked()
	safeMode := sm.safeMode
	sm.mu.Unlock()

	loopCtx, cancel := context.WithCancel(context.Background())
	loopInstance := sm.createLoop(provider, resolvedModel, history, safeMode)

	sm.mu.Lock()
	if sm.loop != nil {
		sm.mu.Unlock()
		cancel()
		return nil
	}
	sm.loop = loopInstance
	sm.loopCancel = cancel
	sm.mu.Unlock()

	go sm.runLoop(loopCtx, loopInstance)
	return nil
}

// createLoop creates a new Loop instance with the current configuration.
func (sm *SessionManager) createLoop(provider llm.Provider, model string, history []llm.Message, safeMode bool) *Loop {
	memory := sm.loadMemory()
	allowedSkills := ToSkillSet(sm.enabledSkills)
	skillCount := len(sm.enabledSkills)
	var skillSummaries []SkillSummary
	if skillCount > 0 && skillCount <= SkillListThreshold {
		skillSummaries = LoadSkillSummaries(context.Background(), sm.skillStore, sm.enabledSkills)
	}
	return NewLoop(LoopConfig{
		Provider: provider,
		Model:    model,
		History:  history,
		Tools: CreateTools(ToolConfig{
			DAGsDir:       sm.environment.DAGsDir,
			SkillStore:    sm.skillStore,
			AllowedSkills: allowedSkills,
		}),
		RecordMessage:    sm.createRecordMessageFunc(),
		Logger:           sm.logger,
		SystemPrompt:     GenerateSystemPrompt(sm.environment, nil, memory, sm.user.Role, skillSummaries, skillCount),
		WorkingDir:       sm.workingDir,
		SessionID:        sm.id,
		OnWorking:        sm.SetWorking,
		OnHeartbeat:      sm.RecordHeartbeat,
		EmitUIAction:     sm.createEmitUIActionFunc(),
		EmitUserPrompt:   sm.createEmitUserPromptFunc(),
		WaitUserResponse: sm.createWaitUserResponseFunc(),
		SafeMode:         safeMode,
		Hooks:            sm.hooks,
		User:             sm.user,
		SessionStore:     sm.sessionStore,
		Registry:         sm.registry,
		SkillStore:       sm.skillStore,
		AllowedSkills:    allowedSkills,
	})
}

// loadMemory loads memory content from the memory store.
func (sm *SessionManager) loadMemory() MemoryContent {
	if sm.memoryStore == nil {
		return MemoryContent{}
	}
	ctx := context.Background()
	global, err := sm.memoryStore.LoadGlobalMemory(ctx)
	if err != nil {
		sm.logger.Debug("failed to load global memory", "error", err)
	}
	var dagMem string
	if sm.dagName != "" {
		dagMem, err = sm.memoryStore.LoadDAGMemory(ctx, sm.dagName)
		if err != nil {
			sm.logger.Debug("failed to load DAG memory", "error", err, "dag_name", sm.dagName)
		}
	}
	return MemoryContent{
		GlobalMemory: global,
		DAGMemory:    dagMem,
		DAGName:      sm.dagName,
		MemoryDir:    sm.memoryStore.MemoryDir(),
	}
}

// runLoop executes the session loop and handles cleanup.
func (sm *SessionManager) runLoop(ctx context.Context, loop *Loop) {
	defer func() {
		if r := recover(); r != nil {
			sm.logger.Error("session loop panicked", "panic", r, "stack", string(debug.Stack()))
		}
		sm.SetWorking(false)
		sm.logger.Info("session loop goroutine exiting")
		sm.clearLoop()
	}()

	err := loop.Go(ctx)
	if err == nil {
		return
	}
	if errors.Is(err, context.Canceled) {
		sm.logger.Info("session loop canceled normally")
	} else {
		sm.logger.Error("session loop stopped with error", "error", err)
	}
}

// extractLLMHistoryLocked converts stored messages to LLM format.
// Must be called with sm.mu held.
func (sm *SessionManager) extractLLMHistoryLocked() []llm.Message {
	history := make([]llm.Message, 0, len(sm.messages))
	for _, msg := range sm.messages {
		if msg.LLMData != nil {
			history = append(history, *msg.LLMData)
		}
	}
	return history
}

// createRecordMessageFunc returns a function for recording messages to the session.
// Persistence errors are logged but not propagated — the session continues operating
// even if individual messages fail to persist, to avoid disrupting the user's workflow.
func (sm *SessionManager) createRecordMessageFunc() MessageRecordFunc {
	return func(ctx context.Context, msg Message) {
		msg.SessionID = sm.id
		if msg.ID == "" {
			msg.ID = uuid.New().String()
		}

		// Calculate and accumulate cost for assistant messages with usage data
		if msg.Type == MessageTypeAssistant && msg.Usage != nil {
			cost := sm.calculateCost(msg.Usage)
			if cost > 0 {
				msg.Cost = &cost
			}
			sm.addCost(cost)
		}

		msg.SequenceID = sm.appendMessage(msg)

		sm.subpub.Publish(msg.SequenceID, StreamResponse{
			Messages: []Message{msg},
		})

		if sm.onMessage != nil {
			if err := sm.onMessage(ctx, msg); err != nil {
				sm.logger.Warn("failed to persist message", "error", err)
			}
		}
	}
}

// appendMessage adds a message to the session and returns the new sequence ID.
func (sm *SessionManager) appendMessage(msg Message) int64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.messages = append(sm.messages, msg)
	sm.sequenceID++
	return sm.sequenceID
}

// createEmitUIActionFunc returns a function for emitting UI actions.
func (sm *SessionManager) createEmitUIActionFunc() UIActionFunc {
	return func(action UIAction) {
		seqID := sm.nextSequenceID()

		sm.subpub.Publish(seqID, StreamResponse{
			Messages: []Message{{
				ID:         fmt.Sprintf("ui-%d", seqID),
				SessionID:  sm.id,
				Type:       MessageTypeUIAction,
				SequenceID: seqID,
				UIAction:   &action,
				CreatedAt:  time.Now(),
			}},
		})
	}
}

// createEmitUserPromptFunc returns a function for emitting user prompts.
func (sm *SessionManager) createEmitUserPromptFunc() EmitUserPromptFunc {
	return func(prompt UserPrompt) {
		msg := Message{
			ID:         fmt.Sprintf("prompt-%s", prompt.PromptID),
			SessionID:  sm.id,
			Type:       MessageTypeUserPrompt,
			UserPrompt: &prompt,
			CreatedAt:  time.Now(),
		}

		msg.SequenceID = sm.appendMessage(msg)

		sm.subpub.Publish(msg.SequenceID, StreamResponse{
			Messages: []Message{msg},
		})

		if sm.onMessage != nil {
			if err := sm.onMessage(context.Background(), msg); err != nil {
				sm.logger.Warn("failed to persist user prompt message", "error", err)
			}
		}
	}
}

// createWaitUserResponseFunc returns a function that blocks until user responds.
func (sm *SessionManager) createWaitUserResponseFunc() WaitUserResponseFunc {
	return func(ctx context.Context, promptID string) (UserPromptResponse, error) {
		ch := make(chan UserPromptResponse, 1)

		sm.promptsMu.Lock()
		sm.pendingPrompts[promptID] = ch
		sm.promptsMu.Unlock()

		defer func() {
			sm.promptsMu.Lock()
			delete(sm.pendingPrompts, promptID)
			sm.promptsMu.Unlock()
		}()

		select {
		case resp := <-ch:
			return resp, nil
		case <-ctx.Done():
			return UserPromptResponse{}, ctx.Err()
		}
	}
}

// SubmitUserResponse delivers a user's response to a pending prompt.
// Returns true if the response was delivered, false if no pending prompt exists.
func (sm *SessionManager) SubmitUserResponse(response UserPromptResponse) bool {
	sm.promptsMu.Lock()
	ch, exists := sm.pendingPrompts[response.PromptID]
	sm.promptsMu.Unlock()

	if !exists {
		slog.Warn("no pending prompt for response", "promptID", response.PromptID)
		return false
	}

	select {
	case ch <- response:
		return true
	default:
		slog.Warn("response dropped, channel full", "promptID", response.PromptID)
		return false
	}
}

// nextSequenceID increments and returns the next sequence ID.
func (sm *SessionManager) nextSequenceID() int64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.sequenceID++
	return sm.sequenceID
}

// GetTotalCost returns the accumulated cost of the session in USD.
func (sm *SessionManager) GetTotalCost() float64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.totalCost
}

// UpdatePricing updates the per-token pricing for this session.
func (sm *SessionManager) UpdatePricing(inputCostPer1M, outputCostPer1M float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.inputCostPer1M = inputCostPer1M
	sm.outputCostPer1M = outputCostPer1M
}

// calculateCost computes the cost of a single message from its token usage.
func (sm *SessionManager) calculateCost(usage *llm.Usage) float64 {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if usage == nil {
		return 0
	}
	return (float64(usage.PromptTokens) * sm.inputCostPer1M / 1_000_000) +
		(float64(usage.CompletionTokens) * sm.outputCostPer1M / 1_000_000)
}

// SetDelegateStarted records that a delegate sub-agent has started.
func (sm *SessionManager) SetDelegateStarted(id, task string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.delegates[id] = DelegateSnapshot{
		ID:     id,
		Task:   task,
		Status: DelegateStatusRunning,
	}
}

// SetDelegateCompleted records that a delegate sub-agent has completed.
func (sm *SessionManager) SetDelegateCompleted(id string, cost float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if snap, ok := sm.delegates[id]; ok {
		snap.Status = DelegateStatusCompleted
		snap.Cost = cost
		sm.delegates[id] = snap
	}
}

// GetDelegates returns snapshots of all tracked delegate sub-agents.
func (sm *SessionManager) GetDelegates() []DelegateSnapshot {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if len(sm.delegates) == 0 {
		return nil
	}
	result := make([]DelegateSnapshot, 0, len(sm.delegates))
	for _, snap := range sm.delegates {
		result = append(result, snap)
	}
	return result
}

// addCost adds a cost amount to the running total.
func (sm *SessionManager) addCost(cost float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.totalCost += cost
}

// sessionRegistry implements SubSessionRegistry by combining
// API-level session storage with SessionManager-level notifications.
type sessionRegistry struct {
	sessions *sync.Map       // API's active sessions map
	parent   *SessionManager // parent session manager
}

func (r *sessionRegistry) RegisterSubSession(id string, mgr *SessionManager) {
	r.sessions.Store(id, mgr)
}

func (r *sessionRegistry) DeregisterSubSession(id string) {
	r.sessions.Delete(id)
}

func (r *sessionRegistry) NotifyParent(event StreamResponse) {
	r.parent.subpub.Broadcast(event)
}

func (r *sessionRegistry) AddCost(cost float64) {
	r.parent.addCost(cost)
}

func (r *sessionRegistry) ParentSessionManager() *SessionManager {
	return r.parent
}
