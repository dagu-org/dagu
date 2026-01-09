package remote

import (
	"context"
	"io"

	"github.com/dagu-org/dagu/internal/core/execution"
)

// StatusWriter is an interface for writing DAG run status updates.
// It abstracts the destination of status updates, allowing for:
// - Local file-based storage (via execution.DAGRunAttempt)
// - Remote push to coordinator (via StatusPusher)
type StatusWriter interface {
	Write(ctx context.Context, status execution.DAGRunStatus) error
}

// LogWriterFactory creates log writers for step stdout/stderr.
// It abstracts where logs are written, allowing for:
// - Local file-based storage
// - Remote streaming to coordinator
type LogWriterFactory interface {
	// NewStepWriter creates a writer for a step's log output.
	// stepName identifies the step, streamType indicates stdout(1) or stderr(2).
	NewStepWriter(ctx context.Context, stepName string, streamType LogStreamType) io.WriteCloser
}
