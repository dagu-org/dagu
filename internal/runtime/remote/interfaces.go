package remote

import (
	"context"

	"github.com/dagu-org/dagu/internal/core/execution"
)

// StatusWriter is an interface for writing DAG run status updates.
// It abstracts the destination of status updates, allowing for:
// - Local file-based storage (via execution.DAGRunAttempt)
// - Remote push to coordinator (via StatusPusher)
type StatusWriter interface {
	Write(ctx context.Context, status execution.DAGRunStatus) error
}
