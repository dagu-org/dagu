package agent

import (
	"context"
	"encoding/json"

	"github.com/dagu-org/dagu/internal/auth"
)

// RequestCommandApprovalFunc requests user approval for a command blocked by policy.
type RequestCommandApprovalFunc func(ctx context.Context, command, reason string) (bool, error)

// ToolExecInfo provides context about a tool execution for hooks.
type ToolExecInfo struct {
	ToolName  string
	Input     json.RawMessage
	SessionID string
	UserID    string
	Username  string
	IPAddress string
	Role      auth.Role
	Audit     *AuditInfo // from AgentTool.Audit; nil = not audited
	// SafeMode indicates whether the session has safe mode enabled.
	// Hooks can use this to decide whether to prompt for approval.
	SafeMode bool
	// RequestCommandApproval prompts the user to approve command execution.
	// It can be nil when prompts are unavailable.
	RequestCommandApproval RequestCommandApprovalFunc
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

// HasBeforeToolExecHooks reports whether any before-exec hooks are registered.
func (h *Hooks) HasBeforeToolExecHooks() bool {
	return h != nil && len(h.beforeToolExec) > 0
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
