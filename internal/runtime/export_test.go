package runtime

import "context"

// SetupChatMessages exports setupChatMessages for testing.
func (r *Runner) SetupChatMessages(ctx context.Context, node *Node) {
	r.setupChatMessages(ctx, node)
}

// SetupPushBackConversation exports setupPushBackConversation for testing.
func (r *Runner) SetupPushBackConversation(ctx context.Context, node *Node) {
	r.setupPushBackConversation(ctx, node)
}

// SetApprovalIteration sets the approval iteration count for testing.
func (n *Node) SetApprovalIteration(iteration int) {
	n.Data.mu.Lock()
	defer n.Data.mu.Unlock()
	n.inner.State.ApprovalIteration = iteration
}
