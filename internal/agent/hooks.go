package agent

import (
	"context"
	"encoding/json"
)

// ToolExecInfo provides context about a tool execution for hooks.
type ToolExecInfo struct {
	ToolName       string
	Input          json.RawMessage
	ConversationID string
	UserID         string
	Username       string
	IPAddress      string
}

// BeforeToolExecHookFunc is called before tool execution.
// Return non-nil error to block execution.
type BeforeToolExecHookFunc func(ctx context.Context, info ToolExecInfo) error

// AfterToolExecHookFunc is called after tool execution.
type AfterToolExecHookFunc func(ctx context.Context, info ToolExecInfo, result ToolOut)

// Hooks provides lifecycle callbacks for agent tool execution.
// Register hooks at startup; invoke during tool execution.
// Not thread-safe for registration — register all hooks before use.
type Hooks struct {
	beforeToolExec []BeforeToolExecHookFunc
	afterToolExec  []AfterToolExecHookFunc
}

// NewHooks creates a new Hooks instance.
func NewHooks() *Hooks { return &Hooks{} }

// OnBeforeToolExec registers a hook called before tool execution.
func (h *Hooks) OnBeforeToolExec(fn BeforeToolExecHookFunc) {
	h.beforeToolExec = append(h.beforeToolExec, fn)
}

// OnAfterToolExec registers a hook called after tool execution.
func (h *Hooks) OnAfterToolExec(fn AfterToolExecHookFunc) {
	h.afterToolExec = append(h.afterToolExec, fn)
}

// RunBeforeToolExec invokes all before-execution hooks in order.
// Returns the first non-nil error, which blocks execution.
// Safe to call on a nil receiver (returns nil).
func (h *Hooks) RunBeforeToolExec(ctx context.Context, info ToolExecInfo) error {
	if h == nil {
		return nil
	}
	for _, fn := range h.beforeToolExec {
		if err := fn(ctx, info); err != nil {
			return err
		}
	}
	return nil
}

// RunAfterToolExec invokes all after-execution hooks in order.
// Safe to call on a nil receiver (no-op).
func (h *Hooks) RunAfterToolExec(ctx context.Context, info ToolExecInfo, result ToolOut) {
	if h == nil {
		return
	}
	for _, fn := range h.afterToolExec {
		fn(ctx, info, result)
	}
}
